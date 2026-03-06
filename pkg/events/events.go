package events

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	// Order events
	OrderCreated   EventType = "order.created"
	OrderConfirmed EventType = "order.confirmed"
	OrderCancelled EventType = "order.cancelled"
	OrderFailed    EventType = "order.failed"

	// Inventory events
	InventoryReserved  EventType = "inventory.reserved"
	InventoryFailed    EventType = "inventory.failed"
	InventoryReleased  EventType = "inventory.released"
	InventoryCommitted EventType = "inventory.committed"

	// Payment events
	PaymentProcessed EventType = "payment.processed"
	PaymentFailed    EventType = "payment.failed"
	PaymentRefunded  EventType = "payment.refunded"
)

// Topics
const (
	TopicOrders    = "atlaspay.orders"
	TopicInventory = "atlaspay.inventory"
	TopicPayments  = "atlaspay.payments"
	TopicDLQ       = "atlaspay.dlq"
)

// Event represents a base event structure
type Event struct {
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	AggregateID   string          `json:"aggregate_id"`
	CorrelationID string          `json:"correlation_id"`
	Timestamp     time.Time       `json:"timestamp"`
	Version       int             `json:"version"`
	Payload       json.RawMessage `json:"payload"`
}

// OrderCreatedPayload represents order created event data
type OrderCreatedPayload struct {
	OrderID    string      `json:"order_id"`
	UserID     string      `json:"user_id"`
	Items      []OrderItem `json:"items"`
	TotalPrice float64     `json:"total_price"`
	Currency   string      `json:"currency"`
}

// OrderItem represents an item in order events
type OrderItem struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

// InventoryReservedPayload represents inventory reserved event data
type InventoryReservedPayload struct {
	OrderID      string               `json:"order_id"`
	Reservations []ReservationDetails `json:"reservations"`
}

// ReservationDetails represents reservation info
type ReservationDetails struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// InventoryFailedPayload represents inventory failure event data
type InventoryFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
	SKU     string `json:"sku"`
}

// PaymentProcessedPayload represents payment processed event data
type PaymentProcessedPayload struct {
	PaymentID string  `json:"payment_id"`
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Status    string  `json:"status"`
}

// PaymentFailedPayload represents payment failure event data
type PaymentFailedPayload struct {
	PaymentID string `json:"payment_id"`
	OrderID   string `json:"order_id"`
	Reason    string `json:"reason"`
}

// OrderFailedPayload represents order failure event data
type OrderFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
	Step    string `json:"step"`
}

// NewEvent creates a new event
func NewEvent(eventType EventType, aggregateID, correlationID string, payload interface{}) (*Event, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:            generateEventID(),
		Type:          eventType,
		AggregateID:   aggregateID,
		CorrelationID: correlationID,
		Timestamp:     time.Now(),
		Version:       1,
		Payload:       payloadBytes,
	}, nil
}

// UnmarshalPayload unmarshals the event payload into the given struct
func (e *Event) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(e.Payload, v)
}

func generateEventID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano()/1e6, randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
