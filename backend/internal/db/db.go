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
		// A restore staged by the backup import lands here before the DB is
		// opened, when nothing holds the file — the safe moment to swap.
		if err := applyStagedRestore(path); err != nil {
			return nil, err
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

// applyStagedRestore swaps in a database staged by the backup import as
// "<path>.incoming". It removes the current DB (and any WAL/SHM/journal) and
// renames the staged file into place. A no-op when nothing is staged.
func applyStagedRestore(path string) error {
	incoming := path + ".incoming"
	if _, err := os.Stat(incoming); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("db: stat staged restore: %w", err)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("db: clear old db for restore: %w", err)
		}
	}
	if err := os.Rename(incoming, path); err != nil {
		return fmt.Errorf("db: apply staged restore: %w", err)
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
