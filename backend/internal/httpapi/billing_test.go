package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/adp/panel/internal/billing"
	"github.com/adp/panel/internal/clients"
)

func TestBilling_SettingsAndClientWallet(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)

	// Defaults are returned until overridden.
	rec := env.do(t, http.MethodGet, "/api/billing/settings", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get settings = %d, body %s", rec.Code, rec.Body.String())
	}
	var cfg billing.Settings
	_ = json.Unmarshal(rec.Body.Bytes(), &cfg)
	if cfg.Enabled || cfg.TariffMonthKopecks == 0 || cfg.StarRateKopecks == 0 {
		t.Fatalf("unexpected default settings: %+v", cfg)
	}

	// Enable billing and set tariffs.
	rec = env.do(t, http.MethodPut, "/api/billing/settings", token, map[string]any{
		"enabled":              true,
		"tariff_week_kopecks":  7000,
		"tariff_month_kopecks": 20000,
		"tariff_year_kopecks":  200000,
		"star_rate_kopecks":    130,
		"support_contact":      "@help",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put settings = %d, body %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cfg)
	if !cfg.Enabled || cfg.SupportContact != "@help" {
		t.Fatalf("settings not saved: %+v", cfg)
	}

	// A client starts with an empty wallet and no subscription.
	c, err := env.clients.Create(context.Background(), clients.Input{Name: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/clients/" + strconv.FormatInt(c.ID, 10) + "/billing"
	rec = env.do(t, http.MethodGet, path, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get client billing = %d, body %s", rec.Code, rec.Body.String())
	}
	var cb billing.ClientBilling
	_ = json.Unmarshal(rec.Body.Bytes(), &cb)
	if cb.BalanceKopecks != 0 || cb.Active || cb.Managed {
		t.Fatalf("fresh client billing = %+v", cb)
	}

	// Operator top-up credits the wallet.
	rec = env.do(t, http.MethodPost, path+"/topup", token, map[string]any{"kopecks": 50000, "detail": "cash"})
	if rec.Code != http.StatusOK {
		t.Fatalf("topup = %d, body %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cb)
	if cb.BalanceKopecks != 50000 {
		t.Fatalf("balance after top-up = %d, want 50000", cb.BalanceKopecks)
	}

	// Operator grant activates a subscription without charging.
	rec = env.do(t, http.MethodPost, path+"/grant", token, map[string]any{"tariff": "month"})
	if rec.Code != http.StatusOK {
		t.Fatalf("grant = %d, body %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cb)
	if !cb.Active || !cb.Managed || cb.BalanceKopecks != 50000 {
		t.Fatalf("after grant = %+v (balance must be unchanged)", cb)
	}
	if len(cb.Transactions) < 2 {
		t.Fatalf("expected top-up + grant in the ledger, got %d rows", len(cb.Transactions))
	}
}
