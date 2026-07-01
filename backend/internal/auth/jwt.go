package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	typeAccess    = "access"
	typeChallenge = "2fa" // short-lived token between password step and TOTP step
)

// TokenManager issues and validates HS256 JWTs.
type TokenManager struct {
	secret       []byte
	AccessTTL    time.Duration
	ChallengeTTL time.Duration
}

// NewTokenManager builds a TokenManager with sensible default lifetimes.
func NewTokenManager(secret []byte) *TokenManager {
	return &TokenManager{secret: secret, AccessTTL: 24 * time.Hour, ChallengeTTL: 10 * time.Minute}
}

// Claims are the JWT claims carried by panel tokens.
type Claims struct {
	jwt.RegisteredClaims
	Username string `json:"usr,omitempty"`
	Typ      string `json:"typ"`
}

func (t *TokenManager) issue(adminID int64, username, typ string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(adminID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		Username: username,
		Typ:      typ,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// IssueAccess mints an access token for an authenticated admin.
func (t *TokenManager) IssueAccess(adminID int64, username string) (string, time.Time, error) {
	return t.issue(adminID, username, typeAccess, t.AccessTTL)
}

// IssueChallenge mints a short-lived token representing "password ok, 2FA pending".
func (t *TokenManager) IssueChallenge(adminID int64) (string, error) {
	tok, _, err := t.issue(adminID, "", typeChallenge, t.ChallengeTTL)
	return tok, err
}

// Parse validates a token and checks it is of the expected type.
func (t *TokenManager) Parse(token, wantType string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return t.secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, errors.New("auth: invalid token")
	}
	if claims.Typ != wantType {
		return nil, errors.New("auth: wrong token type")
	}
	return claims, nil
}

// AdminID returns the admin id encoded in the claims.
func (c *Claims) AdminID() (int64, error) {
	return strconv.ParseInt(c.Subject, 10, 64)
}
