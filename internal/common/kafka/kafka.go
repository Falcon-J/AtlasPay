package kafka

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/atlaspay/platform/internal/common/dlq"
	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/internal/common/metrics"
	"github.com/atlaspay/platform/pkg/events"
	"github.com/segmentio/kafka-go"
)

// Producer handles Kafka message production
type Producer struct {
	writers map[string]*kafka.Writer
	brokers []string
	mu      sync.Mutex
	lifeMu  sync.RWMutex
}

// NewProducer creates a new Kafka producer
func NewProducer(brokers []string) *Producer {
	return &Producer{
		writers: make(map[string]*kafka.Writer),
		brokers: brokers,
	}
}

// getWriter returns a writer for the given topic (lazy initialization)
func (p *Producer) getWriter(topic string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, exists := p.writers[topic]; exists {
		return w
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(p.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  3,
		BatchSize:    1,
		BatchTimeout: 10 * time.Millisecond,
		Async:        false, // Synchronous for guaranteed delivery
	}
	p.writers[topic] = w
	return w
}

// Publish publishes an event to a topic
func (p *Producer) Publish(ctx context.Context, topic string, event *events.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(event.AggregateID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.Type)},
			{Key: "correlation_id", Value: []byte(event.CorrelationID)},
		},
		Time: time.Now(),
	}

	p.lifeMu.RLock()
	defer p.lifeMu.RUnlock()
	writer := p.getWriter(topic)
	err = writer.WriteMessages(ctx, msg)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Str("topic", topic).
			Str("event_type", string(event.Type)).
			Msg("failed to publish event")
		return err
	}

	logger.Info(ctx).
		Str("topic", topic).
		Str("event_id", event.ID).
		Str("event_type", string(event.Type)).
		Str("aggregate_id", event.AggregateID).
		Msg("event published")

	metrics.RecordKafkaProduced(topic)
	return nil
}

// Close closes all writers
func (p *Producer) Close() error {
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Consumer handles Kafka message consumption
type Consumer struct {
	reader       messageReader
	handler      EventHandler
	topic        string
	groupID      string
	maxAttempts  int
	retryBackoff time.Duration
	dlqRecorder  deadLetterRecorder
	dlqProducer  eventPublisher
}

type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Stats() kafka.ReaderStats
	Close() error
}

type deadLetterRecorder interface {
	Record(context.Context, *dlq.Event) error
	IsPublished(context.Context, string) (bool, error)
	MarkPublishAttempt(context.Context, string) error
	MarkPublishFailed(context.Context, string, error) error
	MarkPublished(context.Context, string) error
}

type eventPublisher interface {
	Publish(context.Context, string, *events.Event) error
}

// EventHandler processes events
type EventHandler interface {
	Handle(ctx context.Context, event *events.Event) error
}

// EventHandlerFunc is a function type that implements EventHandler
type EventHandlerFunc func(ctx context.Context, event *events.Event) error

func (f EventHandlerFunc) Handle(ctx context.Context, event *events.Event) error {
	return f(ctx, event)
}

// NewConsumerWithOptions creates a Kafka consumer with retry and DLQ support.
func NewConsumerWithOptions(
	brokers []string,
	topic, groupID string,
	handler EventHandler,
	dlqRecorder *dlq.Repository,
	dlqProducer *Producer,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
		MaxWait:  time.Second,
		// A zero interval makes CommitMessages synchronous. The consumer only
		// acknowledges after the broker confirms the offset commit.
		CommitInterval: 0,
		StartOffset:    kafka.LastOffset,
	})

	return &Consumer{
		reader:       reader,
		handler:      handler,
		topic:        topic,
		groupID:      groupID,
		maxAttempts:  3,
		retryBackoff: 250 * time.Millisecond,
		dlqRecorder:  dlqRecorder,
		dlqProducer:  dlqProducer,
	}
}

// Start starts consuming messages
func (c *Consumer) Start(ctx context.Context) {
	// Start a background goroutine to record lag metrics every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := c.reader.Stats()
				metrics.RecordKafkaLag(c.topic, c.groupID, "0", stats.Lag)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error(ctx).Err(err).Msg("failed to fetch message")
				continue
			}

			// Process the message before committing its offset. A processing error
			// leaves the offset uncommitted so a restart can replay the message.
			event, err := c.processMessageWithRetry(ctx, msg)
			if err != nil {
				logger.Error(ctx).Err(err).Msg("failed to process Kafka message without acknowledgement")
				return
			}

			if event != nil {
				logger.Info(logger.WithCorrelationID(ctx, event.CorrelationID)).
					Str("event_id", event.ID).
					Str("event_type", string(event.Type)).
					Msg("event processed")
			}
			metrics.RecordKafkaConsumed(c.topic, c.groupID)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) (*events.Event, error) {
	// Parse event. Invalid JSON cannot reach the business handler, but it must
	// still be durable and observable before the source offset is committed.
	var event events.Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		malformed := malformedEvent(c.topic, msg.Value)
		if err := c.deadLetterAndCommit(ctx, msg, malformed, fmt.Errorf("invalid event JSON: %w", err)); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err := event.Validate(); err != nil {
		malformed := malformedEvent(c.topic, msg.Value)
		if err := c.deadLetterAndCommit(ctx, msg, malformed, fmt.Errorf("invalid event envelope: %w", err)); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Add correlation ID to context
	handlerCtx := logger.WithCorrelationID(ctx, event.CorrelationID)

	// Handle event with bounded retries before committing to the group.
	if err := c.handleWithRetry(handlerCtx, &event); err != nil {
		logger.Error(handlerCtx).
			Err(err).
			Str("event_id", event.ID).
			Str("event_type", string(event.Type)).
			Int("attempts", c.maxAttempts).
			Msg("failed to handle event after retries, sending to DLQ")

		if dlqErr := c.recordDLQ(handlerCtx, &event, err, c.maxAttempts); dlqErr != nil {
			return nil, fmt.Errorf("dead-letter event %q: %w", event.ID, dlqErr)
		}
	}

	// Commit the message
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		return nil, fmt.Errorf("commit Kafka message: %w", err)
	}
	return &event, nil
}

