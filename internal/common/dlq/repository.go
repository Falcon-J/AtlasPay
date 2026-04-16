package dlq

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event stores an event that exhausted retries during async processing.
type Event struct {
	ID            string          `json:"id"`
	Topic         string          `json:"topic"`
	EventType     string          `json:"event_type"`
	AggregateID   string          `json:"aggregate_id"`
	CorrelationID string          `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload"`
	ErrorMessage  string          `json:"error_message"`
	Attempts      int             `json:"attempts"`
	CreatedAt     time.Time       `json:"created_at"`
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
	`, event.ID, event.Topic, event.EventType, event.AggregateID, event.CorrelationID,
		event.Payload, event.ErrorMessage, event.Attempts, event.CreatedAt)
	return err
}

func (r *Repository) ListRecent(ctx context.Context, limit int) ([]*Event, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, topic, event_type, aggregate_id, correlation_id, payload, error_message, attempts, created_at
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
			&event.CorrelationID, &event.Payload, &event.ErrorMessage, &event.Attempts, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
