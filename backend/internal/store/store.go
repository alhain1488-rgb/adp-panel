// Package store is the typed data-access layer over the SQLite database.
// Queries are hand-written (see PROGRESS.md for why sqlc was not used); each
// method returns typed models. Timestamps are managed in Go as RFC3339 strings
// so the API emits consistent ISO 8601 values.
package store

import (
	"database/sql"
	"time"
)

// Store wraps the database handle.
type Store struct {
	db *sql.DB
}

// New builds a Store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// nowRFC3339 is the canonical timestamp format used for all stored times.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
