package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/adp/panel/internal/brand"
	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/store"
)

// Settings keys (KV). The bot token and passphrase are stored encrypted at rest
// with the panel master key, like every other secret.
const (
	keyTgEnabled  = "backup_tg_enabled"
	keyTgTokenEnc = "backup_tg_token_enc"
	keyTgChat     = "backup_tg_chat"
	keyTgPassEnc  = "backup_tg_passphrase_enc"
	keyTgInterval = "backup_tg_interval_hours"
	keyTgLastAt   = "backup_tg_last_at"
	keyTgLastErr  = "backup_tg_last_error"
	keyTgLastOK   = "backup_tg_last_ok"
	keyTgOffset   = "backup_tg_updates_offset"

	minIntervalHours     = 1
	defaultIntervalHours = 24
	schedulerTick        = 10 * time.Minute
)

// TelegramStatus is the safe, secret-free view returned to the UI.
type TelegramStatus struct {
	Enabled       bool   `json:"enabled"`
	HasToken      bool   `json:"has_token"`
	ChatID        string `json:"chat_id"`
	HasPassphrase bool   `json:"has_passphrase"`
	IntervalHours int    `json:"interval_hours"`
	LastAt        string `json:"last_at"`
	LastError     string `json:"last_error"`
	LastOK        bool   `json:"last_ok"`
}

// TelegramInput updates the config. An empty Token/Passphrase means "keep the
// stored one" — the UI never receives the secrets, so it can't echo them back.
type TelegramInput struct {
	Enabled       bool   `json:"enabled"`
	Token         string `json:"token"`
	ChatID        string `json:"chat_id"`
	Passphrase    string `json:"passphrase"`
	IntervalHours int    `json:"interval_hours"`
}

// Telegram schedules and sends encrypted backups to a Telegram chat, and (via
// the link poller) delivers per-client configs to clients who link their chat.
type Telegram struct {
	svc     *Service
	store   *store.Store
	cipher  *crypto.Cipher
	logger  *slog.Logger
	client  *http.Client
	apiBase string
	now     func() time.Time

	mu          sync.Mutex
	botUsername string // cached getMe username; cleared when the token changes
}

// NewTelegram wires the Telegram backup delivery.
func NewTelegram(svc *Service, st *store.Store, cipher *crypto.Cipher, logger *slog.Logger) *Telegram {
	return &Telegram{
		svc:     svc,
		store:   st,
		cipher:  cipher,
		logger:  logger,
		client:  &http.Client{Timeout: 60 * time.Second},
		apiBase: "https://api.telegram.org",
		now:     time.Now,
	}
}

// Status returns the current config without exposing the token or passphrase.
func (t *Telegram) Status(ctx context.Context) (TelegramStatus, error) {
	get := func(k string) (string, error) { return t.store.GetSetting(ctx, k) }

	enabled, err := get(keyTgEnabled)
	if err != nil {
		return TelegramStatus{}, err
	}
	tokenEnc, err := get(keyTgTokenEnc)
	if err != nil {
		return TelegramStatus{}, err
	}
	chat, err := get(keyTgChat)
	if err != nil {
		return TelegramStatus{}, err
	}
	passEnc, err := get(keyTgPassEnc)
	if err != nil {
		return TelegramStatus{}, err
	}
	interval, err := get(keyTgInterval)
	if err != nil {
		return TelegramStatus{}, err
	}
	lastAt, err := get(keyTgLastAt)
	if err != nil {
		return TelegramStatus{}, err
	}
	lastErr, err := get(keyTgLastErr)
	if err != nil {
		return TelegramStatus{}, err
	}
	lastOK, err := get(keyTgLastOK)
	if err != nil {
		return TelegramStatus{}, err
	}

	return TelegramStatus{
		Enabled:       enabled == "1",
		HasToken:      tokenEnc != "",
		ChatID:        chat,
		HasPassphrase: passEnc != "",
		IntervalHours: parseInterval(interval),
		LastAt:        lastAt,
		LastError:     lastErr,
		LastOK:        lastOK == "1",
	}, nil
}

// SetConfig persists the config, encrypting the token and passphrase. Blank
// secret fields are left untouched so the UI needn't resend them.
func (t *Telegram) SetConfig(ctx context.Context, in TelegramInput) error {
	set := func(k, v string) error { return t.store.SetSetting(ctx, k, v) }

	if in.Enabled {
		if err := set(keyTgEnabled, "1"); err != nil {
			return err
		}
	} else if err := set(keyTgEnabled, ""); err != nil {
		return err
	}
	if err := set(keyTgChat, in.ChatID); err != nil {
		return err
	}
	interval := in.IntervalHours
	if interval < minIntervalHours {
		interval = defaultIntervalHours
	}
	if err := set(keyTgInterval, strconv.Itoa(interval)); err != nil {
		return err
	}
	if in.Token != "" {
		enc, err := t.cipher.Encrypt(in.Token)
		if err != nil {
			return err
		}
		if err := set(keyTgTokenEnc, enc); err != nil {
			return err
		}
		// A new token may be a different bot — drop the cached username.
		t.mu.Lock()
		t.botUsername = ""
		t.mu.Unlock()
	}
	if in.Passphrase != "" {
		enc, err := t.cipher.Encrypt(in.Passphrase)
		if err != nil {
			return err
		}
		if err := set(keyTgPassEnc, enc); err != nil {
			return err
		}
	}
	return nil
}

