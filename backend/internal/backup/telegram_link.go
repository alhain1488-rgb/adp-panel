package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adp/panel/internal/brand"
	"github.com/adp/panel/internal/store"
)

// withFooter appends the branding plate to a bot message.
func withFooter(msg string) string {
	return msg + "\n\n" + brand.TextFooter()
}

// tgUpdate is the subset of a Telegram update we act on: text messages (deep-link
// "/start <token>", the reply-keyboard buttons and commands), inline-button taps
// (callback_query), and the two payment updates (pre_checkout_query and a message
// carrying successful_payment).
type tgUpdate struct {
	UpdateID         int64            `json:"update_id"`
	Message          *tgMessage       `json:"message"`
	CallbackQuery    *tgCallbackQuery `json:"callback_query"`
	PreCheckoutQuery *tgPreCheckout   `json:"pre_checkout_query"`
}

type tgMessage struct {
	Text string `json:"text"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	SuccessfulPayment *tgSuccessfulPayment `json:"successful_payment"`
}

type tgCallbackQuery struct {
	ID   string `json:"id"`
	From struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	Message *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
	Data string `json:"data"`
}

type tgPreCheckout struct {
	ID   string `json:"id"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Currency       string `json:"currency"`
	TotalAmount    int64  `json:"total_amount"`
	InvoicePayload string `json:"invoice_payload"`
}

type tgSuccessfulPayment struct {
	Currency                string `json:"currency"`
	TotalAmount             int64  `json:"total_amount"`
	InvoicePayload          string `json:"invoice_payload"`
	TelegramPaymentChargeID string `json:"telegram_payment_charge_id"`
}

// RunLinkPoller long-polls getUpdates and, per message, either binds a chat to a
// client ("/start <link_token>") or re-delivers a linked client's config ("/config"
// or the reply-keyboard button). deliver (may be nil) pushes the client's config —
// used both right after a bind and on request. It runs until ctx is cancelled and
// tolerates the bot being unconfigured (waits and retries), so it is safe to start
// unconditionally.
func (t *Telegram) RunLinkPoller(ctx context.Context, deliver func(context.Context, *store.Client)) {
	// Long-poll needs a client timeout comfortably above the poll timeout.
	client := &http.Client{Timeout: 70 * time.Second}
	offset := t.loadOffset(ctx)
	commandsSet := false
	for {
		if ctx.Err() != nil {
			return
		}
		token, err := t.tokenOnly(ctx)
		if err != nil || token == "" {
			if sleepCtx(ctx, 30*time.Second) {
				return
			}
			continue
		}
		if !commandsSet {
			t.setMyCommands(ctx, token)
			commandsSet = true
		}
		updates, next, err := getUpdates(ctx, client, t.apiBase, token, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			t.logger.Debug("telegram getUpdates failed", "err", err)
			if sleepCtx(ctx, 5*time.Second) {
				return
			}
			continue
		}
		// Answer time-critical pre_checkout_query updates first (Telegram cancels
		// the charge if not answered within ~10s), then handle everything else.
		for i := range updates {
			if updates[i].PreCheckoutQuery != nil {
				t.handleUpdate(ctx, updates[i], deliver)
			}
		}
		for i := range updates {
			if updates[i].PreCheckoutQuery == nil {
				t.handleUpdate(ctx, updates[i], deliver)
			}
		}
		// Advance the cursor only AFTER the batch is handled. A crash mid-batch
		// then re-delivers it (all handlers are idempotent — Stars crediting by
		// charge id, linking, config resend), which is safe; advancing first
		// would let a captured payment be lost on restart.
		if next != offset {
			offset = next
			t.saveOffset(ctx, offset)
		}
	}
}

// handleUpdate routes one update to the right handler: inline-button taps
// (callback_query) and the two payment updates go to the billing flow; otherwise
// a text message is handled below.
func (t *Telegram) handleUpdate(ctx context.Context, u tgUpdate, deliver func(context.Context, *store.Client)) {
	switch {
	case u.CallbackQuery != nil:
		t.handleCallback(ctx, u.CallbackQuery, deliver)
	case u.PreCheckoutQuery != nil:
		t.handlePreCheckout(ctx, u.PreCheckoutQuery)
	case u.Message != nil && u.Message.SuccessfulPayment != nil:
		t.handleSuccessfulPayment(ctx, u.Message, deliver)
	case u.Message != nil:
		t.handleTextMessage(ctx, u, deliver)
	}
}

