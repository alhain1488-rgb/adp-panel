// Package db opens the SQLite database, applies migrations at startup, and
// exposes a readiness check. It uses the pure-Go modernc.org/sqlite driver
// (no cgo) so builds and containers stay simple.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/adp/panel/internal/migrations"
)

// Open connects to the SQLite database at path, applies pending migrations, and
// returns a ready *sql.DB. Use ":memory:" or a "file::memory:" DSN for tests.
func Open(path string) (*sql.DB, error) {
	if isFilePath(path) {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("db: create data dir: %w", err)
			}
		}
	}

	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	// SQLite handles one writer at a time; serialize connections for correctness.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	if err := migrations.Apply(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return sqlDB, nil
}

// Ready reports whether the database is reachable and migrated.
func Ready(ctx context.Context, sqlDB *sql.DB) error {
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("db: not reachable: %w", err)
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		return fmt.Errorf("db: migrations not applied: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: no migrations applied")
	}
	return nil
}

func isFilePath(path string) bool {
	if path == ":memory:" {
		return false
	}
	if strings.HasPrefix(path, "file::memory:") || strings.Contains(path, ":memory:") {
		return false
	}
	return true
}
