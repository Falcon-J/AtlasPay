package order

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/kafka"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/saga"
	"github.com/atlaspay/platform/pkg/events"
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
		orchestrator: saga.NewOrchestrator(),
		producer:     producer,
		kafkaEnabled: kafkaEnabled,
	}
}

// CreateOrder creates a new order
func (s *Service) CreateOrder(ctx context.Context, userID string, req *CreateOrderRequest) (*Order, error) {
	// Build order
	order := &Order{
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

	if err := s.repo.Create(ctx, order); err != nil {
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
		if err := s.publishOrderCreated(ctx, order); err != nil {
			logger.Error(ctx).Err(err).Str("order_id", order.ID).Msg("failed to publish order created event")
			_ = s.FailOrder(ctx, order.ID)
			return nil, errors.ErrInternalServer.WithDetails("failed to enqueue order workflow")
		}
		return order, nil
	}

	// Trigger Saga in background
	go s.executeOrderSaga(context.Background(), order)

	return order, nil
}

func (s *Service) publishOrderCreated(ctx context.Context, order *Order) error {
	if s.producer == nil {
		return fmt.Errorf("kafka producer is not configured")
	}

	items := make([]events.OrderItem, len(order.Items))
	for i, item := range order.Items {
		items[i] = events.OrderItem{
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
	}

	event, err := events.NewEvent(events.OrderCreated, order.ID, order.ID, events.OrderCreatedPayload{
		OrderID:    order.ID,
		UserID:     order.UserID,
		Items:      items,
		TotalPrice: order.TotalPrice,
		Currency:   order.Currency,
	})
	if err != nil {
		return err
	}
	return s.producer.Publish(ctx, events.TopicOrders, event)
}

// Handle processes order events from Kafka.
func (s *Service) Handle(ctx context.Context, event *events.Event) error {
	switch event.Type {
	case events.OrderCreated:
		var payload events.OrderCreatedPayload
		if err := event.UnmarshalPayload(&payload); err != nil {
			return err
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
		if item.SKU == "FAIL-PAYMENT-001" {
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
