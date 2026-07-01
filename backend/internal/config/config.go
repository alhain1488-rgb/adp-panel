// Package config loads and validates the panel's runtime configuration from the
// environment. Required secrets have no defaults — a missing one is a fatal
// startup error, never an invented value.
package config

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Config holds validated runtime settings.
type Config struct {
	// EncryptionKey is the 32-byte AES-256 master key (decoded from base64).
	EncryptionKey []byte
	// JWTSecret signs access/refresh tokens.
	JWTSecret []byte
	// AdminUsername / AdminPassword seed the first admin on initial start.
	AdminUsername string
	AdminPassword string
	// Domain is the public host the panel is served on.
	Domain string
	// DBPath is the SQLite database file path.
	DBPath string
	// HTTPAddr is the listen address for the Go server (behind Caddy).
	HTTPAddr string
	// SubBaseURL is the base URL used when building subscription links.
	SubBaseURL string
}

// Getenv matches os.Getenv; injected for testability.
type Getenv func(string) string

// Load reads configuration from the process environment.
func Load(getenv Getenv) (*Config, error) {
	cfg := &Config{
		AdminUsername: firstNonEmpty(getenv("PANEL_ADMIN_USERNAME"), "admin"),
		AdminPassword: getenv("PANEL_ADMIN_PASSWORD"),
		Domain:        firstNonEmpty(getenv("PANEL_DOMAIN"), "localhost"),
		DBPath:        firstNonEmpty(getenv("DB_PATH"), "/data/panel.db"),
		HTTPAddr:      firstNonEmpty(getenv("PANEL_HTTP_ADDR"), ":8080"),
		SubBaseURL:    strings.TrimRight(getenv("PANEL_SUB_BASE_URL"), "/"),
	}

	var missing []string

	rawKey := getenv("PANEL_ENCRYPTION_KEY")
	if rawKey == "" {
		missing = append(missing, "PANEL_ENCRYPTION_KEY")
	} else {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawKey))
		if err != nil {
			return nil, fmt.Errorf("PANEL_ENCRYPTION_KEY must be valid base64: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("PANEL_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
		}
		cfg.EncryptionKey = key
	}

	jwt := getenv("PANEL_JWT_SECRET")
	if jwt == "" {
		missing = append(missing, "PANEL_JWT_SECRET")
	} else {
		cfg.JWTSecret = []byte(jwt)
	}

	if cfg.AdminPassword == "" {
		missing = append(missing, "PANEL_ADMIN_PASSWORD")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// Derive the subscription base URL from the domain if not set explicitly.
	if cfg.SubBaseURL == "" {
		scheme := "https"
		if cfg.Domain == "localhost" || strings.HasPrefix(cfg.Domain, "localhost:") {
			scheme = "http"
		}
		cfg.SubBaseURL = fmt.Sprintf("%s://%s", scheme, cfg.Domain)
	}

	return cfg, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
