package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atlaspay/platform/pkg/events"
)

type retryHandler struct {
	failuresBeforeSuccess int
	calls                 int
}

func (h *retryHandler) Handle(ctx context.Context, event *events.Event) error {
	h.calls++
	if h.calls <= h.failuresBeforeSuccess {
		return errors.New("temporary failure")
	}
	return nil
}

func TestConsumerHandleWithRetrySucceedsAfterRetries(t *testing.T) {
	handler := &retryHandler{failuresBeforeSuccess: 2}
	consumer := &Consumer{
		handler:      handler,
		topic:        events.TopicOrders,
		maxAttempts:  3,
		retryBackoff: time.Nanosecond,
	}
	event, err := events.NewEvent(events.OrderCreated, "order-1", "corr-1", events.OrderCreatedPayload{OrderID: "order-1"})
	if err != nil {
		t.Fatal(err)
	}

	if err := consumer.handleWithRetry(context.Background(), event); err != nil {
		t.Fatalf("expected retry to eventually succeed, got %v", err)
	}
	if handler.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", handler.calls)
	}
}

func TestConsumerHandleWithRetryReturnsAfterMaxAttempts(t *testing.T) {
	handler := &retryHandler{failuresBeforeSuccess: 99}
	consumer := &Consumer{
		handler:      handler,
		topic:        events.TopicOrders,
		maxAttempts:  3,
		retryBackoff: time.Nanosecond,
	}
	event, err := events.NewEvent(events.OrderCreated, "order-1", "corr-1", events.OrderCreatedPayload{OrderID: "order-1"})
	if err != nil {
		t.Fatal(err)
	}

	if err := consumer.handleWithRetry(context.Background(), event); err == nil {
		t.Fatal("expected retry failure after max attempts")
	}
	if handler.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", handler.calls)
	}
}
