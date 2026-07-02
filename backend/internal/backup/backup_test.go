package backup_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/backup"
	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
)

// key32 returns a deterministic 32-byte key seeded by b.
func key32(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

// seedSource creates a migrated DB at path, encrypts a few secrets under
// srcKey, and returns the plaintext values for later comparison.
func seedSource(t *testing.T, path string, srcKey []byte) (totp, sshPw string) {
	t.Helper()
	sdb, err := db.Open(path)
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	defer func() { _ = sdb.Close() }()

	c, err := crypto.NewCipher(srcKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	totp, sshPw = "JBSWY3DPEHPK3PXP", "s3cr3t-root-pw"
	encTotp, _ := c.Encrypt(totp)
	encSSH, _ := c.Encrypt(sshPw)

	ctx := context.Background()
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO admins (username, password_hash, totp_secret_enc, totp_enabled) VALUES ('admin','h',?,1)`,
		encTotp); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	// ssh_passphrase_enc left empty on purpose — the empty case must be skipped.
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO servers (name, host, ssh_secret_enc, ssh_passphrase_enc) VALUES ('node-1','1.2.3.4',?,'')`,
		encSSH); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO inbounds (server_id, tag, protocol, port) VALUES (1,'in-1','vless',443)`); err != nil {
		t.Fatalf("insert inbound: %v", err)
	}
	if _, err := sdb.ExecContext(ctx,
		`INSERT INTO clients (name, uuid, password, subscription_token) VALUES ('c1','u-1','p','tok-1')`); err != nil {
		t.Fatalf("insert client: %v", err)
	}
	return totp, sshPw
}

func TestExportImportReencryptsUnderTargetKey(t *testing.T) {
	ctx := context.Background()
	srcKey, dstKey := key32(0x11), key32(0x22)
	pass := "correct horse battery staple"

	// Source panel on srcKey.
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "panel.db")
	totp, sshPw := seedSource(t, srcPath, srcKey)

	srcDB, err := db.Open(srcPath)
	if err != nil {
		t.Fatalf("reopen source: %v", err)
	}
	exp := backup.NewService(srcDB, srcPath, srcKey, "test-1.0")
	archive, filename, err := exp.Export(ctx, pass)
	_ = srcDB.Close()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(archive) == 0 || filename == "" {
		t.Fatal("empty export")
	}

	// Fresh target panel on a DIFFERENT key. Import must not need srcKey to run.
	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "panel.db")
	imp := backup.NewService(nil, dstPath, dstKey, "test-1.0")
	report, err := imp.Import(ctx, archive, pass)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if report.Servers != 1 || report.Inbounds != 1 || report.Clients != 1 {
		t.Fatalf("report counts = %+v", report)
	}
	if report.SourceVersion != "test-1.0" {
		t.Fatalf("source version = %q", report.SourceVersion)
	}

	// db.Open applies the staged "<db>.incoming" and opens the restored DB.
	if _, err := os.Stat(dstPath + ".incoming"); err != nil {
		t.Fatalf("expected staged restore: %v", err)
	}
	rdb, err := db.Open(dstPath)
	if err != nil {
		t.Fatalf("open restored: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	var encTotp, encSSH, encPass string
	if err := rdb.QueryRowContext(ctx, "SELECT totp_secret_enc FROM admins WHERE username='admin'").Scan(&encTotp); err != nil {
		t.Fatalf("read totp: %v", err)
	}
	if err := rdb.QueryRowContext(ctx, "SELECT ssh_secret_enc, ssh_passphrase_enc FROM servers WHERE id=1").Scan(&encSSH, &encPass); err != nil {
		t.Fatalf("read ssh: %v", err)
	}
	if encPass != "" {
		t.Fatal("empty passphrase column should stay empty")
	}

	// Secrets must now decrypt under the TARGET key...
	dstCipher, _ := crypto.NewCipher(dstKey)
	if got, err := dstCipher.Decrypt(encTotp); err != nil || got != totp {
		t.Fatalf("totp under dst key = %q, %v", got, err)
	}
	if got, err := dstCipher.Decrypt(encSSH); err != nil || got != sshPw {
		t.Fatalf("ssh under dst key = %q, %v", got, err)
	}
	// ...and no longer under the SOURCE key.
	srcCipher, _ := crypto.NewCipher(srcKey)
	if _, err := srcCipher.Decrypt(encSSH); err == nil {
		t.Fatal("secret still decryptable under source key — re-encryption failed")
	}
}

func TestImportWrongPassphrase(t *testing.T) {
	ctx := context.Background()
	srcKey := key32(0x33)
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "panel.db")
	seedSource(t, srcPath, srcKey)

	srcDB, _ := db.Open(srcPath)
	archive, _, err := backup.NewService(srcDB, srcPath, srcKey, "v").Export(ctx, "the-right-one")
	_ = srcDB.Close()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dstPath := filepath.Join(t.TempDir(), "panel.db")
	_, err = backup.NewService(nil, dstPath, key32(0x44), "v").Import(ctx, archive, "the-WRONG-one")
	if !errors.Is(err, backup.ErrWrongPassphrase) {
		t.Fatalf("want ErrWrongPassphrase, got %v", err)
	}
	if _, statErr := os.Stat(dstPath + ".incoming"); !os.IsNotExist(statErr) {
		t.Fatal("no restore should be staged on failure")
	}
}

func TestImportBadArchive(t *testing.T) {
	dstPath := filepath.Join(t.TempDir(), "panel.db")
	svc := backup.NewService(nil, dstPath, key32(0x55), "v")
	_, err := svc.Import(context.Background(), []byte("not an adp backup at all"), "whatever")
	if !errors.Is(err, backup.ErrBadArchive) {
		t.Fatalf("want ErrBadArchive, got %v", err)
	}
}

func TestExportRequiresPassphrase(t *testing.T) {
	// A live DB is needed only so Export gets past its passphrase check; it
	// returns before touching it here.
	var nilDB *sql.DB
	_, _, err := backup.NewService(nilDB, filepath.Join(t.TempDir(), "x.db"), key32(1), "v").Export(context.Background(), "")
	if !errors.Is(err, backup.ErrNoPassphrase) {
		t.Fatalf("want ErrNoPassphrase, got %v", err)
	}
}
