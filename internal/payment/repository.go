package payment

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles payment data persistence
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new payment repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create creates a new payment
func (r *Repository) Create(ctx context.Context, payment *Payment) error {
	payment.ID = uuid.New().String()
	payment.Status = PaymentProcessing
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()

	_, err := r.db.Exec(ctx, `
		INSERT INTO payments (id, order_id, user_id, amount, currency, status, payment_method, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, payment.ID, payment.OrderID, payment.UserID, payment.Amount, payment.Currency, payment.Status, payment.PaymentMethod, payment.IdempotencyKey, payment.CreatedAt, payment.UpdatedAt)

	return err
}

// GetByID retrieves a payment by ID
func (r *Repository) GetByID(ctx context.Context, id string) (*Payment, error) {
	payment := &Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, user_id, amount, currency, status, payment_method, idempotency_key, failure_reason, created_at, updated_at
		FROM payments WHERE id = $1
	`, id).Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Currency, &payment.Status, &payment.PaymentMethod, &payment.IdempotencyKey, &payment.FailureReason, &payment.CreatedAt, &payment.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return payment, err
}

// GetByIdempotencyKey retrieves a payment by idempotency key (for deduplication)
func (r *Repository) GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error) {
	payment := &Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, user_id, amount, currency, status, payment_method, idempotency_key, failure_reason, created_at, updated_at
		FROM payments WHERE idempotency_key = $1
	`, key).Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Currency, &payment.Status, &payment.PaymentMethod, &payment.IdempotencyKey, &payment.FailureReason, &payment.CreatedAt, &payment.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return payment, err
}

// GetByOrderID retrieves payment for an order
func (r *Repository) GetByOrderID(ctx context.Context, orderID string) (*Payment, error) {
	payment := &Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, user_id, amount, currency, status, payment_method, idempotency_key, failure_reason, created_at, updated_at
		FROM payments WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1
	`, orderID).Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Currency, &payment.Status, &payment.PaymentMethod, &payment.IdempotencyKey, &payment.FailureReason, &payment.CreatedAt, &payment.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return payment, err
}

// UpdateStatus updates payment status
func (r *Repository) UpdateStatus(ctx context.Context, id string, status PaymentStatus, failureReason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payments SET status = $1, failure_reason = $2, updated_at = $3 WHERE id = $4
	`, status, failureReason, time.Now(), id)
	return err
}

// GetUserPayments retrieves payments for a user
func (r *Repository) GetUserPayments(ctx context.Context, userID string, limit, offset int) ([]*Payment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, user_id, amount, currency, status, payment_method, idempotency_key, failure_reason, created_at, updated_at
		FROM payments WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*Payment
	for rows.Next() {
		p := &Payment{}
		if err := rows.Scan(&p.ID, &p.OrderID, &p.UserID, &p.Amount, &p.Currency, &p.Status, &p.PaymentMethod, &p.IdempotencyKey, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, nil
}
