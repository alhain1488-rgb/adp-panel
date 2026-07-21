// Package billing turns the panel into an optional paid service: clients hold a
// prepaid wallet (in kopecks; 100 = 1 RUB), top it up (today via Telegram Stars,
// with card/SBP/crypto planned), and spend it on time-based subscriptions that
// set an expiry. A background reconciler auto-suspends managed clients whose time
// has run out. When billing is disabled (the default), none of this engages and
// the panel stays personal-use — no client is ever auto-suspended.
package billing

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/adp/panel/internal/store"
)

// Settings KV keys.
const (
	keyEnabled      = "billing_enabled"
	keyTariffWeek   = "billing_tariff_week_kopecks"
	keyTariffMonth  = "billing_tariff_month_kopecks"
	keyTariffYear   = "billing_tariff_year_kopecks"
	keyStarRate     = "billing_star_rate_kopecks"
	keySupport      = "billing_support_contact"
	reconcileEvery  = 5 * time.Minute
	reconcileWarmup = 20 * time.Second
)

// Defaults (kopecks). Week 70₽ / month 200₽ / year 2000₽; ~1.30₽ per Star.
const (
	defWeekKopecks  = 7000
	defMonthKopecks = 20000
	defYearKopecks  = 200000
	defStarRate     = 130
	defSupport      = "@solepytt"
)

// Tariff durations, in days.
const (
	daysWeek  = 7
	daysMonth = 30
	daysYear  = 365
)

// Settings is the operator-tunable billing configuration.
type Settings struct {
	Enabled            bool   `json:"enabled"`
	TariffWeekKopecks  int64  `json:"tariff_week_kopecks"`
	TariffMonthKopecks int64  `json:"tariff_month_kopecks"`
	TariffYearKopecks  int64  `json:"tariff_year_kopecks"`
	StarRateKopecks    int64  `json:"star_rate_kopecks"`
	SupportContact     string `json:"support_contact"`
}

// Tariff is one purchasable subscription plan.
type Tariff struct {
	Key          string `json:"key"` // week | month | year
	PriceKopecks int64  `json:"price_kopecks"`
	Days         int    `json:"days"`
}

