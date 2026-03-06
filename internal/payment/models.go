package payment

import (
	"time"
)

// PaymentStatus represents payment states
type PaymentStatus string

const (
	PaymentPending    PaymentStatus = "pending"
	PaymentProcessing PaymentStatus = "processing"
	PaymentCompleted  PaymentStatus = "completed"
	PaymentFailed     PaymentStatus = "failed"
	PaymentRefunded   PaymentStatus = "refunded"
)

// Payment represents a payment in the system
type Payment struct {
	ID             string        `json:"id" db:"id"`
	OrderID        string        `json:"order_id" db:"order_id"`
	UserID         string        `json:"user_id" db:"user_id"`
	Amount         float64       `json:"amount" db:"amount"`
	Currency       string        `json:"currency" db:"currency"`
	Status         PaymentStatus `json:"status" db:"status"`
	PaymentMethod  string        `json:"payment_method" db:"payment_method"`
	IdempotencyKey string        `json:"idempotency_key" db:"idempotency_key"`
	FailureReason  string        `json:"failure_reason,omitempty" db:"failure_reason"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
}

// CreatePaymentRequest represents payment creation request
type CreatePaymentRequest struct {
	OrderID        string  `json:"order_id" validate:"required"`
	Amount         float64 `json:"amount" validate:"required,gt=0"`
	Currency       string  `json:"currency" validate:"required"`
	PaymentMethod  string  `json:"payment_method" validate:"required"`
	IdempotencyKey string  `json:"idempotency_key" validate:"required"`
}

// RefundRequest represents refund request
type RefundRequest struct {
	Reason string `json:"reason"`
}

// PaymentResponse represents payment API response
type PaymentResponse struct {
	Payment *Payment `json:"payment"`
}
