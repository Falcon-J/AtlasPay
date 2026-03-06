package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/atlaspay/platform/internal/common/cache"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	inventoryCachePrefix = "inventory:"
	inventoryCacheTTL    = 1 * time.Minute // Short TTL for inventory
)

// Repository handles inventory data persistence
type Repository struct {
	db    *pgxpool.Pool
	cache *cache.RedisCache
}

// NewRepository creates a new inventory repository
func NewRepository(db *pgxpool.Pool, cache *cache.RedisCache) *Repository {
	return &Repository{db: db, cache: cache}
}

// GetBySKU retrieves inventory item by SKU with caching
func (r *Repository) GetBySKU(ctx context.Context, sku string) (*InventoryItem, error) {
	// Try cache first
	if r.cache != nil {
		var item InventoryItem
		if err := r.cache.Get(ctx, inventoryCachePrefix+sku, &item); err == nil {
			return &item, nil
		}
	}

	// Cache miss - query DB
	item := &InventoryItem{}
	err := r.db.QueryRow(ctx, `
		SELECT id, sku, name, description, quantity, reserved_qty, unit_price, version, created_at, updated_at
		FROM inventory WHERE sku = $1
	`, sku).Scan(&item.ID, &item.SKU, &item.Name, &item.Description, &item.Quantity, &item.ReservedQty, &item.UnitPrice, &item.Version, &item.CreatedAt, &item.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache for next time
	r.cacheItem(ctx, item)
	return item, nil
}

// ReserveStock reserves inventory for an order using optimistic locking
func (r *Repository) ReserveStock(ctx context.Context, orderID string, items []ReserveItemRequest) ([]*Reservation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var reservations []*Reservation

	for _, reqItem := range items {
		// Get current inventory with lock
		var item InventoryItem
		err := tx.QueryRow(ctx, `
			SELECT id, sku, quantity, reserved_qty, version
			FROM inventory WHERE sku = $1 FOR UPDATE
		`, reqItem.SKU).Scan(&item.ID, &item.SKU, &item.Quantity, &item.ReservedQty, &item.Version)

		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("item %s not found", reqItem.SKU)
		}
		if err != nil {
			return nil, err
		}

		// Check availability
		available := item.Quantity - item.ReservedQty
		if available < reqItem.Quantity {
			return nil, fmt.Errorf("insufficient stock for %s: available=%d, requested=%d", reqItem.SKU, available, reqItem.Quantity)
		}

		// Update reserved quantity with optimistic locking
		result, err := tx.Exec(ctx, `
			UPDATE inventory 
			SET reserved_qty = reserved_qty + $1, version = version + 1, updated_at = $2
			WHERE sku = $3 AND version = $4
		`, reqItem.Quantity, time.Now(), reqItem.SKU, item.Version)
		if err != nil {
			return nil, err
		}

		if result.RowsAffected() == 0 {
			return nil, fmt.Errorf("concurrent modification detected for %s, please retry", reqItem.SKU)
		}

		// Create reservation record
		reservation := &Reservation{
			ID:        uuid.New().String(),
			OrderID:   orderID,
			SKU:       reqItem.SKU,
			Quantity:  reqItem.Quantity,
			Status:    "reserved",
			ExpiresAt: time.Now().Add(15 * time.Minute), // 15 min reservation window
			CreatedAt: time.Now(),
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO reservations (id, order_id, sku, quantity, status, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, reservation.ID, reservation.OrderID, reservation.SKU, reservation.Quantity, reservation.Status, reservation.ExpiresAt, reservation.CreatedAt)
		if err != nil {
			return nil, err
		}

		reservations = append(reservations, reservation)

		// Invalidate cache
		r.invalidateCache(ctx, reqItem.SKU)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return reservations, nil
}

// ReleaseStock releases reserved stock for an order
func (r *Repository) ReleaseStock(ctx context.Context, orderID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get reservations for this order
	rows, err := tx.Query(ctx, `
		SELECT sku, quantity FROM reservations 
		WHERE order_id = $1 AND status = 'reserved'
	`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []struct {
		SKU      string
		Quantity int
	}
	for rows.Next() {
		var item struct {
			SKU      string
			Quantity int
		}
		if err := rows.Scan(&item.SKU, &item.Quantity); err != nil {
			return err
		}
		items = append(items, item)
	}

	// Release each reservation
	for _, item := range items {
		_, err := tx.Exec(ctx, `
			UPDATE inventory SET reserved_qty = reserved_qty - $1, updated_at = $2
			WHERE sku = $3
		`, item.Quantity, time.Now(), item.SKU)
		if err != nil {
			return err
		}

		r.invalidateCache(ctx, item.SKU)
	}

	// Mark reservations as released
	_, err = tx.Exec(ctx, `
		UPDATE reservations SET status = 'released' WHERE order_id = $1
	`, orderID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CommitStock commits reserved stock (after successful payment)
func (r *Repository) CommitStock(ctx context.Context, orderID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get reservations for this order
	rows, err := tx.Query(ctx, `
		SELECT sku, quantity FROM reservations 
		WHERE order_id = $1 AND status = 'reserved'
	`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []struct {
		SKU      string
		Quantity int
	}
	for rows.Next() {
		var item struct {
			SKU      string
			Quantity int
		}
		if err := rows.Scan(&item.SKU, &item.Quantity); err != nil {
			return err
		}
		items = append(items, item)
	}

	// Deduct from both quantity and reserved_qty
	for _, item := range items {
		_, err := tx.Exec(ctx, `
			UPDATE inventory 
			SET quantity = quantity - $1, reserved_qty = reserved_qty - $1, updated_at = $2
			WHERE sku = $3
		`, item.Quantity, time.Now(), item.SKU)
		if err != nil {
			return err
		}

		r.invalidateCache(ctx, item.SKU)
	}

	// Mark reservations as committed
	_, err = tx.Exec(ctx, `
		UPDATE reservations SET status = 'committed' WHERE order_id = $1
	`, orderID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetReservationsByOrder retrieves reservations for an order
func (r *Repository) GetReservationsByOrder(ctx context.Context, orderID string) ([]*Reservation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, sku, quantity, status, expires_at, created_at
		FROM reservations WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reservations []*Reservation
	for rows.Next() {
		res := &Reservation{}
		if err := rows.Scan(&res.ID, &res.OrderID, &res.SKU, &res.Quantity, &res.Status, &res.ExpiresAt, &res.CreatedAt); err != nil {
			return nil, err
		}
		reservations = append(reservations, res)
	}
	return reservations, nil
}

func (r *Repository) cacheItem(ctx context.Context, item *InventoryItem) {
	if r.cache != nil {
		r.cache.Set(ctx, inventoryCachePrefix+item.SKU, item, inventoryCacheTTL)
	}
}

func (r *Repository) invalidateCache(ctx context.Context, sku string) {
	if r.cache != nil {
		r.cache.Delete(ctx, inventoryCachePrefix+sku)
	}
}