// handleTextMessage processes one text message: "/start <token>" links the
// sender's chat to the matching client; "/config" (or the reply-keyboard button,
// or a bare /start from an already-linked chat) re-delivers that chat's config;
// the billing reply-keyboard buttons/commands drive the wallet flow.
func (t *Telegram) handleTextMessage(ctx context.Context, u tgUpdate, deliver func(context.Context, *store.Client)) {
	text := strings.TrimSpace(u.Message.Text)
	if text == "" {
		return
	}
	chatID := strconv.FormatInt(u.Message.Chat.ID, 10)

	// On-demand config request from a linked client.
	if isConfigRequest(text) {
		c, err := t.store.GetClientByTelegramChatID(ctx, chatID)
		if err != nil {
			_ = t.SendMessageTo(ctx, chatID, withFooter(
				"Вы ещё не привязаны. Откройте персональную ссылку, которую дал вам администратор. "+
					"(You're not linked yet. Open the personal link your administrator gave you.)"))
			return
		}
		if deliver != nil {
			deliver(ctx, c)
		}
		return
	}

	// Billing reply-keyboard buttons and commands (balance, top-up, buy, …).
	if t.billingOn(ctx) && t.handleBillingText(ctx, chatID, u.Message.From.Username, text, deliver) {
		return
	}

	if !strings.HasPrefix(text, "/start") {
		return
	}
	fields := strings.Fields(text)
	if len(fields) < 2 {
		// Bare /start: if the chat is already linked, just resend the config;
		// otherwise explain how to link.
		if c, err := t.store.GetClientByTelegramChatID(ctx, chatID); err == nil {
			// A disabled client is in no engine config, so their "config" would be
			// links that cannot connect — someone who signed up but never paid, or
			// a lapsed subscription. Offer the plans instead of dead links.
			if t.billingOn(ctx) && !c.Enabled {
				t.sendTariffs(ctx, chatID, u.Message.From.Username)
				return
			}
			if deliver != nil {
				deliver(ctx, c)
			}
			return
		}
		// A stranger: offer the plans when self-signup is on, otherwise explain
		// that access starts from the administrator's personal link. No client is
		// created here — that waits until they actually move to pay.
		if t.billingOn(ctx) && t.selfSignupOn(ctx) {
			t.sendWelcome(ctx, chatID)
			return
		}
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Откройте персональную ссылку, которую дал вам администратор, чтобы получать сюда свой VPN-конфиг. "+
				"(Open the personal link your administrator gave you to receive your VPN config here.)"))
		return
	}
	c, err := t.store.LinkClientTelegram(ctx, fields[1], chatID, u.Message.From.Username)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, withFooter(
			"Ссылка недействительна. Попросите у администратора новую. "+
				"(This link is invalid — ask your administrator for a new one.)"))
		return
	}
	_ = t.SendMessageTo(ctx, chatID, withFooter(fmt.Sprintf(
		"✅ Готово, %s! Отправляю ваш конфиг. Чтобы получить его снова — кнопка «%s» ниже или команда /config. "+
			"(Done, %s! Sending your config. To get it again, use the «%s» button below or /config.)",
		html.EscapeString(c.Name), configButtonLabel, html.EscapeString(c.Name), configButtonLabel)))
	if deliver != nil {
		deliver(ctx, c)
	}
}

// isConfigRequest reports whether a message is a request for the client's config
// — the reply-keyboard button or a /config-style command.
func isConfigRequest(text string) bool {
	if text == configButtonLabel {
		return true
	}
	cmd := strings.ToLower(strings.TrimSpace(text))
	if i := strings.IndexAny(cmd, " @"); i >= 0 {
		cmd = cmd[:i] // strip args and "@botname"
	}
	switch cmd {
	case "/config", "/getconfig", "/link", "/get", "/sub":
		return true
	}
	return false
}

// setMyCommands registers the bot's command menu (best-effort). The billing
// commands are always listed; they simply do nothing while billing is disabled.
func (t *Telegram) setMyCommands(ctx context.Context, token string) {
	body, _ := json.Marshal(map[string]any{
		"commands": []map[string]string{
			{"command": "config", "description": "Получить мой VPN-конфиг (ссылка + QR)"},
			{"command": "status", "description": "Мой баланс и статус подписки"},
			{"command": "topup", "description": "Пополнить баланс"},
			{"command": "buy", "description": "Купить подписку"},
			{"command": "history", "description": "История операций"},
			{"command": "support", "description": "Связаться с поддержкой"},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/setMyCommands", t.apiBase, token), bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// getUpdates performs one long-poll and returns the updates plus the next offset.
func getUpdates(ctx context.Context, client *http.Client, apiBase, token string, offset int64) ([]tgUpdate, int64, error) {
	u := fmt.Sprintf("%s/bot%s/getUpdates?timeout=50&offset=%d", apiBase, token, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, offset, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, offset, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var out struct {
		OK          bool       `json:"ok"`
		Description string     `json:"description"`
		Result      []tgUpdate `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, offset, err
	}
	if !out.OK {
		if out.Description != "" {
			return nil, offset, fmt.Errorf("telegram: %s", out.Description)
		}
		return nil, offset, fmt.Errorf("telegram: getUpdates failed (HTTP %d)", resp.StatusCode)
	}
	next := offset
	for _, up := range out.Result {
		if up.UpdateID >= next {
			next = up.UpdateID + 1
		}
	}
	return out.Result, next, nil
}

// loadOffset / saveOffset persist the getUpdates cursor so a restart doesn't
// re-process (and re-deliver) links the panel already handled.
func (t *Telegram) loadOffset(ctx context.Context) int64 {
	v, _ := t.store.GetSetting(ctx, keyTgOffset)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func (t *Telegram) saveOffset(ctx context.Context, offset int64) {
	_ = t.store.SetSetting(ctx, keyTgOffset, strconv.FormatInt(offset, 10))
}

// sleepCtx sleeps for d or until ctx is done; returns true if ctx ended.
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
