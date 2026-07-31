// Package tribute accepts card/SBP payments made through Tribute
// (tribute.tg) for panel subscriptions.
//
// Unlike Telegram Stars, the money never passes through the client's wallet: a
// Tribute product has a fixed price and buying it grants subscription time
// directly. The panel therefore stores, per Tribute product, how many days it is
// worth — which also lets Tribute sell periods the panel's own tariffs don't
// have (a six-month plan, say) without touching the tariff model.
package tribute

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/adp/panel/internal/store"
)

// Settings KV keys.
const (
	keyEnabled  = "billing_tribute_enabled"
	keyAPIKey   = "billing_tribute_api_key_enc"
	keyProducts = "billing_tribute_products"
)

// SignatureHeader carries the HMAC-SHA256 of the raw request body, keyed with the
// account's API key.
const SignatureHeader = "trbt-signature"

// Errors surfaced to the HTTP layer.
var (
	// ErrDisabled means Tribute payments are switched off.
	ErrDisabled = errors.New("tribute: payments are disabled")
	// ErrNotConfigured means no API key is stored, so signatures cannot be checked.
	ErrNotConfigured = errors.New("tribute: no API key configured")
	// ErrBadSignature means the body did not match the trbt-signature header.
	ErrBadSignature = errors.New("tribute: signature mismatch")
	// ErrUnknownProduct means the purchased product has no day mapping.
	ErrUnknownProduct = errors.New("tribute: product is not mapped to a subscription")
	// ErrUnknownClient means no client is linked to the paying Telegram account.
	ErrUnknownClient = errors.New("tribute: no client linked to that Telegram user")
	// ErrIgnored means the event is well-formed but not one we act on.
	ErrIgnored = errors.New("tribute: event ignored")
)

// Product maps one Tribute product to the subscription it grants.
type Product struct {
	ProductID int64  `json:"product_id"`
	Days      int    `json:"days"`
	Title     string `json:"title,omitempty"`
	// Link is the Tribute payment link shown to clients in the bot.
	Link string `json:"link,omitempty"`
}

// Settings is the operator-facing Tribute configuration. The API key is never
// returned to the UI — only whether one is set.
type Settings struct {
	Enabled   bool      `json:"enabled"`
	HasAPIKey bool      `json:"has_api_key"`
	Products  []Product `json:"products"`
}

// Cipher decrypts the stored API key. Satisfied by *crypto.Cipher.
type Cipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(encoded string) (string, error)
}

// Granter applies a paid subscription. Satisfied by the billing service; kept as
// an interface so this package does not depend on it.
type Granter interface {
	GrantPaid(ctx context.Context, clientID int64, days int, method, chargeID, detail string, amountKopecks int64) error
}

// Service verifies Tribute webhooks and turns them into subscriptions.
type Service struct {
	store   *store.Store
	cipher  Cipher
	granter Granter
	logger  *slog.Logger
}

func NewService(st *store.Store, cipher Cipher, granter Granter, logger *slog.Logger) *Service {
	return &Service{store: st, cipher: cipher, granter: granter, logger: logger}
}

// GetSettings reads the Tribute configuration.
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	enabled, err := s.store.GetSetting(ctx, keyEnabled)
	if err != nil {
		return Settings{}, err
	}
	enc, err := s.store.GetSetting(ctx, keyAPIKey)
	if err != nil {
		return Settings{}, err
	}
	raw, err := s.store.GetSetting(ctx, keyProducts)
	if err != nil {
		return Settings{}, err
	}
	out := Settings{Enabled: enabled == "1", HasAPIKey: enc != "", Products: []Product{}}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &out.Products); err != nil {
			// A corrupt mapping must not take the whole settings page down.
			s.logger.Warn("tribute: bad product mapping in settings", "err", err)
			out.Products = []Product{}
		}
	}
	return out, nil
}

// SetSettings persists the configuration. An empty apiKey leaves the stored key
// untouched, so the UI can save without ever holding the secret.
func (s *Service) SetSettings(ctx context.Context, in Settings, apiKey string) error {
	enabled := ""
	if in.Enabled {
		enabled = "1"
	}
	if err := s.store.SetSetting(ctx, keyEnabled, enabled); err != nil {
		return err
	}
	products := make([]Product, 0, len(in.Products))
	for _, p := range in.Products {
		if p.ProductID <= 0 || p.Days <= 0 {
			continue // a mapping without both halves grants nothing; drop it
		}
		products = append(products, p)
	}
	blob, err := json.Marshal(products)
	if err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, keyProducts, string(blob)); err != nil {
		return err
	}
	if apiKey != "" {
		enc, err := s.cipher.Encrypt(apiKey)
		if err != nil {
			return err
		}
		if err := s.store.SetSetting(ctx, keyAPIKey, enc); err != nil {
			return err
		}
	}
	return nil
}

