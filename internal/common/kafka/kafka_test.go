package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/atlaspay/platform/internal/common/dlq"
	"github.com/atlaspay/platform/pkg/events"
	"github.com/segmentio/kafka-go"
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

type fakeMessageReader struct {
	commits   []kafka.Message
	commitErr error
}

func (r *fakeMessageReader) FetchMessage(context.Context) (kafka.Message, error) {
	return kafka.Message{}, errors.New("not implemented for processMessage tests")
}

func (r *fakeMessageReader) CommitMessages(_ context.Context, messages ...kafka.Message) error {
	r.commits = append(r.commits, messages...)
	return r.commitErr
}

func (r *fakeMessageReader) Stats() kafka.ReaderStats { return kafka.ReaderStats{} }

func (r *fakeMessageReader) Close() error { return nil }

type fakeDLQRecorder struct {
	events          []*dlq.Event
	publishedState  map[string]bool
	publishAttempts []string
	failed          []string
	published       []string
	recordErr       error
	attemptErr      error
	failureErr      error
	publishedErr    error
}

func (r *fakeDLQRecorder) Record(_ context.Context, event *dlq.Event) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	copy := *event
	r.events = append(r.events, &copy)
	if r.publishedState == nil {
		r.publishedState = make(map[string]bool)
	}
	return nil
}

func (r *fakeDLQRecorder) IsPublished(_ context.Context, id string) (bool, error) {
	return r.publishedState[id], nil
}

func (r *fakeDLQRecorder) MarkPublishAttempt(_ context.Context, id string) error {
	if r.attemptErr != nil {
		return r.attemptErr
	}
	r.publishAttempts = append(r.publishAttempts, id)
	return nil
}

func (r *fakeDLQRecorder) MarkPublishFailed(_ context.Context, id string, _ error) error {
	if r.failureErr != nil {
		return r.failureErr
	}
	r.failed = append(r.failed, id)
	return nil
}

func (r *fakeDLQRecorder) MarkPublished(_ context.Context, id string) error {
	if r.publishedErr != nil {
		return r.publishedErr
	}
	r.published = append(r.published, id)
	if r.publishedState == nil {
		r.publishedState = make(map[string]bool)
	}
	r.publishedState[id] = true
	return nil
}

type fakeDLQPublisher struct {
	events  []*events.Event
	err     error
	calls   int
	errOnce bool
}

func (p *fakeDLQPublisher) Publish(_ context.Context, _ string, event *events.Event) error {
	p.calls++
	if p.err != nil && (!p.errOnce || p.calls == 1) {
		return p.err
	}
	copy := *event
	p.events = append(p.events, &copy)
	return nil
}

func newTestConsumer(handler EventHandler, recorder *fakeDLQRecorder, publisher *fakeDLQPublisher, reader *fakeMessageReader) *Consumer {
	return &Consumer{
		reader:       reader,
		handler:      handler,
		topic:        events.TopicOrders,
		groupID:      "test-group",
		maxAttempts:  3,
		retryBackoff: time.Nanosecond,
		dlqRecorder:  recorder,
		dlqProducer:  publisher,
	}
}

