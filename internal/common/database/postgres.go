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
