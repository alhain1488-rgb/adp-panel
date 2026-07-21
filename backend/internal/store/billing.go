package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Billing sentinel errors.
var (
	// ErrInsufficientFunds is returned when a purchase exceeds the wallet balance.
	ErrInsufficientFunds = errors.New("store: insufficient funds")
	// ErrDuplicateCharge is returned when a Stars payment (by charge id) has
	// already been credited — the crediting is idempotent, so this is not fatal.
	ErrDuplicateCharge = errors.New("store: duplicate charge id")
)

// BillingTx is one row of the append-only wallet ledger.
type BillingTx struct {
	ID            int64
	ClientID      int64
	Kind          string // topup | purchase | adjust | refund | grant
	Method        string // stars | card | sbp | crypto | manual
	AmountKopecks int64  // signed: +credit, -debit
	Stars         int64  // stars paid, when Method == "stars"
	Tariff        string // week | month | year, for purchases
	Detail        string
	ChargeID      string // telegram_payment_charge_id, for Stars top-ups
	CreatedAt     string
}

// CreditWallet adds deltaKopecks (positive to credit, negative to debit) to a
// client's wallet and records tx, atomically. When tx.ChargeID is set and a row
// with that charge id already exists, it returns ErrDuplicateCharge and makes no
// change — this makes Stars crediting idempotent against duplicate updates.
func (s *Store) CreditWallet(ctx context.Context, clientID, deltaKopecks int64, tx BillingTx) (newBalance int64, err error) {
	dbtx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = dbtx.Rollback() }()

	if tx.ChargeID != "" {
		var n int64
		if err := dbtx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM billing_transactions WHERE charge_id = ?", tx.ChargeID).Scan(&n); err != nil {
			return 0, err
		}
		if n > 0 {
			return 0, ErrDuplicateCharge
		}
	}

	res, err := dbtx.ExecContext(ctx,
		"UPDATE clients SET wallet_kopecks = wallet_kopecks + ?, updated_at = ? WHERE id = ?",
		deltaKopecks, nowRFC3339(), clientID)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}
	if err := dbtx.QueryRowContext(ctx,
		"SELECT wallet_kopecks FROM clients WHERE id = ?", clientID).Scan(&newBalance); err != nil {
		return 0, err
	}
	if err := insertBillingTx(ctx, dbtx, clientID, tx); err != nil {
		return 0, err
	}
	return newBalance, dbtx.Commit()
}

// PurchaseSubscription debits priceKopecks from the wallet, extends the client's
// subscription by days (from the later of now / the current expiry, read inside
// the transaction so concurrent extensions can't lose an update), marks them
// billing-managed, (re)enables them, and records tx — all atomically. Returns
// ErrInsufficientFunds if the balance is too low (and makes no change).
func (s *Store) PurchaseSubscription(ctx context.Context, clientID, priceKopecks int64, now time.Time, days int, tx BillingTx) (newBalance int64, err error) {
	dbtx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = dbtx.Rollback() }()

	var balance int64
	var curUntil string
	err = dbtx.QueryRowContext(ctx, "SELECT wallet_kopecks, active_until FROM clients WHERE id = ?", clientID).
		Scan(&balance, &curUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if balance < priceKopecks {
		return balance, ErrInsufficientFunds
	}

	until := extendUntil(now, curUntil, days)
	if _, err := dbtx.ExecContext(ctx,
		"UPDATE clients SET wallet_kopecks = wallet_kopecks - ?, active_until = ?, billing_managed = 1, enabled = 1, updated_at = ? WHERE id = ?",
		priceKopecks, until, nowRFC3339(), clientID); err != nil {
		return 0, err
	}
	newBalance = balance - priceKopecks
	if err := insertBillingTx(ctx, dbtx, clientID, tx); err != nil {
		return 0, err
	}
	return newBalance, dbtx.Commit()
}

// GrantSubscription extends a client's subscription by days without charging the
// wallet (operator comp), marking them billing-managed and enabled, and records
// tx. The new expiry is computed from the current row inside the transaction.
func (s *Store) GrantSubscription(ctx context.Context, clientID int64, now time.Time, days int, tx BillingTx) error {
	dbtx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = dbtx.Rollback() }()

	var curUntil string
	err = dbtx.QueryRowContext(ctx, "SELECT active_until FROM clients WHERE id = ?", clientID).Scan(&curUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	until := extendUntil(now, curUntil, days)
	if _, err := dbtx.ExecContext(ctx,
		"UPDATE clients SET active_until = ?, billing_managed = 1, enabled = 1, updated_at = ? WHERE id = ?",
		until, nowRFC3339(), clientID); err != nil {
		return err
	}
	if err := insertBillingTx(ctx, dbtx, clientID, tx); err != nil {
		return err
	}
	return dbtx.Commit()
}

// extendUntil returns the new RFC3339 expiry: base is the later of now and the
// current expiry (curUntil, may be ""), plus days.
func extendUntil(now time.Time, curUntil string, days int) string {
	base := now.UTC()
	if curUntil != "" {
		if cur, err := time.Parse(time.RFC3339, curUntil); err == nil && cur.After(base) {
			base = cur
		}
	}
	return base.Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)
}

// SuspendExpiredManaged disables, in one atomic statement, every billing-managed
// client whose subscription expired at or before now. Because the expiry is
// re-checked in the WHERE clause at write time, it cannot clobber a concurrent
// purchase that just extended a client (that row no longer matches). Returns the
// number of clients suspended.
func (s *Store) SuspendExpiredManaged(ctx context.Context, now string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE clients SET enabled = 0, updated_at = ?
		WHERE billing_managed = 1 AND enabled = 1 AND active_until != '' AND active_until <= ?`,
		nowRFC3339(), now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// insertBillingTx appends one ledger row within an open transaction.
func insertBillingTx(ctx context.Context, dbtx *sql.Tx, clientID int64, tx BillingTx) error {
	_, err := dbtx.ExecContext(ctx, `
		INSERT INTO billing_transactions
			(client_id, kind, method, amount_kopecks, stars, tariff, detail, charge_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		clientID, tx.Kind, tx.Method, tx.AmountKopecks, tx.Stars, tx.Tariff, tx.Detail, tx.ChargeID, nowRFC3339())
	return err
}

// ListBillingTx returns a client's ledger, newest first, capped at limit.
func (s *Store) ListBillingTx(ctx context.Context, clientID int64, limit int) ([]BillingTx, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, client_id, kind, method, amount_kopecks, stars, tariff, detail, charge_id, created_at
		FROM billing_transactions WHERE client_id = ? ORDER BY id DESC LIMIT ?`, clientID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []BillingTx{}
	for rows.Next() {
		var t BillingTx
		if err := rows.Scan(&t.ID, &t.ClientID, &t.Kind, &t.Method, &t.AmountKopecks,
			&t.Stars, &t.Tariff, &t.Detail, &t.ChargeID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListManagedClients returns every client the billing engine manages (has bought
// or been granted a subscription). Used by the expiry reconciler.
func (s *Store) ListManagedClients(ctx context.Context) ([]Client, error) {
	rows, err := s.db.QueryContext(ctx, clientSelect+" WHERE billing_managed = 1 ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
