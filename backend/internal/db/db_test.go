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

	// second open must not re-run migrations or fail: count stays constant.
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer d2.Close()

	var n2 int
	if err := d2.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n2); err != nil {
		t.Fatal(err)
	}
	// Re-open a third time to confirm idempotency across restarts.
	d3, err := Open(path)
	if err != nil {
		t.Fatalf("reopen 3: %v", err)
	}
	defer d3.Close()
	var n3 int
	if err := d3.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n3); err != nil {
		t.Fatal(err)
	}
	if n2 == 0 || n2 != n3 {
		t.Errorf("schema_migrations count not stable: %d then %d", n2, n3)
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
