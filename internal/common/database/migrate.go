package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

var migrationFilePattern = regexp.MustCompile(`^([0-9]{3,})_[a-z0-9_]+\.sql$`)

type migrationFile struct {
	name    string
	version int
	sql     string
}

// ApplyMigrations applies ordered SQL files once and records their names in
// PostgreSQL. A transaction-scoped advisory lock prevents the four AtlasPay
// processes from applying the same migration concurrently at startup.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := migrationFiles(dir)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('atlaspay.schema_migrations'))`); err != nil {
		return fmt.Errorf("lock schema migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	for _, file := range files {
		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, file.name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", file.name, err)
		}
		if applied {
			continue
		}

		if _, err := tx.Exec(ctx, file.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", file.name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, file.name); err != nil {
			return fmt.Errorf("record migration %s: %w", file.name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func migrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migration directory %q: %w", dir, err)
	}

	files := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !migrationFilePattern.MatchString(entry.Name()) {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		version, err := strconv.Atoi(migrationFilePattern.FindStringSubmatch(entry.Name())[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %s: %w", entry.Name(), err)
		}
		files = append(files, migrationFile{name: entry.Name(), version: version, sql: string(contents)})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no versioned SQL migrations found in %q", dir)
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].version == files[j].version {
			return files[i].name < files[j].name
		}
		return files[i].version < files[j].version
	})
	for i := 1; i < len(files); i++ {
		if files[i-1].version == files[i].version {
			return nil, fmt.Errorf("duplicate migration version %03d", files[i].version)
		}
	}
	return files, nil
}
