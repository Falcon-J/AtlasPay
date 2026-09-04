package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event stores an event that exhausted retries during async processing.
type Event struct {
	ID               string          `json:"id"`
	Topic            string          `json:"topic"`
	EventType        string          `json:"event_type"`
	AggregateID      string          `json:"aggregate_id"`
	CorrelationID    string          `json:"correlation_id"`
	Payload          json.RawMessage `json:"payload"`
	ErrorMessage     string          `json:"error_message"`
	Attempts         int             `json:"attempts"`
	PublishAttempts  int             `json:"publish_attempts"`
	LastPublishError string          `json:"last_publish_error,omitempty"`
	PublishedAt      *time.Time      `json:"published_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// Repository persists dead-letter events in PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Record(ctx context.Context, event *Event) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO dead_letter_events
			(id, topic, event_type, aggregate_id, correlation_id, payload, error_message, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			topic = EXCLUDED.topic,
			event_type = EXCLUDED.event_type,
			aggregate_id = EXCLUDED.aggregate_id,
			correlation_id = EXCLUDED.correlation_id,
			payload = EXCLUDED.payload,
			error_message = EXCLUDED.error_message,
			attempts = EXCLUDED.attempts
	`, event.ID, event.Topic, event.EventType, event.AggregateID, event.CorrelationID,
		event.Payload, event.ErrorMessage, event.Attempts, event.CreatedAt)
	return err
}

// MarkPublishAttempt records that a Kafka DLQ publication was attempted.
func (r *Repository) MarkPublishAttempt(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE dead_letter_events
		SET publish_attempts = publish_attempts + 1,
		    last_publish_error = NULL
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("dead-letter event %q not found", id)
	}
	return nil
}

// IsPublished reports whether the DLQ event has already been published to
// Kafka. It lets source redelivery remain idempotent after successful DLQ
// publication.
func (r *Repository) IsPublished(ctx context.Context, id string) (bool, error) {
	var publishedAt *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT published_at
		FROM dead_letter_events
		WHERE id = $1
	`, id).Scan(&publishedAt)
	if err == pgx.ErrNoRows {
		return false, fmt.Errorf("dead-letter event %q not found", id)
	}
	if err != nil {
		return false, err
	}
	return publishedAt != nil, nil
}

// MarkPublishFailed records the latest publication error while keeping the
// event eligible for a later retry.
func (r *Repository) MarkPublishFailed(ctx context.Context, id string, publishErr error) error {
	if publishErr == nil {
		return fmt.Errorf("publish error is required")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE dead_letter_events
		SET last_publish_error = $2
		WHERE id = $1
	`, id, publishErr.Error())
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("dead-letter event %q not found", id)
	}
	return nil
}

// MarkPublished records a successful Kafka DLQ publication.
func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE dead_letter_events
		SET published_at = COALESCE(published_at, CURRENT_TIMESTAMP),
		    last_publish_error = NULL
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("dead-letter event %q not found", id)
	}
	return nil
}

func (r *Repository) ListRecent(ctx context.Context, limit int) ([]*Event, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, topic, event_type, aggregate_id, correlation_id, payload, error_message, attempts,
		       publish_attempts, last_publish_error, published_at, created_at
		FROM dead_letter_events
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		if err := rows.Scan(&event.ID, &event.Topic, &event.EventType, &event.AggregateID,
			&event.CorrelationID, &event.Payload, &event.ErrorMessage, &event.Attempts,
			&event.PublishAttempts, &event.LastPublishError, &event.PublishedAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
