package tribute

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/store"
)

// plainCipher stands in for AES at-rest encryption in tests.
type plainCipher struct{}

func (plainCipher) Encrypt(s string) (string, error) { return "enc:" + s, nil }
func (plainCipher) Decrypt(s string) (string, error) { return s[len("enc:"):], nil }

// recordingGranter captures what the webhook would apply.
type recordingGranter struct {
	calls    int
	clientID int64
	days     int
	chargeID string
	amount   int64
	err      error
}

func (g *recordingGranter) GrantPaid(_ context.Context, clientID int64, days int, _, chargeID, _ string, amount int64) error {
	g.calls++
	g.clientID, g.days, g.chargeID, g.amount = clientID, days, chargeID, amount
	return g.err
}

func newSvc(t *testing.T) (*Service, *store.Store, *recordingGranter, context.Context) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := store.New(database)
	g := &recordingGranter{}
	return NewService(st, plainCipher{}, g, slog.New(slog.NewTextHandler(io.Discard, nil))), st, g, context.Background()
}

func sign(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func purchaseBody(t *testing.T, productID, purchaseID, tgUser, amount int64) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"name":       "new_digital_product",
		"created_at": "2026-07-31T15:00:00Z",
		"payload": map[string]any{
			"product_id":       productID,
			"product_name":     "КВН на 1 мес.",
			"amount":           amount,
			"currency":         "rub",
			"purchase_id":      purchaseID,
			"telegram_user_id": tgUser,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// configure enables Tribute with one product mapped to 30 days.
func configure(t *testing.T, s *Service, ctx context.Context) {
	t.Helper()
	if err := s.SetSettings(ctx, Settings{
		Enabled:  true,
		Products: []Product{{ProductID: 142365, Days: 30, Title: "1 мес"}},
	}, "secret-key"); err != nil {
		t.Fatal(err)
	}
}

// seedLinked creates a client bound to a Telegram chat.
func seedLinked(t *testing.T, st *store.Store, ctx context.Context, chatID string) *store.Client {
	t.Helper()
	if _, err := st.CreateClient(ctx, store.ClientParams{
		Name: "c" + chatID, UUID: "u" + chatID, Password: "p", SubscriptionToken: "tok" + chatID,
		Enabled: false, TelegramLinkToken: "lnk" + chatID,
	}); err != nil {
		t.Fatal(err)
	}
	c, err := st.LinkClientTelegram(ctx, "lnk"+chatID, chatID, "user")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHandleWebhook_GrantsOnValidSignature(t *testing.T) {
	svc, st, g, ctx := newSvc(t)
	configure(t, svc, ctx)
	c := seedLinked(t, st, ctx, "12321321")

	body := purchaseBody(t, 142365, 777, 12321321, 10000)
	if err := svc.HandleWebhook(ctx, body, sign(body, "secret-key")); err != nil {
		t.Fatal(err)
	}
	if g.calls != 1 {
		t.Fatalf("granter called %d times, want 1", g.calls)
	}
	if g.clientID != c.ID || g.days != 30 || g.amount != 10000 {
		t.Fatalf("granted (client %d, %d days, %d kopecks), want (%d, 30, 10000)",
			g.clientID, g.days, g.amount, c.ID)
	}
	// The charge id must carry Tribute's purchase id — that is the idempotency key.
	if g.chargeID != "tribute:777" {
		t.Fatalf("charge id = %q, want tribute:777", g.chargeID)
	}
}

func TestHandleWebhook_RejectsForgedAndTamperedBodies(t *testing.T) {
	svc, st, g, ctx := newSvc(t)
	configure(t, svc, ctx)
	seedLinked(t, st, ctx, "12321321")
	body := purchaseBody(t, 142365, 778, 12321321, 10000)

	for _, tc := range []struct{ name, sig string }{
		{"wrong key", sign(body, "not-the-key")},
		{"empty", ""},
		{"garbage", "deadbeef"},
	} {
		if err := svc.HandleWebhook(ctx, body, tc.sig); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("%s: err = %v, want ErrBadSignature", tc.name, err)
		}
	}

	// A body altered after signing must not verify — this is the attack that
	// matters: bumping the period or pointing the purchase at another client.
	tampered := purchaseBody(t, 142365, 778, 99999999, 10000)
	if err := svc.HandleWebhook(ctx, tampered, sign(body, "secret-key")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered body: err = %v, want ErrBadSignature", err)
	}
	if g.calls != 0 {
		t.Fatalf("granter ran %d times on rejected requests, want 0", g.calls)
	}
}

func TestHandleWebhook_IgnoresWhatItCannotApply(t *testing.T) {
	svc, st, g, ctx := newSvc(t)
	configure(t, svc, ctx)
	seedLinked(t, st, ctx, "12321321")

	// Unmapped product.
	body := purchaseBody(t, 999999, 779, 12321321, 10000)
	if err := svc.HandleWebhook(ctx, body, sign(body, "secret-key")); !errors.Is(err, ErrUnknownProduct) {
		t.Fatalf("unmapped product: err = %v, want ErrUnknownProduct", err)
	}

	// Payer we have no client for.
	body = purchaseBody(t, 142365, 780, 55555555, 10000)
	if err := svc.HandleWebhook(ctx, body, sign(body, "secret-key")); !errors.Is(err, ErrUnknownClient) {
		t.Fatalf("unknown payer: err = %v, want ErrUnknownClient", err)
	}

	// An event type we deliberately do not act on.
	other, _ := json.Marshal(map[string]any{"name": "digital_product_refund", "payload": map[string]any{}})
	if err := svc.HandleWebhook(ctx, other, sign(other, "secret-key")); !errors.Is(err, ErrIgnored) {
		t.Fatalf("refund event: err = %v, want ErrIgnored", err)
	}

	if g.calls != 0 {
		t.Fatalf("granter ran %d times, want 0", g.calls)
	}
}

func TestHandleWebhook_DisabledAndUnconfigured(t *testing.T) {
	svc, _, _, ctx := newSvc(t)
	body := purchaseBody(t, 142365, 781, 12321321, 10000)

	// Off by default.
	if err := svc.HandleWebhook(ctx, body, sign(body, "secret-key")); !errors.Is(err, ErrDisabled) {
		t.Fatalf("err = %v, want ErrDisabled", err)
	}
	// Enabled but no key stored — signatures cannot be checked, so nothing passes.
	if err := svc.SetSettings(ctx, Settings{Enabled: true}, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleWebhook(ctx, body, sign(body, "secret-key")); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestSettingsRoundTrip_KeepsKeySecret(t *testing.T) {
	svc, _, _, ctx := newSvc(t)
	configure(t, svc, ctx)

	cfg, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || !cfg.HasAPIKey || len(cfg.Products) != 1 || cfg.Products[0].Days != 30 {
		t.Fatalf("round-trip mismatch: %+v", cfg)
	}
	// Saving with a blank key must keep the stored one, so the UI need never hold it.
	if err := svc.SetSettings(ctx, Settings{Enabled: true, Products: cfg.Products}, ""); err != nil {
		t.Fatal(err)
	}
	body := purchaseBody(t, 142365, 782, 1, 10000)
	if err := svc.HandleWebhook(ctx, body, sign(body, "secret-key")); !errors.Is(err, ErrUnknownClient) {
		t.Fatalf("key was lost on save: err = %v, want the signature to still verify", err)
	}

	// A mapping missing either half is dropped rather than silently granting 0 days.
	if err := svc.SetSettings(ctx, Settings{
		Enabled:  true,
		Products: []Product{{ProductID: 1, Days: 0}, {ProductID: 0, Days: 30}, {ProductID: 2, Days: 7}},
	}, ""); err != nil {
		t.Fatal(err)
	}
	cfg, _ = svc.GetSettings(ctx)
	if len(cfg.Products) != 1 || cfg.Products[0].ProductID != 2 {
		t.Fatalf("invalid mappings survived: %+v", cfg.Products)
	}
}
