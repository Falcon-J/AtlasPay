package order

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atlaspay/platform/internal/common/cache"
	"github.com/atlaspay/platform/internal/common/saga"
	"github.com/atlaspay/platform/pkg/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	orderCachePrefix = "order:"
	orderCacheTTL    = 5 * time.Minute
)

// Repository handles order data persistence
type Repository struct {
	db    *pgxpool.Pool
	cache *cache.RedisCache
}

// NewRepository creates a new order repository
func NewRepository(db *pgxpool.Pool, cache *cache.RedisCache) *Repository {
	return &Repository{db: db, cache: cache}
}

// Create creates a new order with items and, when provided, its outbox event
// in the same PostgreSQL transaction.
func (r *Repository) Create(ctx context.Context, order *Order, event *events.Event) error {
	if order.ID == "" {
		order.ID = uuid.New().String()
	}
	order.Status = StatusPending
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert order
	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, user_id, status, total_price, currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, order.ID, order.UserID, order.Status, order.TotalPrice, order.Currency, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return err
	}

	// Insert items
	for i := range order.Items {
		order.Items[i].ID = uuid.New().String()
		order.Items[i].OrderID = order.ID
		order.Items[i].TotalPrice = order.Items[i].UnitPrice * float64(order.Items[i].Quantity)

		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, order_id, sku, name, quantity, unit_price, total_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, order.Items[i].ID, order.Items[i].OrderID, order.Items[i].SKU, order.Items[i].Name,
			order.Items[i].Quantity, order.Items[i].UnitPrice, order.Items[i].TotalPrice)
		if err != nil {
			return err
		}
	}

	if event != nil {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO outbox_events (id, topic, event_type, aggregate_id, payload, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, event.ID, events.TopicOrders, event.Type, event.AggregateID, payload, time.Now())
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Cache the order
	r.cacheOrder(ctx, order)
	return nil
}

type pendingOutboxEvent struct {
	ID      string
	Topic   string
	Payload []byte
}

// ClaimPendingOutbox leases a batch so multiple application instances do not
// publish the same event concurrently. A stale lease is reclaimable after a
// publisher crash.
func (r *Repository) ClaimPendingOutbox(ctx context.Context, limit int) ([]pendingOutboxEvent, error) {
	rows, err := r.db.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND (claimed_at IS NULL OR claimed_at < CURRENT_TIMESTAMP - INTERVAL '1 minute')
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_events AS outbox
		SET claimed_at = CURRENT_TIMESTAMP,
		    attempts = outbox.attempts + 1
		FROM candidates
		WHERE outbox.id = candidates.id
		RETURNING outbox.id, outbox.topic, outbox.payload
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventsToPublish []pendingOutboxEvent
	for rows.Next() {
		var event pendingOutboxEvent
		if err := rows.Scan(&event.ID, &event.Topic, &event.Payload); err != nil {
			return nil, err
		}
		eventsToPublish = append(eventsToPublish, event)
	}
	return eventsToPublish, rows.Err()
}

func (r *Repository) MarkOutboxPublished(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = CURRENT_TIMESTAMP, claimed_at = NULL, last_error = NULL
		WHERE id = $1
	`, id)
	return err
}

func (r *Repository) MarkOutboxFailed(ctx context.Context, id string, publishErr error) error {
	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events
		SET claimed_at = NULL, last_error = $2
		WHERE id = $1
	`, id, publishErr.Error())
	return err
}

// ClaimSaga leases one order for one event. A retry of the same Kafka event
// refreshes its lease; a different replay is ignored while the original saga
// is active. Expired leases are reclaimable after a worker crash.
func (r *Repository) ClaimSaga(ctx context.Context, orderID, eventID string) (bool, error) {
	var claimedOrderID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO saga_claims (order_id, event_id, status, lease_expires_at, updated_at)
		VALUES ($1, $2, 'running', CURRENT_TIMESTAMP + INTERVAL '2 minutes', CURRENT_TIMESTAMP)
		ON CONFLICT (order_id) DO UPDATE
		SET event_id = EXCLUDED.event_id,
		    status = 'running',
		    lease_expires_at = EXCLUDED.lease_expires_at,
		    updated_at = EXCLUDED.updated_at
		WHERE saga_claims.event_id = EXCLUDED.event_id
		   OR saga_claims.lease_expires_at < CURRENT_TIMESTAMP
		RETURNING order_id
	`, orderID, eventID).Scan(&claimedOrderID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return claimedOrderID == orderID, nil
}

// Observe persists the latest saga snapshot and step transition. The unique
// identity makes a replay of the same transition harmless.
func (r *Repository) Observe(ctx context.Context, current *saga.Saga) error {
	if len(current.StepLogs) == 0 {
		return nil
	}

	payload, err := json.Marshal(current)
	if err != nil {
		return err
	}
	step := current.StepLogs[len(current.StepLogs)-1]
	_, err = r.db.Exec(ctx, `
		INSERT INTO saga_logs (id, saga_id, step_name, status, payload, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (saga_id, step_name, status) DO UPDATE
		SET payload = EXCLUDED.payload,
		    error_message = EXCLUDED.error_message,
		    created_at = EXCLUDED.created_at
	`, uuid.New().String(), current.ID, step.StepName, step.Status, payload, step.Error, time.Now())
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE saga_claims
		SET status = $2,
		    lease_expires_at = CURRENT_TIMESTAMP + INTERVAL '2 minutes',
		    updated_at = CURRENT_TIMESTAMP
		WHERE order_id = $1
	`, current.ID, current.Status)
	return err
}

