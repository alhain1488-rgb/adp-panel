// Package backup produces and restores a portable, passphrase-encrypted snapshot
// of the whole panel database. The design goal is painless migration to a new
// host: install the panel with one command, import a backup, and every server,
// inbound and client reappears — with SSH credentials that still work.
//
// The archive is self-contained: it carries a consistent copy of panel.db plus
// the *source* server's AES master key, so encrypted secrets can be read on the
// target. To keep the "master key only lives in PANEL_ENCRYPTION_KEY" invariant
// intact, import does not adopt the old key — instead it re-encrypts every
// secret column under the *target* server's own key, then discards the old one.
//
// On-disk archive layout (the .adpbak file):
//
//	magic("ADPBAK1") | salt(16) | nonce(12) | AES-256-GCM(gzip(tar(meta.json, panel.db)))
//
// The AES key is derived from the passphrase with argon2id; the magic is used as
// GCM additional data. A wrong passphrase fails the GCM tag — it never yields a
// half-decrypted file.
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/argon2"

	_ "modernc.org/sqlite"

	"github.com/adp/panel/internal/crypto"
)

// Archive envelope + KDF parameters. Bumping magic is how we'd version the format.
const (
	magic     = "ADPBAK1"
	saltLen   = 16
	keyLen    = 32 // AES-256
	argonTime = 1
	argonMem  = 64 * 1024 // 64 MiB
	argonPar  = 4
	// maxArchive bounds how much we read from an uploaded file (the DB is tiny;
	// this only guards against absurd inputs).
	maxArchive = 128 << 20 // 128 MiB
)

// Errors returned to callers; the HTTP layer maps these to 4xx.
var (
	ErrNoPassphrase    = errors.New("backup: passphrase required")
	ErrBadArchive      = errors.New("backup: not a valid ADP backup file")
	ErrWrongPassphrase = errors.New("backup: wrong passphrase or corrupted file")
	ErrNoKey           = errors.New("backup: archive has no master key; cannot restore encrypted secrets")
)

// encFields are the columns holding AES-256-GCM ciphertext (the "_enc" columns).
// Import re-encrypts each of these from the source key to the target key. Adding
// an encrypted column anywhere in the schema means adding it here too.
var encFields = []struct{ table, col string }{
	{"admins", "totp_secret_enc"},
	{"servers", "ssh_secret_enc"},
	{"servers", "ssh_passphrase_enc"},
}

// meta is the small JSON header stored inside the archive alongside the DB.
type meta struct {
	PanelVersion string `json:"panel_version"`
	CreatedAt    string `json:"created_at"`
	MasterKeyB64 string `json:"master_key_b64"` // source server's 32-byte AES key
}

// Report summarizes what an import will bring back, for display to the operator.
type Report struct {
	Servers       int    `json:"servers"`
	Inbounds      int    `json:"inbounds"`
	Clients       int    `json:"clients"`
	SourceVersion string `json:"source_version"`
	CreatedAt     string `json:"created_at"`
}

// Service builds and restores backups for one running panel.
type Service struct {
	db      *sql.DB
	dbPath  string
	key     []byte // this server's 32-byte master key
	version string
	now     func() time.Time
}

// NewService wires the backup service. db is the live handle (used for a
// consistent VACUUM INTO snapshot), dbPath is DB_PATH, key is the current
// PANEL_ENCRYPTION_KEY (32 bytes), version stamps the archive metadata.
func NewService(db *sql.DB, dbPath string, key []byte, version string) *Service {
	return &Service{db: db, dbPath: dbPath, key: key, version: version, now: time.Now}
}

