package order

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"time"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/kafka"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/atlaspay/platform/internal/common/saga"
	"github.com/atlaspay/platform/pkg/events"
	"github.com/google/uuid"
)

// Service handles order business logic
type Service struct {
	repo         *Repository
	inventorySvc saga.InventoryService
	paymentSvc   saga.PaymentService
	orchestrator *saga.Orchestrator
	producer     *kafka.Producer
	kafkaEnabled bool
}

// NewService creates a new order service
func NewService(repo *Repository, inventorySvc saga.InventoryService, paymentSvc saga.PaymentService) *Service {
	return NewServiceWithKafka(repo, inventorySvc, paymentSvc, nil, false)
}

// NewServiceWithKafka creates a new order service with optional Kafka order processing.
func NewServiceWithKafka(repo *Repository, inventorySvc saga.InventoryService, paymentSvc saga.PaymentService, producer *kafka.Producer, kafkaEnabled bool) *Service {
	return &Service{
		repo:         repo,
		inventorySvc: inventorySvc,
		paymentSvc:   paymentSvc,
		orchestrator: saga.NewOrchestrator(repo),
		producer:     producer,
		kafkaEnabled: kafkaEnabled,
	}
}

// CreateOrder creates a new order
func (s *Service) CreateOrder(ctx context.Context, userID string, req *CreateOrderRequest) (*Order, error) {
	// Build order
	order := &Order{
		ID:       uuid.New().String(),
		UserID:   userID,
		Currency: "USD",
		Items:    make([]OrderItem, len(req.Items)),
	}

	// For demo, we'll use mock prices - in production, fetch from inventory service
	for i, item := range req.Items {
		order.Items[i] = OrderItem{
			SKU:       item.SKU,
			Name:      "Product " + item.SKU, // Mock name
			Quantity:  item.Quantity,
			UnitPrice: 99.99, // Mock price - would come from inventory service
		}
		order.Items[i].TotalPrice = order.Items[i].UnitPrice * float64(order.Items[i].Quantity)
	}

	order.CalculateTotal()

	var orderEvent *events.Event
	if s.kafkaEnabled {
		var err error
		orderEvent, err = s.newOrderCreatedEvent(order)
		if err != nil {
			return nil, errors.ErrInternalServer.WithDetails("failed to build order event")
		}
	}

	if err := s.repo.Create(ctx, order, orderEvent); err != nil {
		logger.Error(ctx).Err(err).Msg("failed to create order")
		return nil, errors.ErrInternalServer.WithDetails("failed to create order")
	}

	logger.Info(ctx).
		Str("order_id", order.ID).
		Str("user_id", userID).
		Float64("total", order.TotalPrice).
		Bool("kafka_enabled", s.kafkaEnabled).
		Msg("order created")

	if s.kafkaEnabled {
		metrics.RecordOrder("created", order.TotalPrice)
		return order, nil
	}

	// Trigger Saga in background
	go s.executeOrderSaga(context.Background(), order)

	metrics.RecordOrder("created", order.TotalPrice)
	return order, nil
}

func (s *Service) newOrderCreatedEvent(order *Order) (*events.Event, error) {
	items := make([]events.OrderItem, len(order.Items))
	for i, item := range order.Items {
		items[i] = events.OrderItem{
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
	}

	return events.NewEvent(events.OrderCreated, order.ID, order.ID, events.OrderCreatedPayload{
		OrderID:    order.ID,
		UserID:     order.UserID,
		Items:      items,
		TotalPrice: order.TotalPrice,
		Currency:   order.Currency,
	})
}

// StartOutboxPublisher delivers committed order events to Kafka and retries
// failures. The database remains the source of truth for publication state.
func (s *Service) StartOutboxPublisher(ctx context.Context) {
	if !s.kafkaEnabled || s.producer == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.publishOutboxBatch(ctx)
			}
		}
	}()
}

func (s *Service) publishOutboxBatch(ctx context.Context) {
	pending, err := s.repo.ClaimPendingOutbox(ctx, 50)
	if err != nil {
		logger.Error(ctx).Err(err).Msg("failed to claim outbox events")
		return
	}

	for _, pendingEvent := range pending {
		var event events.Event
		if err := json.Unmarshal(pendingEvent.Payload, &event); err != nil {
			logger.Error(ctx).Err(err).Str("event_id", pendingEvent.ID).Msg("invalid outbox event")
			_ = s.repo.MarkOutboxFailed(ctx, pendingEvent.ID, err)
			continue
		}

		if err := s.producer.Publish(ctx, pendingEvent.Topic, &event); err != nil {
			logger.Error(ctx).Err(err).Str("event_id", pendingEvent.ID).Msg("outbox publish failed")
			_ = s.repo.MarkOutboxFailed(ctx, pendingEvent.ID, err)
			continue
		}

		if err := s.repo.MarkOutboxPublished(ctx, pendingEvent.ID); err != nil {
			logger.Error(ctx).Err(err).Str("event_id", pendingEvent.ID).Msg("failed to mark outbox event published")
		}
	}
}

