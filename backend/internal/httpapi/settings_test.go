package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSettings_DefaultsAndUpdate(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)

	// Defaults come from config until overridden.
	rec := env.do(t, http.MethodGet, "/api/settings", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get = %d, body %s", rec.Code, rec.Body.String())
	}
	var s settingsDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s.Domain != "panel.test" || s.SubscriptionBaseURL != "http://panel.test" {
		t.Fatalf("defaults = %+v", s)
	}
	if s.HysteriaEngine != "sing-box" {
		t.Errorf("hysteria_engine = %q, want sing-box", s.HysteriaEngine)
	}

	// Update persists domain, sub base url, sync interval, theme.
	rec = env.do(t, http.MethodPut, "/api/settings", token, map[string]any{
		"domain": "vpn.example.com", "subscription_base_url": "https://vpn.example.com",
		"sync_interval_seconds": 300, "theme": "dark",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put = %d, body %s", rec.Code, rec.Body.String())
	}
	var updated settingsDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Domain != "vpn.example.com" || updated.SyncIntervalSeconds != 300 || updated.Theme != "dark" {
		t.Fatalf("updated = %+v", updated)
	}

	// Persisted across requests.
	rec = env.do(t, http.MethodGet, "/api/settings", token, nil)
	var reread settingsDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &reread)
	if reread.Domain != "vpn.example.com" || reread.SubscriptionBaseURL != "https://vpn.example.com" {
		t.Errorf("reread = %+v", reread)
	}
}

func TestSwaggerAndSpec_Served(t *testing.T) {
	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/swagger", "", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("swagger = %d, ct %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = env.do(t, http.MethodGet, "/openapi.yaml", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi = %d", rec.Code)
	}
	if len(rec.Body.Bytes()) < 100 || rec.Header().Get("Content-Type") != "application/yaml" {
		t.Errorf("openapi body/ct wrong: %d bytes, ct %q", rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
}
