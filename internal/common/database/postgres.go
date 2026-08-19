package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB represents a PostgreSQL database connection pool
type PostgresDB struct {
	Pool *pgxpool.Pool
}

// NewPostgresDB creates a new PostgreSQL connection pool
func NewPostgresDB(ctx context.Context, databaseURL string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	// Connection pool settings for high performance
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &PostgresDB{Pool: pool}, nil
}

// ConnectWithRetry connects to PostgreSQL with the retry policy shared by the
// standalone application services.
func ConnectWithRetry(ctx context.Context, databaseURL string) (*PostgresDB, error) {
	return connectWithRetry(ctx, databaseURL, NewPostgresDB)
}

func connectWithRetry(
	ctx context.Context,
	databaseURL string,
	connect func(context.Context, string) (*PostgresDB, error),
) (*PostgresDB, error) {
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		db, err := connect(ctx, databaseURL)
		if err == nil {
			return db, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return nil, lastErr
}

// Close closes the database connection pool
func (db *PostgresDB) Close() {
	db.Pool.Close()
}

// Health checks the database health
func (db *PostgresDB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// ExecScript runs a raw SQL script (useful for migrations)
func (db *PostgresDB) ExecScript(ctx context.Context, script string) error {
	_, err := db.Pool.Exec(ctx, script)
	return err
}

// Stats returns pool statistics (useful for metrics)
func (db *PostgresDB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}
