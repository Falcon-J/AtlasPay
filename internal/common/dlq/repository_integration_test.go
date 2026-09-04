package dlq

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryDLQPublicationStateIsDurable(t *testing.T) {
	if os.Getenv("ATLASPAY_INTEGRATION") != "1" {
		t.Skip("set ATLASPAY_INTEGRATION=1 to run against PostgreSQL")
	}

	ctx := context.Background()
	dbURL := os.Getenv("ATLASPAY_TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://atlaspay:atlaspay_secret@postgres:5432/atlaspay?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	eventID := uuid.New().String()
	defer pool.Exec(ctx, "DELETE FROM dead_letter_events WHERE id = $1", eventID)

	repo := NewRepository(pool)
	event := &Event{
		ID:            eventID,
		Topic:         "atlaspay.orders",
		EventType:     "order.created",
		AggregateID:   "order-1",
		CorrelationID: "correlation-1",
		Payload:       json.RawMessage(`{"order_id":"order-1"}`),
		ErrorMessage:  "payment unavailable",
		Attempts:      3,
	}

	if err := repo.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	published, err := repo.IsPublished(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if published {
		t.Fatal("newly recorded DLQ event must not be published")
	}
	if err := repo.MarkPublishAttempt(ctx, eventID); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPublishFailed(ctx, eventID, os.ErrDeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPublished(ctx, eventID); err != nil {
		t.Fatal(err)
	}

	published, err = repo.IsPublished(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Fatal("published DLQ event must remain marked after the state transition")
	}

	if err := repo.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM dead_letter_events WHERE id = $1", eventID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("redelivered DLQ event created %d rows, want 1", count)
	}
}
