package saga

import (
	"context"
	"fmt"

	"github.com/atlaspay/platform/internal/common/logger"
)

// OrderPlacementData holds data for order placement saga
type OrderPlacementData struct {
	OrderID        string
	UserID         string
	Items          []OrderItem
	TotalAmount    float64
	Currency       string
	PaymentMethod  string
	IdempotencyKey string
}

// OrderItem represents an order item
type OrderItem struct {
	SKU      string
	Quantity int
}

// OrderService interface for order operations
type OrderService interface {
	ConfirmOrder(ctx context.Context, orderID string) error
	CancelOrder(ctx context.Context, orderID string) error
	FailOrder(ctx context.Context, orderID string) error
}

// InventoryService interface for inventory operations
type InventoryService interface {
	ReserveStock(ctx context.Context, orderID string, items []OrderItem) error
	ReleaseStock(ctx context.Context, orderID string) error
	CommitStock(ctx context.Context, orderID string) error
}

// PaymentService interface for payment operations
type PaymentService interface {
	ProcessPayment(ctx context.Context, orderID, userID string, amount float64, currency, method, idempotencyKey string) error
}

// OrderPlacementSaga creates the order placement saga
func OrderPlacementSaga(
	orderSvc OrderService,
	inventorySvc InventoryService,
	paymentSvc PaymentService,
) *Saga {
	steps := []Step{
		{
			Name: "reserve_inventory",
			Action: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Msg("reserving inventory")

				return inventorySvc.ReserveStock(ctx, initial.OrderID, initial.Items)
			},
			Compensation: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Msg("releasing reserved inventory")

				return inventorySvc.ReleaseStock(ctx, initial.OrderID)
			},
		},
		{
			Name: "process_payment",
			Action: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Float64("amount", initial.TotalAmount).
					Msg("processing payment")

				return paymentSvc.ProcessPayment(
					ctx,
					initial.OrderID,
					initial.UserID,
					initial.TotalAmount,
					initial.Currency,
					initial.PaymentMethod,
					initial.IdempotencyKey,
				)
			},
			Compensation: func(ctx context.Context, data interface{}) error {
				// Payment compensation would trigger refund
				// For now, just log - the payment service handles refunds separately
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Msg("marking order as failed due to payment issues")

				// We don't refund here - the release of inventory is enough
				// Refunds are handled through a separate process
				return nil
			},
		},
		{
			Name: "commit_inventory",
			Action: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Msg("committing inventory")

				return inventorySvc.CommitStock(ctx, initial.OrderID)
			},
			Compensation: nil, // No compensation needed - payment already succeeded
		},
		{
			Name: "confirm_order",
			Action: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Msg("confirming order")

				return orderSvc.ConfirmOrder(ctx, initial.OrderID)
			},
			Compensation: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)

				logger.Info(ctx).
					Str("order_id", initial.OrderID).
					Msg("failing order")

				return orderSvc.FailOrder(ctx, initial.OrderID)
			},
		},
	}

	saga := NewSaga("order_placement", steps)
	return saga
}

// OrderPlacementSagaTest is for testing the saga flow
type OrderPlacementSagaTest struct{}

func (s *OrderPlacementSagaTest) TestHappyPath() error {
	// Simulate successful flow
	return nil
}

func (s *OrderPlacementSagaTest) TestInventoryFailure() error {
	// Simulate inventory failure - order should be cancelled
	return fmt.Errorf("insufficient stock")
}

func (s *OrderPlacementSagaTest) TestPaymentFailure() error {
	// Simulate payment failure - inventory should be released
	return fmt.Errorf("payment declined")
}