// Transaction is one ledger entry as shown to the UI/bot.
type Transaction struct {
	ID            int64  `json:"id"`
	Kind          string `json:"kind"`
	Method        string `json:"method"`
	AmountKopecks int64  `json:"amount_kopecks"`
	Stars         int64  `json:"stars,omitempty"`
	Tariff        string `json:"tariff,omitempty"`
	Detail        string `json:"detail,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// ClientBilling is a client's wallet + subscription snapshot.
type ClientBilling struct {
	ClientID       int64         `json:"client_id"`
	BalanceKopecks int64         `json:"balance_kopecks"`
	ActiveUntil    string        `json:"active_until"`
	Active         bool          `json:"active"`
	Managed        bool          `json:"managed"`
	Transactions   []Transaction `json:"transactions"`
}

// Resync re-pushes engine config to all nodes after a client's enabled state
// changes. Satisfied by *sync.Service; kept as an interface to avoid the import.
type Resync interface{ AsyncAll() }

// Service is the billing engine.
type Service struct {
	store  *store.Store
	resync Resync
	logger *slog.Logger
	now    func() time.Time
}

// NewService builds a billing Service. resync may be nil (no auto re-sync).
func NewService(st *store.Store, resync Resync, logger *slog.Logger) *Service {
	return &Service{store: st, resync: resync, logger: logger, now: time.Now}
}

// GetSettings reads the billing configuration, applying defaults for unset keys.
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	get := func(k string) (string, error) { return s.store.GetSetting(ctx, k) }
	enabled, err := get(keyEnabled)
	if err != nil {
		return Settings{}, err
	}
	week, err := s.getKopecks(ctx, keyTariffWeek, defWeekKopecks)
	if err != nil {
		return Settings{}, err
	}
	month, err := s.getKopecks(ctx, keyTariffMonth, defMonthKopecks)
	if err != nil {
		return Settings{}, err
	}
	year, err := s.getKopecks(ctx, keyTariffYear, defYearKopecks)
	if err != nil {
		return Settings{}, err
	}
	rate, err := s.getKopecks(ctx, keyStarRate, defStarRate)
	if err != nil {
		return Settings{}, err
	}
	support, err := get(keySupport)
	if err != nil {
		return Settings{}, err
	}
	if support == "" {
		support = defSupport
	}
	return Settings{
		Enabled:            enabled == "1",
		TariffWeekKopecks:  week,
		TariffMonthKopecks: month,
		TariffYearKopecks:  year,
		StarRateKopecks:    rate,
		SupportContact:     support,
	}, nil
}

func (s *Service) getKopecks(ctx context.Context, key string, def int64) (int64, error) {
	v, err := s.store.GetSetting(ctx, key)
	if err != nil {
		return 0, err
	}
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return def, nil
	}
	return n, nil
}

// SetSettings persists the billing configuration.
func (s *Service) SetSettings(ctx context.Context, in Settings) error {
	set := func(k, v string) error { return s.store.SetSetting(ctx, k, v) }
	enabled := ""
	if in.Enabled {
		enabled = "1"
	}
	pairs := [][2]string{
		{keyEnabled, enabled},
		{keyTariffWeek, strconv.FormatInt(clampNonNeg(in.TariffWeekKopecks, defWeekKopecks), 10)},
		{keyTariffMonth, strconv.FormatInt(clampNonNeg(in.TariffMonthKopecks, defMonthKopecks), 10)},
		{keyTariffYear, strconv.FormatInt(clampNonNeg(in.TariffYearKopecks, defYearKopecks), 10)},
		{keyStarRate, strconv.FormatInt(clampPos(in.StarRateKopecks, defStarRate), 10)},
		{keySupport, in.SupportContact},
	}
	for _, p := range pairs {
		if err := set(p[0], p[1]); err != nil {
			return err
		}
	}
	return nil
}

func clampNonNeg(v, def int64) int64 {
	if v < 0 {
		return def
	}
	return v
}

func clampPos(v, def int64) int64 {
	if v <= 0 {
		return def
	}
	return v
}

// Enabled reports whether billing is turned on.
func (s *Service) Enabled(ctx context.Context) bool {
	v, _ := s.store.GetSetting(ctx, keyEnabled)
	return v == "1"
}

// Tariffs returns the three plans in ascending duration order.
func (s *Service) Tariffs(ctx context.Context) ([]Tariff, error) {
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return []Tariff{
		{Key: "week", PriceKopecks: cfg.TariffWeekKopecks, Days: daysWeek},
		{Key: "month", PriceKopecks: cfg.TariffMonthKopecks, Days: daysMonth},
		{Key: "year", PriceKopecks: cfg.TariffYearKopecks, Days: daysYear},
	}, nil
}

// TariffByKey returns the tariff for key ("week"|"month"|"year").
func (s *Service) TariffByKey(ctx context.Context, key string) (Tariff, bool) {
	tariffs, err := s.Tariffs(ctx)
	if err != nil {
		return Tariff{}, false
	}
	for _, t := range tariffs {
		if t.Key == key {
			return t, true
		}
	}
	return Tariff{}, false
}

// ClientBilling returns a client's wallet + subscription snapshot with recent
// ledger entries (capped at txLimit).
func (s *Service) ClientBilling(ctx context.Context, clientID int64, txLimit int) (ClientBilling, error) {
	c, err := s.store.GetClient(ctx, clientID)
	if err != nil {
		return ClientBilling{}, err
	}
	rows, err := s.store.ListBillingTx(ctx, clientID, txLimit)
	if err != nil {
		return ClientBilling{}, err
	}
	txs := make([]Transaction, 0, len(rows))
	for _, r := range rows {
		txs = append(txs, Transaction{
			ID: r.ID, Kind: r.Kind, Method: r.Method, AmountKopecks: r.AmountKopecks,
			Stars: r.Stars, Tariff: r.Tariff, Detail: r.Detail, CreatedAt: r.CreatedAt,
		})
	}
	return ClientBilling{
		ClientID:       c.ID,
		BalanceKopecks: c.WalletKopecks,
		ActiveUntil:    c.ActiveUntil,
		Active:         s.isActive(c.ActiveUntil),
		Managed:        c.BillingManaged,
		Transactions:   txs,
	}, nil
}

// isActive reports whether an RFC3339 expiry is in the future.
func (s *Service) isActive(activeUntil string) bool {
	if activeUntil == "" {
		return false
	}
	until, err := time.Parse(time.RFC3339, activeUntil)
	if err != nil {
		return false
	}
	return until.After(s.now())
}

// CreditStars credits a Stars top-up to a client's wallet, converting stars to
// kopecks at the configured rate. Crediting is idempotent by chargeID: a repeated
// charge returns store.ErrDuplicateCharge and changes nothing. Returns the amount
// credited (kopecks) and the resulting balance.
func (s *Service) CreditStars(ctx context.Context, clientID, stars int64, chargeID string) (creditedKopecks, newBalance int64, err error) {
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return 0, 0, err
	}
	creditedKopecks = stars * cfg.StarRateKopecks
	newBalance, err = s.store.CreditWallet(ctx, clientID, creditedKopecks, store.BillingTx{
		Kind: "topup", Method: "stars", AmountKopecks: creditedKopecks, Stars: stars, ChargeID: chargeID,
	})
	if err != nil {
		return creditedKopecks, 0, err
	}
	return creditedKopecks, newBalance, nil
}

// ManualAdjust applies an operator credit (positive) or debit (negative) to a
// client's wallet and records it.
func (s *Service) ManualAdjust(ctx context.Context, clientID, deltaKopecks int64, detail string) (int64, error) {
	return s.store.CreditWallet(ctx, clientID, deltaKopecks, store.BillingTx{
		Kind: "adjust", Method: "manual", AmountKopecks: deltaKopecks, Detail: detail,
	})
}

// Purchase spends wallet balance on a tariff, extending the subscription from the
// later of now / current expiry (computed atomically in the store). Returns
// store.ErrInsufficientFunds on low balance.
func (s *Service) Purchase(ctx context.Context, clientID int64, tariffKey string) (ClientBilling, error) {
	tariff, ok := s.TariffByKey(ctx, tariffKey)
	if !ok {
		return ClientBilling{}, store.ErrNotFound
	}
	if _, err := s.store.PurchaseSubscription(ctx, clientID, tariff.PriceKopecks, s.now(), tariff.Days, store.BillingTx{
		Kind: "purchase", Method: "wallet", AmountKopecks: -tariff.PriceKopecks, Tariff: tariff.Key,
		Detail: tariffKey,
	}); err != nil {
		return ClientBilling{}, err
	}
	s.triggerResync()
	return s.ClientBilling(ctx, clientID, 20)
}

// Grant extends a client's subscription by days without charging the wallet
// (operator comp). Days must be positive.
func (s *Service) Grant(ctx context.Context, clientID int64, days int, detail string) (ClientBilling, error) {
	if days <= 0 {
		return ClientBilling{}, store.ErrNotFound
	}
	if err := s.store.GrantSubscription(ctx, clientID, s.now(), days, store.BillingTx{
		Kind: "grant", Method: "manual", AmountKopecks: 0, Detail: detail,
	}); err != nil {
		return ClientBilling{}, err
	}
	s.triggerResync()
	return s.ClientBilling(ctx, clientID, 20)
}

func (s *Service) triggerResync() {
	if s.resync != nil {
		s.resync.AsyncAll()
	}
}

// Reconcile auto-suspends managed clients whose subscription has expired. It only
// ever disables (enabling happens on purchase), so it never fights a manual
// operator toggle. The suspension is a single atomic statement that re-checks the
// expiry at write time, so it can't clobber a purchase that just extended a client
// (that row no longer matches the WHERE). No-op when billing is disabled.
func (s *Service) Reconcile(ctx context.Context) {
	if !s.Enabled(ctx) {
		return
	}
	now := s.now().UTC().Format(time.RFC3339)
	n, err := s.store.SuspendExpiredManaged(ctx, now)
	if err != nil {
		s.logger.Warn("billing: suspend expired clients failed", "err", err)
		return
	}
	if n > 0 {
		s.logger.Info("billing: suspended expired subscriptions", "count", n)
		s.triggerResync()
	}
}

// RunReconciler runs Reconcile on a fixed cadence until ctx is cancelled.
func (s *Service) RunReconciler(ctx context.Context) {
	if !sleepCtx(ctx, reconcileWarmup) {
		s.Reconcile(ctx)
	} else {
		return
	}
	ticker := time.NewTicker(reconcileEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Reconcile(ctx)
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}
