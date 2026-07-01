package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func validKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func envFrom(m map[string]string) Getenv {
	return func(k string) string { return m[k] }
}

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load(envFrom(map[string]string{
		"PANEL_ENCRYPTION_KEY": validKey(),
		"PANEL_JWT_SECRET":     "supersecret",
		"PANEL_ADMIN_PASSWORD": "hunter2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("EncryptionKey len = %d, want 32", len(cfg.EncryptionKey))
	}
	if cfg.AdminUsername != "admin" {
		t.Errorf("AdminUsername = %q, want default admin", cfg.AdminUsername)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want default :8080", cfg.HTTPAddr)
	}
	if cfg.SubBaseURL != "http://localhost" {
		t.Errorf("SubBaseURL = %q, want http://localhost", cfg.SubBaseURL)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	_, err := Load(envFrom(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
	for _, want := range []string{"PANEL_ENCRYPTION_KEY", "PANEL_JWT_SECRET", "PANEL_ADMIN_PASSWORD"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %s", err.Error(), want)
		}
	}
}

func TestLoad_BadKeyLength(t *testing.T) {
	_, err := Load(envFrom(map[string]string{
		"PANEL_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"PANEL_JWT_SECRET":     "x",
		"PANEL_ADMIN_PASSWORD": "y",
	}))
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("expected 32-byte error, got %v", err)
	}
}

func TestLoad_BadKeyBase64(t *testing.T) {
	_, err := Load(envFrom(map[string]string{
		"PANEL_ENCRYPTION_KEY": "not base64!!!",
		"PANEL_JWT_SECRET":     "x",
		"PANEL_ADMIN_PASSWORD": "y",
	}))
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("expected base64 error, got %v", err)
	}
}

func TestLoad_CustomDomainHTTPS(t *testing.T) {
	cfg, err := Load(envFrom(map[string]string{
		"PANEL_ENCRYPTION_KEY": validKey(),
		"PANEL_JWT_SECRET":     "x",
		"PANEL_ADMIN_PASSWORD": "y",
		"PANEL_DOMAIN":         "panel.example.com",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SubBaseURL != "https://panel.example.com" {
		t.Errorf("SubBaseURL = %q", cfg.SubBaseURL)
	}
}