// GetPersistedSaga returns the latest durable saga snapshot for an order.
func (r *Repository) GetPersistedSaga(ctx context.Context, sagaID string) (*saga.Saga, bool, error) {
	var payload []byte
	err := r.db.QueryRow(ctx, `
		SELECT payload
		FROM saga_logs
		WHERE saga_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, sagaID).Scan(&payload)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	current := &saga.Saga{}
	if err := json.Unmarshal(payload, current); err != nil {
		return nil, false, err
	}
	return current, true, nil
}

// GetByID retrieves an order by ID with Redis caching
func (r *Repository) GetByID(ctx context.Context, id string) (*Order, error) {
	// Try cache first
	if r.cache != nil {
		var order Order
		if err := r.cache.Get(ctx, orderCachePrefix+id, &order); err == nil {
			// Fetch items separately (not cached for simplicity)
			items, _ := r.getOrderItems(ctx, id)
			order.Items = items
			return &order, nil
		}
	}

	// Cache miss - query DB
	order := &Order{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, status, total_price, currency, created_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(&order.ID, &order.UserID, &order.Status, &order.TotalPrice, &order.Currency, &order.CreatedAt, &order.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Get items
	order.Items, err = r.getOrderItems(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache for next time
	r.cacheOrder(ctx, order)
	return order, nil
}

func (r *Repository) getOrderItems(ctx context.Context, orderID string) ([]OrderItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, sku, name, quantity, unit_price, total_price
		FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OrderItem
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.SKU, &item.Name, &item.Quantity, &item.UnitPrice, &item.TotalPrice); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// UpdateStatus updates order status with state machine validation
func (r *Repository) UpdateStatus(ctx context.Context, id string, newStatus OrderStatus) error {
	order, err := r.GetByID(ctx, id)
	if err != nil || order == nil {
		return fmt.Errorf("order not found")
	}

	if !order.CanTransitionTo(newStatus) {
		return fmt.Errorf("invalid status transition from %s to %s", order.Status, newStatus)
	}

	_, err = r.db.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3
	`, newStatus, time.Now(), id)
	if err != nil {
		return err
	}

	// Invalidate cache
	r.invalidateCache(ctx, id)
	return nil
}

// List retrieves orders with filtering and pagination
func (r *Repository) List(ctx context.Context, filter *OrderFilter) ([]*Order, int64, error) {
	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) FROM orders WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter.UserID != "" {
		countQuery += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, filter.UserID)
		argIndex++
	}
	if filter.Status != "" {
		countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}

	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Query orders
	query := `SELECT id, user_id, status, total_price, currency, created_at, updated_at 
			  FROM orders WHERE 1=1`

	args = []interface{}{}
	argIndex = 1

	if filter.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, filter.UserID)
		argIndex++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	offset := (filter.Page - 1) * filter.PageSize
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, filter.PageSize, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		if err := rows.Scan(&order.ID, &order.UserID, &order.Status, &order.TotalPrice, &order.Currency, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, 0, err
		}
		orders = append(orders, order)
	}

	return orders, total, nil
}

// GetByUserID retrieves orders for a specific user
func (r *Repository) GetByUserID(ctx context.Context, userID string, page, pageSize int) ([]*Order, int64, error) {
	return r.List(ctx, &OrderFilter{
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	})
}

func (r *Repository) cacheOrder(ctx context.Context, order *Order) {
	if r.cache != nil {
		r.cache.Set(ctx, orderCachePrefix+order.ID, order, orderCacheTTL)
	}
}

func (r *Repository) invalidateCache(ctx context.Context, id string) {
	if r.cache != nil {
		r.cache.Delete(ctx, orderCachePrefix+id)
	}
}
