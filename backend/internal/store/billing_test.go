package store

import (
	"errors"
	"testing"
	"time"
)

func TestCreditWallet_Idempotent(t *testing.T) {
	st, ctx := newStore(t)
	c, err := st.CreateClient(ctx, ClientParams{Name: "a", UUID: "u-a", Password: "p", SubscriptionToken: "tok-a", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	bal, err := st.CreditWallet(ctx, c.ID, 1000, BillingTx{Kind: "topup", Method: "stars", AmountKopecks: 1000, Stars: 10, ChargeID: "chg1"})
	if err != nil {
		t.Fatal(err)
	}
	if bal != 1000 {
		t.Fatalf("balance = %d, want 1000", bal)
	}

	// Re-crediting the same charge id is a no-op.
	if _, err := st.CreditWallet(ctx, c.ID, 1000, BillingTx{Kind: "topup", ChargeID: "chg1"}); !errors.Is(err, ErrDuplicateCharge) {
		t.Fatalf("err = %v, want ErrDuplicateCharge", err)
	}
	got, _ := st.GetClient(ctx, c.ID)
	if got.WalletKopecks != 1000 {
		t.Fatalf("wallet = %d, want 1000 (unchanged)", got.WalletKopecks)
	}

	// A different charge id credits again.
	bal, err = st.CreditWallet(ctx, c.ID, 500, BillingTx{Kind: "topup", ChargeID: "chg2"})
	if err != nil {
		t.Fatal(err)
	}
	if bal != 1500 {
		t.Fatalf("balance = %d, want 1500", bal)
	}
	txs, err := st.ListBillingTx(ctx, c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 {
		t.Fatalf("ledger has %d rows, want 2", len(txs))
	}
}

func TestPurchaseSubscription(t *testing.T) {
	st, ctx := newStore(t)
	c, err := st.CreateClient(ctx, ClientParams{Name: "b", UUID: "u-b", Password: "p", SubscriptionToken: "tok-b", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	// Not enough balance → no change.
	if _, err := st.PurchaseSubscription(ctx, c.ID, 20000, now, 30, BillingTx{Kind: "purchase"}); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
	got, _ := st.GetClient(ctx, c.ID)
	if got.ActiveUntil != "" || got.BillingManaged {
		t.Fatal("failed purchase must not mutate the client")
	}

	if _, err := st.CreditWallet(ctx, c.ID, 25000, BillingTx{Kind: "topup", ChargeID: "x"}); err != nil {
		t.Fatal(err)
	}
	bal, err := st.PurchaseSubscription(ctx, c.ID, 20000, now, 30,
		BillingTx{Kind: "purchase", Method: "wallet", AmountKopecks: -20000, Tariff: "month"})
	if err != nil {
		t.Fatal(err)
	}
	if bal != 5000 {
		t.Fatalf("balance = %d, want 5000", bal)
	}
	wantUntil := now.Add(30 * 24 * time.Hour).Format(time.RFC3339)
	got, _ = st.GetClient(ctx, c.ID)
	if got.ActiveUntil != wantUntil {
		t.Fatalf("active_until = %q, want %q", got.ActiveUntil, wantUntil)
	}
	if !got.BillingManaged {
		t.Fatal("purchase must mark the client billing-managed")
	}
	if !got.Enabled {
		t.Fatal("purchase must (re)enable the client")
	}

	// A second purchase stacks the duration from the current expiry.
	if _, err := st.CreditWallet(ctx, c.ID, 20000, BillingTx{Kind: "topup", ChargeID: "y"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PurchaseSubscription(ctx, c.ID, 20000, now, 30, BillingTx{Kind: "purchase", AmountKopecks: -20000}); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetClient(ctx, c.ID)
	if want := now.Add(60 * 24 * time.Hour).Format(time.RFC3339); got.ActiveUntil != want {
		t.Fatalf("stacked active_until = %q, want %q", got.ActiveUntil, want)
	}

	managed, err := st.ListManagedClients(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) != 1 || managed[0].ID != c.ID {
		t.Fatalf("ListManagedClients = %+v, want [client %d]", managed, c.ID)
	}
}

func TestGrantSubscription(t *testing.T) {
	st, ctx := newStore(t)
	c, err := st.CreateClient(ctx, ClientParams{Name: "g", UUID: "u-g", Password: "p", SubscriptionToken: "tok-g", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2031, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := st.GrantSubscription(ctx, c.ID, now, 30, BillingTx{Kind: "grant", Method: "manual"}); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetClient(ctx, c.ID)
	if want := now.Add(30 * 24 * time.Hour).Format(time.RFC3339); got.ActiveUntil != want || !got.BillingManaged || !got.Enabled {
		t.Fatalf("grant did not apply: %+v (want until %s)", got, want)
	}
	if got.WalletKopecks != 0 {
		t.Fatalf("grant must not touch the wallet, balance = %d", got.WalletKopecks)
	}
}

func TestSuspendExpiredManaged(t *testing.T) {
	st, ctx := newStore(t)
	c, err := st.CreateClient(ctx, ClientParams{Name: "s", UUID: "u-s", Password: "p", SubscriptionToken: "tok-s", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// A plain client (not billing-managed) must never be suspended.
	plain, err := st.CreateClient(ctx, ClientParams{Name: "plain", UUID: "u-pl", Password: "p", SubscriptionToken: "tok-pl", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := st.GrantSubscription(ctx, c.ID, now, 7, BillingTx{Kind: "grant"}); err != nil {
		t.Fatal(err)
	}

	// Still active → nothing suspended.
	n, err := st.SuspendExpiredManaged(ctx, now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("suspended %d while active, want 0", n)
	}

	// Past expiry → the managed client is suspended, the plain one is untouched.
	n, err = st.SuspendExpiredManaged(ctx, now.Add(30*24*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("suspended %d, want 1", n)
	}
	if got, _ := st.GetClient(ctx, c.ID); got.Enabled {
		t.Fatal("expired managed client must be disabled")
	}
	if got, _ := st.GetClient(ctx, plain.ID); !got.Enabled {
		t.Fatal("plain client must stay enabled")
	}
}
