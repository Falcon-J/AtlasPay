package order

import (
	"time"
)

// OrderStatus represents order state machine states
type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusConfirmed OrderStatus = "confirmed"
	StatusPaid      OrderStatus = "paid"
	StatusShipped   OrderStatus = "shipped"
	StatusDelivered OrderStatus = "delivered"
	StatusCancelled OrderStatus = "cancelled"
	StatusFailed    OrderStatus = "failed"
)

// Order represents an order in the system
type Order struct {
	ID         string      `json:"id" db:"id"`
	UserID     string      `json:"user_id" db:"user_id"`
	Status     OrderStatus `json:"status" db:"status"`
	TotalPrice float64     `json:"total_price" db:"total_price"`
	Currency   string      `json:"currency" db:"currency"`
	Items      []OrderItem `json:"items" db:"-"`
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at" db:"updated_at"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID         string  `json:"id" db:"id"`
	OrderID    string  `json:"order_id" db:"order_id"`
	SKU        string  `json:"sku" db:"sku"`
	Name       string  `json:"name" db:"name"`
	Quantity   int     `json:"quantity" db:"quantity"`
	UnitPrice  float64 `json:"unit_price" db:"unit_price"`
	TotalPrice float64 `json:"total_price" db:"total_price"`
}

// CreateOrderRequest represents order creation request
type CreateOrderRequest struct {
	Items []CreateOrderItemRequest `json:"items" validate:"required,min=1"`
}

// CreateOrderItemRequest represents an item in order creation
type CreateOrderItemRequest struct {
	SKU      string `json:"sku" validate:"required"`
	Quantity int    `json:"quantity" validate:"required,min=1"`
}

// OrderResponse represents order API response
type OrderResponse struct {
	Order *Order `json:"order"`
}

// OrderListResponse represents order list API response
type OrderListResponse struct {
	Orders     []*Order `json:"orders"`
	TotalCount int64    `json:"total_count"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
}

// OrderFilter represents order filtering options
type OrderFilter struct {
	UserID   string
	Status   OrderStatus
	Page     int
	PageSize int
}

// ValidTransitions defines valid status transitions
var ValidTransitions = map[OrderStatus][]OrderStatus{
	StatusPending:   {StatusConfirmed, StatusCancelled, StatusFailed},
	StatusConfirmed: {StatusPaid, StatusCancelled, StatusFailed},
	StatusPaid:      {StatusShipped, StatusCancelled},
	StatusShipped:   {StatusDelivered},
	StatusDelivered: {},
	StatusCancelled: {},
	StatusFailed:    {StatusPending}, // Allow retry
}

// CanTransitionTo checks if the order can transition to a new status
func (o *Order) CanTransitionTo(newStatus OrderStatus) bool {
	validStatuses, exists := ValidTransitions[o.Status]
	if !exists {
		return false
	}
	for _, status := range validStatuses {
		if status == newStatus {
			return true
		}
	}
	return false
}

// CalculateTotal calculates the total price of the order
func (o *Order) CalculateTotal() {
	var total float64
	for _, item := range o.Items {
		total += item.TotalPrice
	}
	o.TotalPrice = total
}
