package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/atlaspay/platform/internal/common/logger"
	"github.com/atlaspay/platform/pkg/events"
	"github.com/segmentio/kafka-go"
)

// Producer handles Kafka message production
type Producer struct {
	writers map[string]*kafka.Writer
	brokers []string
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

	return nil
}

// Close closes all writers
func (p *Producer) Close() error {
	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Consumer handles Kafka message consumption
type Consumer struct {
	reader  *kafka.Reader
	handler EventHandler
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

// NewConsumer creates a new Kafka consumer
func NewConsumer(brokers []string, topic, groupID string, handler EventHandler) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &Consumer{
		reader:  reader,
		handler: handler,
	}
}

// Start starts consuming messages
func (c *Consumer) Start(ctx context.Context) {
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

			// Parse event
			var event events.Event
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				logger.Error(ctx).Err(err).Msg("failed to unmarshal event")
				c.reader.CommitMessages(ctx, msg)
				continue
			}

			// Add correlation ID to context
			handlerCtx := logger.WithCorrelationID(ctx, event.CorrelationID)

			// Handle event
			if err := c.handler.Handle(handlerCtx, &event); err != nil {
				logger.Error(handlerCtx).
					Err(err).
					Str("event_id", event.ID).
					Str("event_type", string(event.Type)).
					Msg("failed to handle event, sending to DLQ")

				// TODO: Send to DLQ
				// For now, just commit and move on
			}

			// Commit the message
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				logger.Error(ctx).Err(err).Msg("failed to commit message")
			}

			logger.Info(handlerCtx).
				Str("event_id", event.ID).
				Str("event_type", string(event.Type)).
				Msg("event processed")
		}
	}
}

// Close closes the consumer
func (c *Consumer) Close() error {
	return c.reader.Close()
}