// apiKey returns the decrypted API key, which doubles as the webhook signing key.
func (s *Service) apiKey(ctx context.Context) (string, error) {
	enc, err := s.store.GetSetting(ctx, keyAPIKey)
	if err != nil {
		return "", err
	}
	if enc == "" {
		return "", ErrNotConfigured
	}
	return s.cipher.Decrypt(enc)
}

// event is the envelope Tribute posts to the webhook URL.
type event struct {
	Name      string          `json:"name"`
	CreatedAt string          `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
}

// digitalProductPayload is the part of a purchase we act on.
type digitalProductPayload struct {
	ProductID      int64  `json:"product_id"`
	ProductName    string `json:"product_name"`
	Amount         int64  `json:"amount"` // smallest currency unit — kopecks for RUB
	Currency       string `json:"currency"`
	PurchaseID     int64  `json:"purchase_id"`
	TelegramUserID int64  `json:"telegram_user_id"`
}

// HandleWebhook verifies the signature and applies the purchase. It is safe to
// call repeatedly with the same body: crediting is keyed on Tribute's purchase
// id, so retries (Tribute repeats for ~24h) grant the subscription once.
func (s *Service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return ErrDisabled
	}
	key, err := s.apiKey(ctx)
	if err != nil {
		return err
	}
	if !ValidSignature(body, signature, key) {
		return ErrBadSignature
	}

	var ev event
	if err := json.Unmarshal(body, &ev); err != nil {
		return fmt.Errorf("tribute: bad body: %w", err)
	}
	if ev.Name != "new_digital_product" {
		// Refunds and other events are acknowledged but not acted on: reversing a
		// subscription automatically would fight the operator's own toggles.
		s.logger.Info("tribute: event ignored", "name", ev.Name)
		return ErrIgnored
	}

	var p digitalProductPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("tribute: bad payload: %w", err)
	}
	days := 0
	for _, prod := range cfg.Products {
		if prod.ProductID == p.ProductID {
			days = prod.Days
			break
		}
	}
	if days <= 0 {
		s.logger.Warn("tribute: unmapped product paid", "product", p.ProductID, "purchase", p.PurchaseID)
		return ErrUnknownProduct
	}

	// Private chats give chat id == user id, which is how the bot stored it.
	chatID := strconv.FormatInt(p.TelegramUserID, 10)
	c, err := s.store.GetClientByTelegramChatID(ctx, chatID)
	if err != nil {
		s.logger.Warn("tribute: payment from an unknown Telegram user",
			"tg_user", p.TelegramUserID, "purchase", p.PurchaseID)
		return ErrUnknownClient
	}

	detail := p.ProductName
	if detail == "" {
		detail = "Tribute product " + strconv.FormatInt(p.ProductID, 10)
	}
	chargeID := "tribute:" + strconv.FormatInt(p.PurchaseID, 10)
	if err := s.granter.GrantPaid(ctx, c.ID, days, "tribute", chargeID, detail, p.Amount); err != nil {
		if errors.Is(err, store.ErrDuplicateCharge) {
			return nil // already applied — a retry, not a problem
		}
		return err
	}
	s.logger.Info("tribute: subscription granted",
		"client", c.ID, "days", days, "purchase", p.PurchaseID, "amount", p.Amount)
	return nil
}

// ValidSignature reports whether sig is the hex HMAC-SHA256 of body under key.
func ValidSignature(body []byte, sig, key string) bool {
	if sig == "" || key == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	// Constant-time compare; hmac.Equal also guards against length leaks.
	return hmac.Equal([]byte(want), []byte(sig))
}

// PayOption is one purchasable plan, as offered in the bot.
type PayOption struct {
	Title string
	Link  string
}

// PayOptions returns the plans clients can pay for by card/SBP right now: the
// configured products that actually carry a payment link. Returns nothing when
// Tribute is disabled, so the bot silently falls back to its "coming soon" copy.
func (s *Service) PayOptions(ctx context.Context) ([]PayOption, error) {
	cfg, err := s.GetSettings(ctx)
	if err != nil || !cfg.Enabled {
		return nil, err
	}
	out := make([]PayOption, 0, len(cfg.Products))
	for _, p := range cfg.Products {
		if p.Link == "" {
			continue
		}
		title := p.Title
		if title == "" {
			title = fmt.Sprintf("%d дн.", p.Days)
		}
		out = append(out, PayOption{Title: title, Link: p.Link})
	}
	return out, nil
}
