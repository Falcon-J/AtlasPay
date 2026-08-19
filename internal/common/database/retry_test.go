package database

import (
	"context"
	"errors"
	"testing"
)

func TestConnectWithRetryReturnsFirstSuccessfulConnection(t *testing.T) {
	attempts := 0

	db, err := connectWithRetry(context.Background(), "unused", func(context.Context, string) (*PostgresDB, error) {
		attempts++
		return &PostgresDB{}, nil
	})
	if err != nil {
		t.Fatalf("connectWithRetry returned error: %v", err)
	}
	if db == nil {
		t.Fatal("connectWithRetry returned nil database")
	}
	if attempts != 1 {
		t.Fatalf("connectWithRetry made %d attempts, want 1", attempts)
	}
}

func TestConnectWithRetryReturnsContextErrorWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	connectErr := errors.New("database unavailable")

	_, err := connectWithRetry(ctx, "unused", func(context.Context, string) (*PostgresDB, error) {
		return nil, connectErr
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("connectWithRetry returned %v, want context.Canceled", err)
	}
}