// Export returns an encrypted archive of the current database and a suggested
// filename. The snapshot is taken with VACUUM INTO so it is internally
// consistent even while the panel is serving traffic.
func (s *Service) Export(ctx context.Context, passphrase string) ([]byte, string, error) {
	if passphrase == "" {
		return nil, "", ErrNoPassphrase
	}

	dbBytes, err := s.snapshot(ctx)
	if err != nil {
		return nil, "", err
	}

	// Pack meta.json + panel.db into a gzip'd tar.
	m := meta{
		PanelVersion: s.version,
		CreatedAt:    s.now().UTC().Format(time.RFC3339),
		MasterKeyB64: base64.StdEncoding.EncodeToString(s.key),
	}
	metaBytes, err := json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	var packed bytes.Buffer
	gz := gzip.NewWriter(&packed)
	tw := tar.NewWriter(gz)
	if err := writeTar(tw, "meta.json", metaBytes); err != nil {
		return nil, "", err
	}
	if err := writeTar(tw, "panel.db", dbBytes); err != nil {
		return nil, "", err
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gz.Close(); err != nil {
		return nil, "", err
	}

	// Encrypt the whole tarball under a passphrase-derived key.
	sealed, err := sealArchive(passphrase, packed.Bytes())
	if err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("adp-panel-backup-%s.adpbak", s.now().UTC().Format("20060102-150405"))
	return sealed, filename, nil
}

// Import decrypts the archive, re-encrypts every secret under this server's key,
// and stages the restored database as "<db>.incoming". The caller must then
// restart the process; on the next start db.Open swaps the staged file in. The
// live database is never touched if anything fails.
func (s *Service) Import(ctx context.Context, archive []byte, passphrase string) (Report, error) {
	if passphrase == "" {
		return Report{}, ErrNoPassphrase
	}
	if len(archive) > maxArchive {
		return Report{}, ErrBadArchive
	}

	packed, err := openArchive(passphrase, archive)
	if err != nil {
		return Report{}, err
	}
	m, dbBytes, err := unpack(packed)
	if err != nil {
		return Report{}, err
	}
	if m.MasterKeyB64 == "" {
		return Report{}, ErrNoKey
	}
	oldKey, err := base64.StdEncoding.DecodeString(m.MasterKeyB64)
	if err != nil || len(oldKey) != keyLen {
		return Report{}, ErrNoKey
	}

	// Write the extracted DB to a scratch file next to the live one (same
	// filesystem, so the final rename is atomic) and re-encrypt it there.
	tmpPath := s.dbPath + ".restore.tmp"
	cleanupDBFiles(tmpPath)
	if err := os.WriteFile(tmpPath, dbBytes, 0o600); err != nil {
		return Report{}, fmt.Errorf("backup: write scratch db: %w", err)
	}
	defer cleanupDBFiles(tmpPath)

	report, err := s.reencrypt(ctx, tmpPath, oldKey)
	if err != nil {
		return Report{}, err
	}
	report.SourceVersion = m.PanelVersion
	report.CreatedAt = m.CreatedAt

	// Stage for pickup on next start. Rename replaces any prior staged file.
	incoming := s.dbPath + ".incoming"
	cleanupDBFiles(incoming)
	if err := os.Rename(tmpPath, incoming); err != nil {
		return Report{}, fmt.Errorf("backup: stage restore: %w", err)
	}
	return report, nil
}

// snapshot returns a consistent single-file copy of the live database.
func (s *Service) snapshot(ctx context.Context) ([]byte, error) {
	// VACUUM INTO requires a destination that does not yet exist; use a fresh
	// temp directory on the same filesystem as the DB.
	dir, err := os.MkdirTemp(filepath.Dir(s.dbPath), "adp-snap-")
	if err != nil {
		return nil, fmt.Errorf("backup: temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	snapPath := filepath.Join(dir, "panel.db")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", snapPath); err != nil {
		return nil, fmt.Errorf("backup: snapshot: %w", err)
	}
	data, err := os.ReadFile(snapPath)
	if err != nil {
		return nil, fmt.Errorf("backup: read snapshot: %w", err)
	}
	return data, nil
}

// reencrypt opens the scratch DB, rewrites every secret column from the source
// key to this server's key, and returns a Report of its contents.
func (s *Service) reencrypt(ctx context.Context, path string, oldKey []byte) (Report, error) {
	oldCipher, err := crypto.NewCipher(oldKey)
	if err != nil {
		return Report{}, ErrNoKey
	}
	newCipher, err := crypto.NewCipher(s.key)
	if err != nil {
		return Report{}, err
	}

	// Rollback-journal mode leaves no -wal/-shm side files after a clean close,
	// so the single restored file is safe to rename into place.
	rdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(DELETE)")
	if err != nil {
		return Report{}, fmt.Errorf("backup: open scratch db: %w", err)
	}
	rdb.SetMaxOpenConns(1)
	defer func() { _ = rdb.Close() }()

	for _, f := range encFields {
		if err := reencryptColumn(ctx, rdb, f.table, f.col, oldCipher, newCipher); err != nil {
			return Report{}, err
		}
	}

	var rep Report
	if err := rdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM servers").Scan(&rep.Servers); err != nil {
		return Report{}, fmt.Errorf("backup: count servers: %w", err)
	}
	if err := rdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM inbounds").Scan(&rep.Inbounds); err != nil {
		return Report{}, fmt.Errorf("backup: count inbounds: %w", err)
	}
	if err := rdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM clients").Scan(&rep.Clients); err != nil {
		return Report{}, fmt.Errorf("backup: count clients: %w", err)
	}

	if err := rdb.Close(); err != nil {
		return Report{}, fmt.Errorf("backup: close scratch db: %w", err)
	}
	return rep, nil
}