func validMessage(t *testing.T) kafka.Message {
	t.Helper()
	event, err := events.NewEvent(events.OrderCreated, "order-1", "corr-1", events.OrderCreatedPayload{OrderID: "order-1"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return kafka.Message{Value: body}
}

func TestConsumerProcessMessageAcknowledgesDuplicateDelivery(t *testing.T) {
	handler := &retryHandler{}
	reader := &fakeMessageReader{}
	consumer := newTestConsumer(handler, &fakeDLQRecorder{}, &fakeDLQPublisher{}, reader)
	message := validMessage(t)

	for i := 0; i < 2; i++ {
		if _, err := consumer.processMessage(context.Background(), message); err != nil {
			t.Fatalf("delivery %d returned error: %v", i+1, err)
		}
	}

	if handler.calls != 2 {
		t.Fatalf("expected at-least-once handler delivery twice, got %d calls", handler.calls)
	}
	if len(reader.commits) != 2 {
		t.Fatalf("expected both deliveries to be acknowledged, got %d commits", len(reader.commits))
	}
}

func TestConsumerProcessMessageDeadLettersMalformedPayloadBeforeAcknowledgement(t *testing.T) {
	handler := &retryHandler{}
	reader := &fakeMessageReader{}
	recorder := &fakeDLQRecorder{}
	publisher := &fakeDLQPublisher{}
	consumer := newTestConsumer(handler, recorder, publisher, reader)
	raw := []byte(`{"type":`)

	if _, err := consumer.processMessage(context.Background(), kafka.Message{Value: raw}); err != nil {
		t.Fatalf("malformed message returned error: %v", err)
	}

	if handler.calls != 0 {
		t.Fatalf("malformed payload must not reach business handler, got %d calls", handler.calls)
	}
	if len(recorder.events) != 1 || len(publisher.events) != 1 {
		t.Fatalf("expected one durable record and publication, got %d records and %d publications", len(recorder.events), len(publisher.events))
	}
	if recorder.events[0].Attempts != 1 {
		t.Fatalf("malformed payload should record one parse attempt, got %d", recorder.events[0].Attempts)
	}
	var preserved struct {
		RawPayloadBase64 string `json:"raw_payload_base64"`
	}
	if err := json.Unmarshal(recorder.events[0].Payload, &preserved); err != nil {
		t.Fatalf("malformed payload was not JSON-safe: %v", err)
	}
	if got := preserved.RawPayloadBase64; got != base64.StdEncoding.EncodeToString(raw) {
		t.Fatalf("raw payload was not preserved, got %q", got)
	}
	if recorder.events[0].ID == "" || publisher.events[0].ID != recorder.events[0].ID {
		t.Fatalf("expected stable DLQ identity, record=%q publication=%q", recorder.events[0].ID, publisher.events[0].ID)
	}
	if len(reader.commits) != 1 {
		t.Fatalf("expected malformed message to commit only after DLQ success, got %d commits", len(reader.commits))
	}
}

func TestConsumerProcessMessageDeadLettersSemanticallyMalformedEvents(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "null envelope", body: `null`},
		{name: "empty envelope", body: `{}`},
		{name: "missing event id", body: `{"type":"order.created","aggregate_id":"order-1","correlation_id":"corr-1","version":1,"payload":{"order_id":"order-1"}}`},
		{name: "empty event type", body: `{"id":"event-1","type":"","aggregate_id":"order-1","correlation_id":"corr-1","version":1,"payload":{"order_id":"order-1"}}`},
		{name: "unknown event type", body: `{"id":"event-1","type":"order.unknown","aggregate_id":"order-1","correlation_id":"corr-1","version":1,"payload":{"order_id":"order-1"}}`},
		{name: "invalid payload shape", body: `{"id":"event-1","type":"order.created","aggregate_id":"order-1","correlation_id":"corr-1","version":1,"payload":[]}`},
		{name: "missing order id", body: `{"id":"event-1","type":"order.created","aggregate_id":"order-1","correlation_id":"corr-1","version":1,"payload":{}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &retryHandler{}
			reader := &fakeMessageReader{}
			recorder := &fakeDLQRecorder{}
			publisher := &fakeDLQPublisher{}
			consumer := newTestConsumer(handler, recorder, publisher, reader)

			if _, err := consumer.processMessage(context.Background(), kafka.Message{Value: []byte(tt.body)}); err != nil {
				t.Fatalf("semantically malformed message returned error: %v", err)
			}
			if handler.calls != 0 {
				t.Fatalf("semantically malformed message reached business handler %d times", handler.calls)
			}
			if len(recorder.events) != 1 || len(publisher.events) != 1 || len(reader.commits) != 1 {
				t.Fatalf("expected one DLQ record, publication, and commit; got %d, %d, %d", len(recorder.events), len(publisher.events), len(reader.commits))
			}
		})
	}
}

func TestConsumerProcessMessageDeadLettersAfterRetryExhaustion(t *testing.T) {
	handler := &retryHandler{failuresBeforeSuccess: 99}
	reader := &fakeMessageReader{}
	recorder := &fakeDLQRecorder{}
	publisher := &fakeDLQPublisher{}
	consumer := newTestConsumer(handler, recorder, publisher, reader)

	if _, err := consumer.processMessage(context.Background(), validMessage(t)); err != nil {
		t.Fatalf("retry exhaustion should be acknowledged after DLQ success: %v", err)
	}
	if handler.calls != 3 {
		t.Fatalf("expected three handler attempts, got %d", handler.calls)
	}
	if len(recorder.events) != 1 || len(publisher.events) != 1 || len(reader.commits) != 1 {
		t.Fatalf("expected one DLQ record, publication, and commit; got %d, %d, %d", len(recorder.events), len(publisher.events), len(reader.commits))
	}
}

func TestConsumerProcessMessageDoesNotAcknowledgeWhenDLQFails(t *testing.T) {
	tests := []struct {
		name      string
		recorder  *fakeDLQRecorder
		publisher *fakeDLQPublisher
	}{
		{name: "persistence", recorder: &fakeDLQRecorder{recordErr: errors.New("database unavailable")}, publisher: &fakeDLQPublisher{}},
		{name: "publication", recorder: &fakeDLQRecorder{}, publisher: &fakeDLQPublisher{err: errors.New("kafka unavailable")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakeMessageReader{}
			consumer := newTestConsumer(&retryHandler{failuresBeforeSuccess: 99}, tt.recorder, tt.publisher, reader)
			if _, err := consumer.processMessage(context.Background(), validMessage(t)); err == nil {
				t.Fatal("expected DLQ failure to prevent acknowledgement")
			}
			if len(reader.commits) != 0 {
				t.Fatalf("expected no commit after DLQ %s failure, got %d", tt.name, len(reader.commits))
			}
		})
	}
}

func TestConsumerProcessMessageRetriesDLQPublicationWithStableIdentity(t *testing.T) {
	reader := &fakeMessageReader{}
	recorder := &fakeDLQRecorder{}
	publisher := &fakeDLQPublisher{err: errors.New("temporary kafka outage"), errOnce: true}
	consumer := newTestConsumer(&retryHandler{failuresBeforeSuccess: 99}, recorder, publisher, reader)
	message := validMessage(t)

	if _, err := consumer.processMessage(context.Background(), message); err == nil {
		t.Fatal("expected first DLQ publication to fail")
	}
	if len(reader.commits) != 0 {
		t.Fatal("first DLQ publication failure must not acknowledge the source message")
	}
	if _, err := consumer.processMessage(context.Background(), message); err != nil {
		t.Fatalf("expected redelivery to retry DLQ publication: %v", err)
	}
	if len(recorder.events) != 2 || recorder.events[0].ID != recorder.events[1].ID {
		t.Fatalf("expected stable DLQ record identity across redelivery, got %+v", recorder.events)
	}
	if len(recorder.failed) != 1 || len(recorder.published) != 1 || len(reader.commits) != 1 {
		t.Fatalf("expected one failure, one publication, and one commit; got %d, %d, %d", len(recorder.failed), len(recorder.published), len(reader.commits))
	}
}

func TestConsumerProcessMessageDoesNotRepublishAlreadyPublishedDLQ(t *testing.T) {
	reader := &fakeMessageReader{}
	recorder := &fakeDLQRecorder{}
	publisher := &fakeDLQPublisher{}
	consumer := newTestConsumer(&retryHandler{failuresBeforeSuccess: 99}, recorder, publisher, reader)
	message := validMessage(t)

	if _, err := consumer.processMessage(context.Background(), message); err != nil {
		t.Fatalf("first DLQ handling returned error: %v", err)
	}
	if _, err := consumer.processMessage(context.Background(), message); err != nil {
		t.Fatalf("duplicate DLQ handling returned error: %v", err)
	}
	if publisher.calls != 1 {
		t.Fatalf("expected published DLQ to be sent once across redelivery, got %d publications", publisher.calls)
	}
	if len(reader.commits) != 2 {
		t.Fatalf("expected both source deliveries to be acknowledged, got %d commits", len(reader.commits))
	}
}

func TestConsumerProcessMessageWithRetryKeepsMessageUnacknowledgedUntilDLQRecovers(t *testing.T) {
	reader := &fakeMessageReader{}
	recorder := &fakeDLQRecorder{}
	publisher := &fakeDLQPublisher{err: errors.New("temporary kafka outage"), errOnce: true}
	consumer := newTestConsumer(&retryHandler{failuresBeforeSuccess: 99}, recorder, publisher, reader)
	consumer.retryBackoff = time.Nanosecond

	if _, err := consumer.processMessageWithRetry(context.Background(), validMessage(t)); err != nil {
		t.Fatalf("expected local retry to recover DLQ publication: %v", err)
	}
	if len(reader.commits) != 1 || publisher.calls != 2 {
		t.Fatalf("expected one commit after two DLQ publication attempts, commits=%d publications=%d", len(reader.commits), publisher.calls)
	}
}

type cancelingRetryHandler struct {
	cancel context.CancelFunc
	calls  int
}

func (h *cancelingRetryHandler) Handle(context.Context, *events.Event) error {
	h.calls++
	h.cancel()
	return fmt.Errorf("temporary failure")
}

func TestConsumerHandleWithRetryStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	handler := &cancelingRetryHandler{cancel: cancel}
	consumer := &Consumer{
		handler:      handler,
		topic:        events.TopicOrders,
		maxAttempts:  3,
		retryBackoff: time.Hour,
	}
	event, err := events.NewEvent(events.OrderCreated, "order-1", "corr-1", events.OrderCreatedPayload{OrderID: "order-1"})
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.handleWithRetry(ctx, event)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if handler.calls != 1 {
		t.Fatalf("expected cancellation to stop retries, got %d calls", handler.calls)
	}
}
