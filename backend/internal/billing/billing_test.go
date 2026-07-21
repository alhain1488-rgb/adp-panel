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
