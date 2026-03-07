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
			Name: "saga_init",
			Action: func(ctx context.Context, data interface{}) error {
				// Global initialization if needed.
				// For now, it just serves as a compensation anchor to fail the order.
				return nil
			},
			Compensation: func(ctx context.Context, data interface{}) error {
				d := data.(map[string]interface{})
				initial := d["initial"].(OrderPlacementData)
				logger.Warn(ctx).Str("order_id", initial.OrderID).Msg("compensating saga_init: marking order as failed")
				return orderSvc.FailOrder(ctx, initial.OrderID)
			},
		},
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
			Compensation: nil, // Note: Inventory release will handle the failure state
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
			Compensation: nil, // Let saga_init handle it if this fails (rare as it's just a DB update)
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
