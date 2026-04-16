package payment

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/logger"
)

// Service handles payment business logic
type Service struct {
	repo *Repository
}

// NewService creates a new payment service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ProcessPaymentV2 processes a payment with idempotency (for HTTP handlers)
func (s *Service) ProcessPaymentV2(ctx context.Context, userID string, req *CreatePaymentRequest) (*Payment, error) {
	return s.processPaymentInternal(ctx, userID, req)
}

func (s *Service) processPaymentInternal(ctx context.Context, userID string, req *CreatePaymentRequest) (*Payment, error) {
	// Check for existing payment with same idempotency key (idempotency check)
	existing, err := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if existing != nil {
		// Return existing payment (idempotent response)
		logger.Info(ctx).
			Str("payment_id", existing.ID).
			Str("idempotency_key", req.IdempotencyKey).
			Msg("returning existing payment (idempotent request)")
		return existing, nil
	}

	// Create payment record
	payment := &Payment{
		OrderID:        req.OrderID,
		UserID:         userID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		PaymentMethod:  req.PaymentMethod,
		IdempotencyKey: req.IdempotencyKey,
	}

	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}

	// Simulate payment processing (in production, call payment gateway)
	success := s.simulatePaymentGateway(payment)

	if success {
		if err := s.repo.UpdateStatus(ctx, payment.ID, PaymentCompleted, ""); err != nil {
			return nil, errors.ErrInternalServer.WithDetails(err.Error())
		}
		payment.Status = PaymentCompleted

		logger.Info(ctx).
			Str("payment_id", payment.ID).
			Str("order_id", payment.OrderID).
			Float64("amount", payment.Amount).
			Msg("payment completed successfully")
	} else {
		failureReason := "Payment declined by processor"
		if err := s.repo.UpdateStatus(ctx, payment.ID, PaymentFailed, failureReason); err != nil {
			return nil, errors.ErrInternalServer.WithDetails(err.Error())
		}
		payment.Status = PaymentFailed
		payment.FailureReason = failureReason

		logger.Warn(ctx).
			Str("payment_id", payment.ID).
			Str("order_id", payment.OrderID).
			Str("reason", failureReason).
			Msg("payment failed")

		return payment, errors.ErrPaymentFailed.WithDetails(failureReason)
	}

	return payment, nil
}

// ProcessPayment satisfies the saga.PaymentService interface
func (s *Service) ProcessPayment(ctx context.Context, orderID, userID string, amount float64, currency, method, idempotencyKey string) error {
	req := &CreatePaymentRequest{
		OrderID:        orderID,
		Amount:         amount,
		Currency:       currency,
		PaymentMethod:  method,
		IdempotencyKey: idempotencyKey,
	}

	// Override for demo failure sku
	// In a real system, the SKU might be checked earlier or the payment service might be told to fail.
	// We'll rely on the repo or simulator if we wanted, but let's keep it simple.

	_, err := s.processPaymentInternal(ctx, userID, req)
	return err
}

// GetPayment retrieves a payment by ID
func (s *Service) GetPayment(ctx context.Context, id string) (*Payment, error) {
	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if payment == nil {
		return nil, errors.ErrNotFound.WithDetails("payment not found")
	}
	return payment, nil
}

// GetPaymentByOrder retrieves payment for an order
func (s *Service) GetPaymentByOrder(ctx context.Context, orderID string) (*Payment, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}

// RefundPayment processes a refund
func (s *Service) RefundPayment(ctx context.Context, id string, reason string) (*Payment, error) {
	payment, err := s.GetPayment(ctx, id)
	if err != nil {
		return nil, err
	}

	if payment.Status != PaymentCompleted {
		return nil, errors.ErrBadRequest.WithDetails("can only refund completed payments")
	}

	// Simulate refund processing
	if err := s.repo.UpdateStatus(ctx, id, PaymentRefunded, reason); err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}

	payment.Status = PaymentRefunded
	logger.Info(ctx).
		Str("payment_id", id).
		Str("reason", reason).
		Msg("payment refunded")

	return payment, nil
}

// simulatePaymentGateway simulates a payment gateway response
// In production, this would call Stripe, PayPal, etc.
func (s *Service) simulatePaymentGateway(payment *Payment) bool {
	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Special case for demo failure SKU
	// The frontend uses SKU 'FAIL-PAYMENT-001' to trigger failure.
	// We'll check if the idempotency key contains 'FAIL' as a signal.
	if strings.HasPrefix(payment.IdempotencyKey, "FAIL") {
		return false
	}

	// Keep demos deterministic: saga and explicit demo requests succeed
	// unless they intentionally use the FAIL prefix above.
	if strings.HasPrefix(payment.IdempotencyKey, "SAGA-") ||
		strings.HasPrefix(payment.IdempotencyKey, "DEMO-") {
		return true
	}

	// 95% success rate for general demo
	return rand.Float32() < 0.95
}
