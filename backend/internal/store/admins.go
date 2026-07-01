package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("store: not found")

// Admin is the single panel administrator.
type Admin struct {
	ID            int64
	Username      string
	PasswordHash  string
	TOTPSecretEnc string
	TOTPEnabled   bool
	CreatedAt     string
	UpdatedAt     string
}

func scanAdmin(row interface {
	Scan(...any) error
}) (*Admin, error) {
	var a Admin
	var enabled int64
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecretEnc, &enabled, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.TOTPEnabled = enabled != 0
	return &a, nil
}

const adminCols = "id, username, password_hash, totp_secret_enc, totp_enabled, created_at, updated_at"

// CountAdmins returns how many admins exist.
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&n)
	return n, err
}

// CreateAdmin inserts a new admin and returns it.
func (s *Store) CreateAdmin(ctx context.Context, username, passwordHash string) (*Admin, error) {
	now := nowRFC3339()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO admins (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username, passwordHash, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetAdminByID(ctx, id)
}

// GetAdminByID looks up an admin by id.
func (s *Store) GetAdminByID(ctx context.Context, id int64) (*Admin, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+adminCols+" FROM admins WHERE id = ?", id)
	return scanAdmin(row)
}

// GetAdminByUsername looks up an admin by username.
func (s *Store) GetAdminByUsername(ctx context.Context, username string) (*Admin, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+adminCols+" FROM admins WHERE username = ?", username)
	return scanAdmin(row)
}

// SetTOTPSecret stores the (encrypted) TOTP secret without enabling it yet.
func (s *Store) SetTOTPSecret(ctx context.Context, id int64, secretEnc string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE admins SET totp_secret_enc = ?, updated_at = ? WHERE id = ?",
		secretEnc, nowRFC3339(), id)
	return err
}

// SetTOTPEnabled flips the 2FA flag.
func (s *Store) SetTOTPEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE admins SET totp_enabled = ?, updated_at = ? WHERE id = ?",
		boolToInt(enabled), nowRFC3339(), id)
	return err
}
