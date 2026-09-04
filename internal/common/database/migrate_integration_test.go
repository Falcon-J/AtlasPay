package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplyMigrationsIsIdempotentAgainstPostgres(t *testing.T) {
	if os.Getenv("ATLASPAY_INTEGRATION") != "1" {
		t.Skip("set ATLASPAY_INTEGRATION=1 to run against PostgreSQL")
	}

	ctx := context.Background()
	databaseURL := os.Getenv("ATLASPAY_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://atlaspay:atlaspay_secret@postgres:5432/atlaspay?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	migrationDir := filepath.Join("..", "..", "..", "migrations")
	for i := 0; i < 2; i++ {
		if err := ApplyMigrations(ctx, pool, migrationDir); err != nil {
			t.Fatalf("ApplyMigrations attempt %d: %v", i+1, err)
		}
	}

	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := []string{"001_initial.sql", "002_dlq_publication_state.sql"}
	if len(versions) != len(want) {
		t.Fatalf("applied migrations = %v, want %v", versions, want)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("applied migrations = %v, want %v", versions, want)
		}
	}
}
