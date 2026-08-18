package payment

import "context"

// Port is the payment capability used by the gateway and checkout saga.
// LocalService and Client are the two supported adapters at this seam.
type Port interface {
	ProcessPaymentV2(ctx context.Context, userID string, req *CreatePaymentRequest) (*Payment, error)
	ProcessPayment(ctx context.Context, orderID, userID string, amount float64, currency, method, idempotencyKey string) error
	GetPayment(ctx context.Context, id string) (*Payment, error)
	RefundPayment(ctx context.Context, id string, reason string) (*Payment, error)
}