// reencryptColumn rewrites one (table, column) of ciphertext. Empty values mean
// "no secret" and are skipped. A value that will not decrypt under the source
// key means the archive is corrupt or mismatched — a hard error.
func reencryptColumn(ctx context.Context, rdb *sql.DB, table, col string, oldC, newC *crypto.Cipher) error {
	//nolint:gosec // table/col come from the fixed encFields list, not user input.
	rows, err := rdb.QueryContext(ctx, fmt.Sprintf("SELECT id, %s FROM %s WHERE %s != ''", col, table, col))
	if err != nil {
		return fmt.Errorf("backup: scan %s.%s: %w", table, col, err)
	}
	type pair struct {
		id  int64
		val string
	}
	var pending []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.id, &p.val); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backup: read %s.%s: %w", table, col, err)
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	// Close the reader before issuing writes: the single connection can't do both.
	if err := rows.Close(); err != nil {
		return err
	}

	for _, p := range pending {
		plain, err := oldC.Decrypt(p.val)
		if err != nil {
			return fmt.Errorf("backup: %s.%s: %w", table, col, ErrWrongPassphrase)
		}
		enc, err := newC.Encrypt(plain)
		if err != nil {
			return err
		}
		//nolint:gosec // fixed identifiers, value is bound.
		if _, err := rdb.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ?", table, col), enc, p.id); err != nil {
			return fmt.Errorf("backup: rewrite %s.%s: %w", table, col, err)
		}
	}
	return nil
}

// --- archive crypto helpers ------------------------------------------------

func sealArchive(passphrase string, plaintext []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	aead, err := aeadFor(passphrase, salt)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, []byte(magic))

	out := make([]byte, 0, len(magic)+len(salt)+len(nonce)+len(ct))
	out = append(out, magic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

func openArchive(passphrase string, archive []byte) ([]byte, error) {
	if len(archive) < len(magic)+saltLen {
		return nil, ErrBadArchive
	}
	if string(archive[:len(magic)]) != magic {
		return nil, ErrBadArchive
	}
	off := len(magic)
	salt := archive[off : off+saltLen]
	off += saltLen

	aead, err := aeadFor(passphrase, salt)
	if err != nil {
		return nil, err
	}
	ns := aead.NonceSize()
	if len(archive) < off+ns {
		return nil, ErrBadArchive
	}
	nonce := archive[off : off+ns]
	ct := archive[off+ns:]

	plain, err := aead.Open(nil, nonce, ct, []byte(magic))
	if err != nil {
		return nil, ErrWrongPassphrase
	}
	return plain, nil
}

func aeadFor(passphrase string, salt []byte) (cipher.AEAD, error) {
	dk := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMem, argonPar, keyLen)
	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func unpack(packed []byte) (meta, []byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		return meta{}, nil, ErrBadArchive
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	var (
		m       meta
		dbBytes []byte
		haveM   bool
	)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return meta{}, nil, ErrBadArchive
		}
		switch hdr.Name {
		case "meta.json":
			b, err := io.ReadAll(io.LimitReader(tr, 1<<20))
			if err != nil {
				return meta{}, nil, ErrBadArchive
			}
			if err := json.Unmarshal(b, &m); err != nil {
				return meta{}, nil, ErrBadArchive
			}
			haveM = true
		case "panel.db":
			b, err := io.ReadAll(io.LimitReader(tr, maxArchive))
			if err != nil {
				return meta{}, nil, ErrBadArchive
			}
			dbBytes = b
		}
	}
	if !haveM || len(dbBytes) == 0 {
		return meta{}, nil, ErrBadArchive
	}
	return m, dbBytes, nil
}

func writeTar(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// cleanupDBFiles removes a SQLite database file and its WAL/SHM/journal siblings.
func cleanupDBFiles(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		_ = os.Remove(p)
	}
}
