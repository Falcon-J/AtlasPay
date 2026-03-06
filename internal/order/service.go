package order

import (
	"context"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/logger"
)

// Service handles order business logic
type Service struct {
	repo *Repository
}

// NewService creates a new order service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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
		Msg("order created")

	return order, nil
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