// RunNow builds a backup and sends it to Telegram immediately, recording the
// outcome. Used by the "Backup now" button and the scheduler.
func (t *Telegram) RunNow(ctx context.Context) error {
	token, chat, passphrase, err := t.credentials(ctx)
	if err != nil {
		return err
	}

	data, filename, err := t.svc.Export(ctx, passphrase)
	if err != nil {
		t.recordResult(ctx, err)
		return err
	}
	caption := fmt.Sprintf("ADP panel backup — %s", t.now().UTC().Format("2006-01-02 15:04 UTC"))
	if err := sendDocument(ctx, t.client, t.apiBase, token, chat, filename, data, caption); err != nil {
		t.recordResult(ctx, err)
		return err
	}
	t.recordResult(ctx, nil)
	return nil
}

// RunScheduler runs until ctx is cancelled, sending a backup whenever the
// configured interval has elapsed since the last successful send.
func (t *Telegram) RunScheduler(ctx context.Context) {
	ticker := time.NewTicker(schedulerTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.due(ctx) {
				if err := t.RunNow(ctx); err != nil {
					t.logger.Warn("scheduled telegram backup failed", "err", err)
				} else {
					t.logger.Info("scheduled telegram backup sent")
				}
			}
		}
	}
}

// due reports whether an automatic backup should run now.
func (t *Telegram) due(ctx context.Context) bool {
	st, err := t.Status(ctx)
	if err != nil || !st.Enabled || !st.HasToken || !st.HasPassphrase || st.ChatID == "" {
		return false
	}
	if st.LastAt == "" {
		return true // never sent — send the first one soon after enabling
	}
	last, err := time.Parse(time.RFC3339, st.LastAt)
	if err != nil {
		return true
	}
	return t.now().Sub(last) >= time.Duration(st.IntervalHours)*time.Hour
}

func (t *Telegram) credentials(ctx context.Context) (token, chat, passphrase string, err error) {
	tokenEnc, err := t.store.GetSetting(ctx, keyTgTokenEnc)
	if err != nil {
		return "", "", "", err
	}
	passEnc, err := t.store.GetSetting(ctx, keyTgPassEnc)
	if err != nil {
		return "", "", "", err
	}
	chat, err = t.store.GetSetting(ctx, keyTgChat)
	if err != nil {
		return "", "", "", err
	}
	if tokenEnc == "" || passEnc == "" || chat == "" {
		return "", "", "", fmt.Errorf("telegram: set bot token, chat id and passphrase first")
	}
	if token, err = t.cipher.Decrypt(tokenEnc); err != nil {
		return "", "", "", err
	}
	if passphrase, err = t.cipher.Decrypt(passEnc); err != nil {
		return "", "", "", err
	}
	return token, chat, passphrase, nil
}

// recordResult stamps the last run's time and status.
func (t *Telegram) recordResult(ctx context.Context, runErr error) {
	_ = t.store.SetSetting(ctx, keyTgLastAt, t.now().UTC().Format(time.RFC3339))
	if runErr != nil {
		_ = t.store.SetSetting(ctx, keyTgLastOK, "")
		_ = t.store.SetSetting(ctx, keyTgLastErr, runErr.Error())
		return
	}
	_ = t.store.SetSetting(ctx, keyTgLastOK, "1")
	_ = t.store.SetSetting(ctx, keyTgLastErr, "")
}

func parseInterval(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < minIntervalHours {
		return defaultIntervalHours
	}
	return n
}

