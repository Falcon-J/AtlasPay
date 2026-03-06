package inventory

import (
	"time"
)

// InventoryItem represents stock for a product
type InventoryItem struct {
	ID          string    `json:"id" db:"id"`
	SKU         string    `json:"sku" db:"sku"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Quantity    int       `json:"quantity" db:"quantity"`
	ReservedQty int       `json:"reserved_qty" db:"reserved_qty"`
	UnitPrice   float64   `json:"unit_price" db:"unit_price"`
	Version     int       `json:"version" db:"version"` // For optimistic locking
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// AvailableQuantity returns the available (non-reserved) quantity
func (i *InventoryItem) AvailableQuantity() int {
	return i.Quantity - i.ReservedQty
}

// Reservation represents a stock reservation
type Reservation struct {
	ID        string    `json:"id" db:"id"`
	OrderID   string    `json:"order_id" db:"order_id"`
	SKU       string    `json:"sku" db:"sku"`
	Quantity  int       `json:"quantity" db:"quantity"`
	Status    string    `json:"status" db:"status"` // reserved, committed, released
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ReserveRequest represents stock reservation request
type ReserveRequest struct {
	OrderID string               `json:"order_id" validate:"required"`
	Items   []ReserveItemRequest `json:"items" validate:"required,min=1"`
}

// ReserveItemRequest represents an item to reserve
type ReserveItemRequest struct {
	SKU      string `json:"sku" validate:"required"`
	Quantity int    `json:"quantity" validate:"required,min=1"`
}

// ReleaseRequest represents stock release request
type ReleaseRequest struct {
	OrderID string `json:"order_id" validate:"required"`
}

// InventoryResponse represents inventory API response
type InventoryResponse struct {
	Item *InventoryItem `json:"item"`
}

// ReservationResponse represents reservation API response
type ReservationResponse struct {
	Reservations []*Reservation `json:"reservations"`
	Success      bool           `json:"success"`
}
