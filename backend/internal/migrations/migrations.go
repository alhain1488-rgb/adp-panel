// Package migrations embeds the SQL schema and applies pending migrations at
// startup. Migrations are additive and forward-only in normal operation: each
// *.up.sql file is applied once, inside a transaction, and recorded in
// schema_migrations. Never edit an applied migration — add a new one.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.sql
var files embed.FS

type migration struct {
	version int
	name    string
	upSQL   string
}

// Apply runs all pending up-migrations against db. It is idempotent.
func Apply(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("migrations: create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	all, err := load()
	if err != nil {
		return err
	}

	for _, m := range all {
		if applied[m.version] {
			continue
		}
		if err := applyOne(db, m); err != nil {
			return fmt.Errorf("migrations: apply %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

func applyOne(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(m.upSQL); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
		return err
	}
	return tx.Commit()
}

func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func load() ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version, label, err := parseName(name)
		if err != nil {
			return nil, err
		}
		content, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: version, name: label, upSQL: string(content)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// parseName turns "0001_init.up.sql" into (1, "init").
func parseName(name string) (int, string, error) {
	base := strings.TrimSuffix(name, ".up.sql")
	idx := strings.Index(base, "_")
	if idx <= 0 {
		return 0, "", fmt.Errorf("migrations: bad filename %q", name)
	}
	version, err := strconv.Atoi(base[:idx])
	if err != nil {
		return 0, "", fmt.Errorf("migrations: bad version in %q: %w", name, err)
	}
	return version, base[idx+1:], nil
}
