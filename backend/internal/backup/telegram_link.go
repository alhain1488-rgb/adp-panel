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

	"github.com/adp/panel/internal/store"
)

// tgUpdate is the subset of a Telegram update we act on: a text message (we only
// care about "/start <token>" deep links).
type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
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
		if next != offset {
			offset = next
			t.saveOffset(ctx, offset)
		}
		for i := range updates {
			t.handleUpdate(ctx, updates[i], deliver)
		}
	}
}

// handleUpdate processes one update: "/start <token>" links the sender's chat to
// the matching client; "/config" (or the reply-keyboard button, or a bare /start
// from an already-linked chat) re-delivers that chat's client config.
func (t *Telegram) handleUpdate(ctx context.Context, u tgUpdate, deliver func(context.Context, *store.Client)) {
	if u.Message == nil {
		return
	}
	text := strings.TrimSpace(u.Message.Text)
	if text == "" {
		return
	}
	chatID := strconv.FormatInt(u.Message.Chat.ID, 10)

	// On-demand config request from a linked client.
	if isConfigRequest(text) {
		c, err := t.store.GetClientByTelegramChatID(ctx, chatID)
		if err != nil {
			_ = t.SendMessageTo(ctx, chatID,
				"Вы ещё не привязаны. Откройте персональную ссылку, которую дал вам администратор.")
			return
		}
		if deliver != nil {
			deliver(ctx, c)
		}
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
			if deliver != nil {
				deliver(ctx, c)
			}
			return
		}
		_ = t.SendMessageTo(ctx, chatID,
			"Откройте персональную ссылку, которую дал вам администратор, чтобы получать сюда свой VPN-конфиг.")
		return
	}
	c, err := t.store.LinkClientTelegram(ctx, fields[1], chatID, u.Message.From.Username)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, "Ссылка недействительна. Попросите у администратора новую.")
		return
	}
	_ = t.SendMessageTo(ctx, chatID, fmt.Sprintf(
		"✅ Готово, %s! Отправляю ваш конфиг. Чтобы получить его снова — кнопка «%s» ниже или команда /config.",
		html.EscapeString(c.Name), configButtonLabel))
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

// setMyCommands registers the "/config" command in the bot's menu (best-effort).
func (t *Telegram) setMyCommands(ctx context.Context, token string) {
	body, _ := json.Marshal(map[string]any{
		"commands": []map[string]string{
			{"command": "config", "description": "Получить мой VPN-конфиг (ссылка + QR)"},
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