// Handle processes order events from Kafka.
func (s *Service) Handle(ctx context.Context, event *events.Event) error {
	switch event.Type {
	case events.OrderCreated:
		var payload events.OrderCreatedPayload
		if err := event.UnmarshalPayload(&payload); err != nil {
			return err
		}

		persistedSaga, found, err := s.repo.GetPersistedSaga(ctx, payload.OrderID)
		if err != nil {
			return err
		}
		if found && (persistedSaga.Status == saga.SagaCompleted || persistedSaga.Status == saga.SagaCompensated) {
			logger.Info(ctx).
				Str("order_id", payload.OrderID).
				Str("saga_status", string(persistedSaga.Status)).
				Msg("ignoring redelivered terminal order event")
			return nil
		}

		eventID := event.ID
		if eventID == "" {
			eventID = event.CorrelationID
		}
		claimed, err := s.repo.ClaimSaga(ctx, payload.OrderID, eventID)
		if err != nil {
			return err
		}
		if !claimed {
			logger.Info(ctx).
				Str("order_id", payload.OrderID).
				Str("event_id", eventID).
				Msg("ignoring duplicate in-flight order event")
			return nil
		}

		order, err := s.GetOrder(ctx, payload.OrderID)
		if err != nil {
			return err
		}
		return s.executeOrderSaga(ctx, order)
	default:
		logger.Info(ctx).Str("event_type", string(event.Type)).Msg("order service ignored event")
		return nil
	}
}

func (s *Service) executeOrderSaga(ctx context.Context, order *Order) error {
	orderSaga := saga.OrderPlacementSaga(s, s.inventorySvc, s.paymentSvc)
	orderSaga.ID = order.ID

	sagaData := saga.OrderPlacementData{
		OrderID:        order.ID,
		UserID:         order.UserID,
		TotalAmount:    order.TotalPrice,
		Currency:       order.Currency,
		PaymentMethod:  "demo_card",
		IdempotencyKey: "SAGA-" + order.ID,
		Items:          make([]saga.OrderItem, len(order.Items)),
	}

	for i, item := range order.Items {
		sagaData.Items[i] = saga.OrderItem{
			SKU:      item.SKU,
			Quantity: item.Quantity,
		}
		if strings.HasPrefix(item.SKU, "FAIL-") {
			sagaData.IdempotencyKey = "FAIL-" + order.ID
		}
	}

	if err := s.orchestrator.Execute(ctx, orderSaga, sagaData); err != nil {
		logger.Error(ctx).Err(err).Str("order_id", order.ID).Msg("saga execution failed")
		if stderrors.Is(err, saga.ErrCompensated) {
			return nil
		}
		_ = s.FailOrder(ctx, order.ID)
		return err
	}

	logger.Info(ctx).Str("order_id", order.ID).Msg("saga execution completed")
	return nil
}

// GetSagaState retrieves the current status of a saga for an order
func (s *Service) GetSagaState(ctx context.Context, orderID string) (*saga.Saga, error) {
	persisted, found, err := s.repo.GetPersistedSaga(ctx, orderID)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if found {
		return persisted, nil
	}

	sg, exists := s.orchestrator.GetSaga(orderID)
	if !exists {
		return nil, errors.ErrOrderNotFound
	}
	return sg, nil
}

// GetOrder retrieves an order by ID
func (s *Service) GetOrder(ctx context.Context, id string) (*Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if order == nil {
		return nil, errors.ErrOrderNotFound
	}
	return order, nil
}

// GetUserOrders retrieves orders for a user
func (s *Service) GetUserOrders(ctx context.Context, userID string, page, pageSize int) ([]*Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	orders, total, err := s.repo.GetByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, errors.ErrInternalServer.WithDetails(err.Error())
	}
	return orders, total, nil
}

// UpdateOrderStatus updates order status (for saga coordination)
func (s *Service) UpdateOrderStatus(ctx context.Context, id string, status OrderStatus) error {
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		logger.Error(ctx).Err(err).Str("order_id", id).Str("status", string(status)).Msg("failed to update order status")
		return errors.ErrInternalServer.WithDetails(err.Error())
	}

	logger.Info(ctx).
		Str("order_id", id).
		Str("new_status", string(status)).
		Msg("order status updated")

	return nil
}

// ConfirmOrder confirms an order (called after successful inventory reservation)
func (s *Service) ConfirmOrder(ctx context.Context, id string) error {
	return s.UpdateOrderStatus(ctx, id, StatusConfirmed)
}

// CancelOrder cancels an order
func (s *Service) CancelOrder(ctx context.Context, id string) error {
	return s.UpdateOrderStatus(ctx, id, StatusCancelled)
}

// FailOrder marks an order as failed
func (s *Service) FailOrder(ctx context.Context, id string) error {
	return s.UpdateOrderStatus(ctx, id, StatusFailed)
}

// MarkPaid marks an order as paid
func (s *Service) MarkPaid(ctx context.Context, id string) error {
	return s.UpdateOrderStatus(ctx, id, StatusPaid)
}

// MarkShipped marks an order as shipped
func (s *Service) MarkShipped(ctx context.Context, id string) error {
	return s.UpdateOrderStatus(ctx, id, StatusShipped)
}

// MarkDelivered marks an order as delivered
func (s *Service) MarkDelivered(ctx context.Context, id string) error {
	return s.UpdateOrderStatus(ctx, id, StatusDelivered)
}