func (c *Consumer) processMessageWithRetry(ctx context.Context, msg kafka.Message) (*events.Event, error) {
	for {
		event, err := c.processMessage(ctx, msg)
		if err == nil {
			return event, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		logger.Error(ctx).Err(err).Msg("message remains unacknowledged; retrying processing")
		if err := waitForRetry(ctx, c.retryBackoff); err != nil {
			return nil, err
		}
	}
}

func (c *Consumer) handleWithRetry(ctx context.Context, event *events.Event) error {
	var err error
	eventType := string(event.Type)
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		err = c.handler.Handle(ctx, event)
		if err == nil {
			metrics.RecordKafkaEventAttempt(c.topic, eventType, "success")
			return nil
		}

		metrics.RecordKafkaEventAttempt(c.topic, eventType, "failed")
		if attempt < c.maxAttempts {
			metrics.RecordKafkaRetry(c.topic, eventType)
			if err := waitForRetry(ctx, c.retryBackoff*time.Duration(attempt)); err != nil {
				return err
			}
		}
	}
	return err
}

func (c *Consumer) recordDLQ(ctx context.Context, event *events.Event, handlerErr error, attempts int) error {
	if c.dlqRecorder == nil {
		return fmt.Errorf("dead-letter recorder is not configured")
	}

	dlqID := stableDLQID(c.topic, event)
	if err := c.dlqRecorder.Record(ctx, &dlq.Event{
		ID:            dlqID,
		Topic:         c.topic,
		EventType:     string(event.Type),
		AggregateID:   event.AggregateID,
		CorrelationID: event.CorrelationID,
		Payload:       event.Payload,
		ErrorMessage:  handlerErr.Error(),
		Attempts:      attempts,
	}); err != nil {
		return err
	}
	metrics.RecordDeadLetterEvent(c.topic, string(event.Type))
	published, err := c.dlqRecorder.IsPublished(ctx, dlqID)
	if err != nil {
		return err
	}
	if published {
		return nil
	}

	if c.dlqProducer != nil {
		dlqEvent, err := events.NewEvent(events.EventType("event.dead_lettered"), event.AggregateID, event.CorrelationID, map[string]interface{}{
			"topic":       c.topic,
			"event_type":  event.Type,
			"event_id":    event.ID,
			"error":       handlerErr.Error(),
			"attempts":    attempts,
			"failed_body": json.RawMessage(event.Payload),
		})
		if err != nil {
			return err
		}
		dlqEvent.ID = dlqID
		if err := c.dlqRecorder.MarkPublishAttempt(ctx, dlqID); err != nil {
			return err
		}
		if err := c.dlqProducer.Publish(ctx, events.TopicDLQ, dlqEvent); err != nil {
			if stateErr := c.dlqRecorder.MarkPublishFailed(ctx, dlqID, err); stateErr != nil {
				return fmt.Errorf("publish dead-letter event: %w; record publish failure: %v", err, stateErr)
			}
			return err
		}
		if err := c.dlqRecorder.MarkPublished(ctx, dlqID); err != nil {
			return err
		}
	}

	return nil
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Consumer) deadLetterAndCommit(ctx context.Context, msg kafka.Message, event *events.Event, processingErr error) error {
	if err := c.recordDLQ(ctx, event, processingErr, 1); err != nil {
		return fmt.Errorf("dead-letter malformed event: %w", err)
	}
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		return fmt.Errorf("commit malformed Kafka message: %w", err)
	}
	return nil
}

func malformedEvent(topic string, raw []byte) *events.Event {
	payload, _ := json.Marshal(map[string]string{
		"raw_payload_base64": base64.StdEncoding.EncodeToString(raw),
	})
	return &events.Event{
		ID:      stableDLQID(topic, &events.Event{Payload: raw}),
		Type:    events.EventType("event.malformed"),
		Payload: payload,
	}
}

func stableDLQID(topic string, event *events.Event) string {
	source := event.ID
	if source == "" {
		source = string(event.Payload)
	}
	hash := sha1.Sum([]byte(topic + "\x00" + source))
	return fmt.Sprintf("%x-%x-%x-%x-%x", hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}

// Close closes the consumer
func (c *Consumer) Close() error {
	return c.reader.Close()
}
