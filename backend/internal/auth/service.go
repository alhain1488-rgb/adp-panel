package auth

import (
	"context"
	"errors"
	"time"

	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/store"
)

// Sentinel errors mapped to HTTP responses by the handlers.
var (
	ErrInvalidCredentials = errors.New("auth: invalid username or password")
	ErrInvalidCode        = errors.New("auth: invalid code")
	ErrTOTPNotSetup       = errors.New("auth: 2FA not set up")
	ErrChallenge          = errors.New("auth: invalid or expired challenge")
)

// Service implements admin authentication and 2FA.
type Service struct {
	store  *store.Store
	cipher *crypto.Cipher
	tokens *TokenManager
}

// NewService builds an auth Service.
func NewService(st *store.Store, cipher *crypto.Cipher, tokens *TokenManager) *Service {
	return &Service{store: st, cipher: cipher, tokens: tokens}
}

// LoginResult is the outcome of a password login.
type LoginResult struct {
	Need2FA     bool
	Token       string
	ExpiresAt   time.Time
	ChallengeID string
	AdminID     int64
}

// SeedAdmin creates the initial admin from config if none exists.
func (s *Service) SeedAdmin(ctx context.Context, username, password string) (bool, error) {
	n, err := s.store.CountAdmins(ctx)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return false, err
	}
	if _, err := s.store.CreateAdmin(ctx, username, hash); err != nil {
		return false, err
	}
	return true, nil
}

// Login verifies credentials. If 2FA is enabled it returns a challenge instead
// of an access token.
func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	admin, err := s.store.GetAdminByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !crypto.VerifyPassword(admin.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	if admin.TOTPEnabled {
		challenge, err := s.tokens.IssueChallenge(admin.ID)
		if err != nil {
			return nil, err
		}
		return &LoginResult{Need2FA: true, ChallengeID: challenge, AdminID: admin.ID}, nil
	}

	token, exp, err := s.tokens.IssueAccess(admin.ID, admin.Username)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, ExpiresAt: exp, AdminID: admin.ID}, nil
}

// VerifyTOTP completes login by checking a TOTP code against a challenge.
func (s *Service) VerifyTOTP(ctx context.Context, challengeID, code string) (*LoginResult, error) {
	claims, err := s.tokens.Parse(challengeID, typeChallenge)
	if err != nil {
		return nil, ErrChallenge
	}
	adminID, err := claims.AdminID()
	if err != nil {
		return nil, ErrChallenge
	}
	admin, err := s.store.GetAdminByID(ctx, adminID)
	if err != nil {
		return nil, ErrChallenge
	}
	secret, err := s.cipher.Decrypt(admin.TOTPSecretEnc)
	if err != nil {
		return nil, ErrTOTPNotSetup
	}
	if !validateTOTP(secret, code) {
		return nil, ErrInvalidCode
	}
	token, exp, err := s.tokens.IssueAccess(admin.ID, admin.Username)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, ExpiresAt: exp, AdminID: admin.ID}, nil
}

// TOTPSetup is returned when enrolling 2FA.
type TOTPSetup struct {
	Secret     string
	OtpauthURL string
	QR         string
}

// SetupTOTP generates a new TOTP secret for the admin, stores it encrypted (not
// yet enabled), and returns the enrollment payload.
func (s *Service) SetupTOTP(ctx context.Context, adminID int64) (*TOTPSetup, error) {
	admin, err := s.store.GetAdminByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	secret, url, qr, err := generateTOTP(admin.Username)
	if err != nil {
		return nil, err
	}
	enc, err := s.cipher.Encrypt(secret)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetTOTPSecret(ctx, adminID, enc); err != nil {
		return nil, err
	}
	return &TOTPSetup{Secret: secret, OtpauthURL: url, QR: qr}, nil
}

// EnableTOTP verifies the first code and turns on 2FA.
func (s *Service) EnableTOTP(ctx context.Context, adminID int64, code string) error {
	admin, err := s.store.GetAdminByID(ctx, adminID)
	if err != nil {
		return err
	}
	if admin.TOTPSecretEnc == "" {
		return ErrTOTPNotSetup
	}
	secret, err := s.cipher.Decrypt(admin.TOTPSecretEnc)
	if err != nil {
		return ErrTOTPNotSetup
	}
	if !validateTOTP(secret, code) {
		return ErrInvalidCode
	}
	return s.store.SetTOTPEnabled(ctx, adminID, true)
}

// Admin returns the admin record.
func (s *Service) Admin(ctx context.Context, adminID int64) (*store.Admin, error) {
	return s.store.GetAdminByID(ctx, adminID)
}

// ParseAccess validates a bearer access token and returns the admin id.
func (s *Service) ParseAccess(token string) (int64, error) {
	claims, err := s.tokens.Parse(token, typeAccess)
	if err != nil {
		return 0, err
	}
	return claims.AdminID()
}
