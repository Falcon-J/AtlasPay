package order

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryClaimSagaLease(t *testing.T) {
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

	orderID := "claim-test-" + uuid.New().String()
	defer pool.Exec(ctx, "DELETE FROM saga_claims WHERE order_id = $1", orderID)

	repo := NewRepository(pool, nil)
	claimed, err := repo.ClaimSaga(ctx, orderID, "event-a")
	if err != nil || !claimed {
		t.Fatalf("first event should claim order: claimed=%v err=%v", claimed, err)
	}

	claimed, err = repo.ClaimSaga(ctx, orderID, "event-a")
	if err != nil || !claimed {
		t.Fatalf("same event retry should refresh claim: claimed=%v err=%v", claimed, err)
	}

	claimed, err = repo.ClaimSaga(ctx, orderID, "event-b")
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("different event must not claim an active order")
	}

	if _, err := pool.Exec(ctx, `UPDATE saga_claims SET lease_expires_at = $2 WHERE order_id = $1`, orderID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	claimed, err = repo.ClaimSaga(ctx, orderID, "event-b")
	if err != nil || !claimed {
		t.Fatalf("expired claim should be reclaimable: claimed=%v err=%v", claimed, err)
	}
}
