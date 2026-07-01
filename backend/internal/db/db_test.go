package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpen_MigratesAndReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	// core tables exist
	for _, table := range []string{"admins", "servers", "inbounds", "clients", "client_inbounds", "audit_logs", "settings"} {
		var name string
		err := database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}

	if err := Ready(context.Background(), database); err != nil {
		t.Errorf("Ready: %v", err)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.db")
	d1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d1.Close()

	// second open must not re-run migrations or fail
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer d2.Close()

	var n int
	if err := d2.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("schema_migrations count = %d, want 1", n)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.db")
	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// inbound referencing a non-existent server must be rejected
	_, err = database.Exec("INSERT INTO inbounds (server_id, tag, protocol, port) VALUES (999, 'x', 'vless', 443)")
	if err == nil {
		t.Error("expected foreign key violation, got nil")
	}
}