// sendDocument uploads a file to a Telegram chat via the Bot API.
func sendDocument(ctx context.Context, client *http.Client, apiBase, token, chatID, filename string, data []byte, caption string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("chat_id", chatID); err != nil {
		return err
	}
	if caption != "" {
		if err := mw.WriteField("caption", caption); err != nil {
			return err
		}
	}
	fw, err := mw.CreateFormFile("document", filename)
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/sendDocument", apiBase, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(body, &out)
	if !out.OK {
		if out.Description != "" {
			return fmt.Errorf("telegram: %s", out.Description)
		}
		return fmt.Errorf("telegram: request failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// CanSend reports whether the bot token and chat id are configured (a passphrase
// is only needed for backups, not for arbitrary messages).
func (t *Telegram) CanSend(ctx context.Context) bool {
	tokenEnc, _ := t.store.GetSetting(ctx, keyTgTokenEnc)
	chat, _ := t.store.GetSetting(ctx, keyTgChat)
	return tokenEnc != "" && chat != ""
}

// botChat returns the decrypted bot token and chat id.
func (t *Telegram) botChat(ctx context.Context) (token, chat string, err error) {
	tokenEnc, err := t.store.GetSetting(ctx, keyTgTokenEnc)
	if err != nil {
		return "", "", err
	}
	chat, err = t.store.GetSetting(ctx, keyTgChat)
	if err != nil {
		return "", "", err
	}
	if tokenEnc == "" || chat == "" {
		return "", "", fmt.Errorf("telegram: set the bot token and chat id first")
	}
	token, err = t.cipher.Decrypt(tokenEnc)
	return token, chat, err
}

// tokenOnly returns just the decrypted bot token (chat/passphrase not required).
func (t *Telegram) tokenOnly(ctx context.Context) (string, error) {
	tokenEnc, err := t.store.GetSetting(ctx, keyTgTokenEnc)
	if err != nil {
		return "", err
	}
	if tokenEnc == "" {
		return "", fmt.Errorf("telegram: bot token is not set")
	}
	return t.cipher.Decrypt(tokenEnc)
}

// SendMessage sends an HTML-formatted text message to the configured backup chat.
// Wrapping text in <code>…</code> makes Telegram copy it to the clipboard on tap.
func (t *Telegram) SendMessage(ctx context.Context, htmlText string) error {
	_, chat, err := t.botChat(ctx)
	if err != nil {
		return err
	}
	return t.SendMessageTo(ctx, chat, htmlText)
}

// SendMessageTo sends an HTML text message to an explicit chat id.
func (t *Telegram) SendMessageTo(ctx context.Context, chatID, htmlText string) error {
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"chat_id": chatID, "text": htmlText, "parse_mode": "HTML", "disable_web_page_preview": true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendMessage", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return tgDo(t.client, req)
}

// configButtonLabel is the persistent reply-keyboard button a linked client taps
// to re-request their config; a tap arrives as a message with this exact text.
const configButtonLabel = "🔄 Получить конфиг"

// clientKeyboardJSON is the reply keyboard shown to linked clients so they can
// fetch their config again with one tap.
func clientKeyboardJSON() string {
	b, _ := json.Marshal(map[string]any{
		"keyboard":        [][]map[string]string{{{"text": configButtonLabel}}},
		"resize_keyboard": true,
		"is_persistent":   true,
	})
	return string(b)
}

// SendPhoto sends a photo (e.g. a QR PNG) with an HTML caption to the backup chat.
func (t *Telegram) SendPhoto(ctx context.Context, filename string, photo []byte, htmlCaption string) error {
	_, chat, err := t.botChat(ctx)
	if err != nil {
		return err
	}
	return t.SendPhotoTo(ctx, chat, filename, photo, htmlCaption)
}

// SendPhotoTo sends a photo with an HTML caption to an explicit chat id.
func (t *Telegram) SendPhotoTo(ctx context.Context, chatID, filename string, photo []byte, htmlCaption string) error {
	return t.sendPhoto(ctx, chatID, filename, photo, htmlCaption, "")
}

// sendPhoto uploads a photo to a chat, optionally attaching a reply markup.
func (t *Telegram) sendPhoto(ctx context.Context, chatID, filename string, photo []byte, htmlCaption, replyMarkup string) error {
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", chatID)
	if htmlCaption != "" {
		_ = mw.WriteField("caption", htmlCaption)
		_ = mw.WriteField("parse_mode", "HTML")
	}
	if replyMarkup != "" {
		_ = mw.WriteField("reply_markup", replyMarkup)
	}
	fw, err := mw.CreateFormFile("photo", filename)
	if err != nil {
		return err
	}
	if _, err := fw.Write(photo); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendPhoto", t.apiBase, token), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return tgDo(t.client, req)
}

// SendClientConfig delivers a client's subscription link + QR to a specific chat,
// with the "get config again" reply keyboard attached. The link is wrapped in
// <code> so tapping it copies the link in Telegram.
func (t *Telegram) SendClientConfig(ctx context.Context, chatID, name, subURL string, qr []byte) error {
	caption := fmt.Sprintf(
		"<b>%s</b>\nВаша VPN-подписка (Your VPN subscription):\n<code>%s</code>\n\n%s",
		html.EscapeString(name), html.EscapeString(subURL), brand.TextFooter())
	return t.sendPhoto(ctx, chatID, "vpn-config.png", qr, caption, clientKeyboardJSON())
}

// BotUsername returns the bot's @username (via getMe), cached after the first
// successful lookup and invalidated when the token changes.
func (t *Telegram) BotUsername(ctx context.Context) (string, error) {
	t.mu.Lock()
	cached := t.botUsername
	t.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	token, err := t.tokenOnly(ctx)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/bot%s/getMe", t.apiBase, token), nil)
	if err != nil {
		return "", err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	_ = json.Unmarshal(body, &out)
	if !out.OK || out.Result.Username == "" {
		if out.Description != "" {
			return "", fmt.Errorf("telegram getMe: %s", out.Description)
		}
		return "", fmt.Errorf("telegram: could not read bot username")
	}
	t.mu.Lock()
	t.botUsername = out.Result.Username
	t.mu.Unlock()
	return out.Result.Username, nil
}

// tgDo runs a Bot API request and surfaces the {ok,description} result.
func tgDo(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(body, &out)
	if !out.OK {
		if out.Description != "" {
			return fmt.Errorf("telegram: %s", out.Description)
		}
		return fmt.Errorf("telegram: request failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}
