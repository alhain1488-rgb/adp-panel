package billing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/store"
)

func newSvc(t *testing.T) (*Service, *store.Store, context.Context) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := store.New(database)
	svc := NewService(st, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, st, context.Background()
}

func seedClient(t *testing.T, st *store.Store, ctx context.Context, name string) *store.Client {
	t.Helper()
	c, err := st.CreateClient(ctx, store.ClientParams{
		Name: name, UUID: "u-" + name, Password: "p", SubscriptionToken: "tok-" + name, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func defaults(enabled bool) Settings {
	return Settings{
		Enabled: enabled, StarRateKopecks: 130,
		TariffWeekKopecks: 7000, TariffMonthKopecks: 20000, TariffYearKopecks: 200000,
		SupportContact: "@x",
	}
}

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	svc, _, ctx := newSvc(t)
	// Unset → defaults.
	cfg, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || cfg.TariffMonthKopecks != defMonthKopecks || cfg.StarRateKopecks != defStarRate || cfg.SupportContact != defSupport {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	cfg, _ = svc.GetSettings(ctx)
	if !cfg.Enabled || cfg.TariffWeekKopecks != 7000 || cfg.StarRateKopecks != 130 {
		t.Fatalf("round-trip mismatch: %+v", cfg)
	}
}

func TestCreditStars_And_Purchase(t *testing.T) {
	svc, st, ctx := newSvc(t)
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedClient(t, st, ctx, "a")

	credited, bal, err := svc.CreditStars(ctx, c.ID, 250, "chg-1")
	if err != nil {
		t.Fatal(err)
	}
	if credited != 250*130 || bal != 250*130 {
		t.Fatalf("credited=%d bal=%d, want %d", credited, bal, 250*130)
	}
	// Duplicate charge is reported and does not double-credit.
	if _, _, err := svc.CreditStars(ctx, c.ID, 250, "chg-1"); !errors.Is(err, store.ErrDuplicateCharge) {
		t.Fatalf("err=%v, want ErrDuplicateCharge", err)
	}

	cb, err := svc.Purchase(ctx, c.ID, "month") // 20000 kopecks
	if err != nil {
		t.Fatal(err)
	}
	if cb.BalanceKopecks != 250*130-20000 {
		t.Fatalf("balance=%d, want %d", cb.BalanceKopecks, 250*130-20000)
	}
	if !cb.Active || !cb.Managed {
		t.Fatalf("expected active+managed, got %+v", cb)
	}

	// Not enough for a year now.
	if _, err := svc.Purchase(ctx, c.ID, "year"); !errors.Is(err, store.ErrInsufficientFunds) {
		t.Fatalf("err=%v, want ErrInsufficientFunds", err)
	}
}

func TestPurchaseExtendsFromExpiry(t *testing.T) {
	svc, st, ctx := newSvc(t)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedClient(t, st, ctx, "b")
	if _, err := svc.ManualAdjust(ctx, c.ID, 100000, "seed"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Purchase(ctx, c.ID, "week"); err != nil { // +7d
		t.Fatal(err)
	}
	cb2, err := svc.Purchase(ctx, c.ID, "week") // stacks +7d
	if err != nil {
		t.Fatal(err)
	}
	got, _ := time.Parse(time.RFC3339, cb2.ActiveUntil)
	want := fixed.Add(14 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("active_until=%v, want %v (durations must stack)", got, want)
	}
}

func TestReconcile_SuspendsExpiredOnly(t *testing.T) {
	svc, st, ctx := newSvc(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedClient(t, st, ctx, "c")
	if _, err := svc.ManualAdjust(ctx, c.ID, 100000, "seed"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Purchase(ctx, c.ID, "week"); err != nil { // active until now+7d
		t.Fatal(err)
	}

	// Still active → stays enabled.
	svc.Reconcile(ctx)
	if got, _ := st.GetClient(ctx, c.ID); !got.Enabled {
		t.Fatal("client should stay enabled while subscription is active")
	}

	// Time passes beyond expiry → suspended.
	svc.now = func() time.Time { return now.Add(30 * 24 * time.Hour) }
	svc.Reconcile(ctx)
	if got, _ := st.GetClient(ctx, c.ID); got.Enabled {
		t.Fatal("client should be suspended after expiry")
	}
}

func TestReconcile_NoopWhenDisabled(t *testing.T) {
	svc, st, ctx := newSvc(t)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedClient(t, st, ctx, "d")
	if _, err := svc.ManualAdjust(ctx, c.ID, 100000, "seed"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Purchase(ctx, c.ID, "week"); err != nil {
		t.Fatal(err)
	}
	// Disable billing, advance past expiry: reconcile must NOT touch the client.
	cfg := defaults(false)
	if err := svc.SetSettings(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return now.Add(30 * 24 * time.Hour) }
	svc.Reconcile(ctx)
	if got, _ := st.GetClient(ctx, c.ID); !got.Enabled {
		t.Fatal("billing disabled: reconcile must not suspend anyone")
	}
}

func TestGrantExtendsWithoutCharging(t *testing.T) {
	svc, st, ctx := newSvc(t)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedClient(t, st, ctx, "e")
	cb, err := svc.Grant(ctx, c.ID, 30, "promo")
	if err != nil {
		t.Fatal(err)
	}
	if cb.BalanceKopecks != 0 {
		t.Fatalf("grant must not charge the wallet, balance=%d", cb.BalanceKopecks)
	}
	if !cb.Active || !cb.Managed {
		t.Fatalf("grant should activate + manage: %+v", cb)
	}
}

// fakeRefunder records Stars refund calls and can fail on demand.
type fakeRefunder struct {
	calls            int
	userID, chargeID string
	err              error
}

func (f *fakeRefunder) RefundStarPayment(_ context.Context, userID, chargeID string) error {
	f.calls++
	f.userID, f.chargeID = userID, chargeID
	return f.err
}

// seedLinkedClient creates a client with a bound Telegram chat.
func seedLinkedClient(t *testing.T, st *store.Store, ctx context.Context, name, chatID string) *store.Client {
	t.Helper()
	if _, err := st.CreateClient(ctx, store.ClientParams{
		Name: name, UUID: "u-" + name, Password: "p", SubscriptionToken: "tok-" + name,
		Enabled: true, TelegramLinkToken: "lnk-" + name,
	}); err != nil {
		t.Fatal(err)
	}
	linked, err := st.LinkClientTelegram(ctx, "lnk-"+name, chatID, name)
	if err != nil {
		t.Fatal(err)
	}
	return linked
}

// topupID credits a Stars payment and returns its ledger id.
func topupID(t *testing.T, svc *Service, st *store.Store, ctx context.Context, clientID, stars int64, charge string) int64 {
	t.Helper()
	if _, _, err := svc.CreditStars(ctx, clientID, stars, charge); err != nil {
		t.Fatal(err)
	}
	txs, err := st.ListBillingTx(ctx, clientID, 1)
	if err != nil {
		t.Fatal(err)
	}
	return txs[0].ID
}

func TestRefundStars(t *testing.T) {
	svc, st, ctx := newSvc(t)
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedLinkedClient(t, st, ctx, "ref", "424242")
	id := topupID(t, svc, st, ctx, c.ID, 100, "chg-a") // 100 ⭐ → 130 ₽

	// Without a transport wired in, nothing may happen to the ledger.
	if _, err := svc.RefundStars(ctx, c.ID, id); !errors.Is(err, ErrNoRefunder) {
		t.Fatalf("err = %v, want ErrNoRefunder", err)
	}
	after, _ := st.GetClient(ctx, c.ID)
	if after.WalletKopecks != 13000 {
		t.Fatalf("wallet = %d, want 13000 (untouched)", after.WalletKopecks)
	}

	fr := &fakeRefunder{}
	svc.SetStarRefunder(fr)
	cb, err := svc.RefundStars(ctx, c.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	if fr.calls != 1 || fr.userID != "424242" || fr.chargeID != "chg-a" {
		t.Fatalf("refunder got (%d calls, user %q, charge %q), want (1, \"424242\", \"chg-a\")",
			fr.calls, fr.userID, fr.chargeID)
	}
	if cb.BalanceKopecks != 0 {
		t.Fatalf("balance = %d, want 0", cb.BalanceKopecks)
	}

	// The refunded top-up is reported as such and is no longer refundable.
	var seen bool
	for _, tx := range cb.Transactions {
		if tx.ID == id {
			seen = true
			if !tx.Refunded || tx.Refundable {
				t.Fatalf("top-up flags = (refunded %v, refundable %v), want (true, false)", tx.Refunded, tx.Refundable)
			}
		}
	}
	if !seen {
		t.Fatal("refunded top-up missing from the ledger snapshot")
	}

	// Second attempt is refused before Telegram is called again.
	if _, err := svc.RefundStars(ctx, c.ID, id); !errors.Is(err, store.ErrAlreadyRefunded) {
		t.Fatalf("err = %v, want ErrAlreadyRefunded", err)
	}
	if fr.calls != 1 {
		t.Fatalf("refunder called %d times, want 1", fr.calls)
	}
}

func TestRefundStars_TelegramFailureLeavesLedgerAlone(t *testing.T) {
	svc, st, ctx := newSvc(t)
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	c := seedLinkedClient(t, st, ctx, "tgfail", "777")
	id := topupID(t, svc, st, ctx, c.ID, 50, "chg-b")

	fr := &fakeRefunder{err: errors.New("telegram: CHARGE_ALREADY_REFUNDED")}
	svc.SetStarRefunder(fr)
	if _, err := svc.RefundStars(ctx, c.ID, id); err == nil {
		t.Fatal("expected the Telegram failure to surface")
	}
	after, _ := st.GetClient(ctx, c.ID)
	if after.WalletKopecks != 6500 {
		t.Fatalf("wallet = %d, want 6500 (untouched after a failed refund)", after.WalletKopecks)
	}
	tx, _ := st.GetBillingTx(ctx, id)
	if tx.RefundedAt != "" {
		t.Fatal("top-up stamped refunded even though Telegram refused")
	}
}

func TestRefundStars_GuardsSpentBalanceAndUnlinkedClients(t *testing.T) {
	svc, st, ctx := newSvc(t)
	if err := svc.SetSettings(ctx, defaults(true)); err != nil {
		t.Fatal(err)
	}
	fr := &fakeRefunder{}
	svc.SetStarRefunder(fr)

	// Spent balance → refused, and Telegram is never touched.
	c := seedLinkedClient(t, st, ctx, "spent", "111")
	id := topupID(t, svc, st, ctx, c.ID, 100, "chg-c")         // 130 ₽
	if _, err := svc.Purchase(ctx, c.ID, "week"); err != nil { // −70 ₽
		t.Fatal(err)
	}
	if _, err := svc.RefundStars(ctx, c.ID, id); !errors.Is(err, store.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
	if fr.calls != 0 {
		t.Fatalf("refunder called %d times on a refusable refund, want 0", fr.calls)
	}

	// A client who never linked Telegram has no payer to refund.
	u := seedClient(t, st, ctx, "unlinked")
	uid := topupID(t, svc, st, ctx, u.ID, 100, "chg-d")
	if _, err := svc.RefundStars(ctx, u.ID, uid); !errors.Is(err, ErrNotLinked) {
		t.Fatalf("err = %v, want ErrNotLinked", err)
	}
	if fr.calls != 0 {
		t.Fatalf("refunder called %d times for an unlinked client, want 0", fr.calls)
	}
}
