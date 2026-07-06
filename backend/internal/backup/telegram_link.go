package backup

import (
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

// RunLinkPoller long-polls getUpdates and binds a Telegram chat to a client when
// the user opens their "/start <link_token>" deep link. onLink (may be nil) runs
// after a successful bind — used to push the client's config immediately. It runs
// until ctx is cancelled and tolerates the bot being unconfigured (waits and
// retries), so it is safe to start unconditionally.
func (t *Telegram) RunLinkPoller(ctx context.Context, onLink func(context.Context, *store.Client)) {
	// Long-poll needs a client timeout comfortably above the poll timeout.
	client := &http.Client{Timeout: 70 * time.Second}
	offset := t.loadOffset(ctx)
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
			t.handleUpdate(ctx, updates[i], onLink)
		}
	}
}

// handleUpdate processes one update: a "/start <token>" links the sender's chat
// to the matching client.
func (t *Telegram) handleUpdate(ctx context.Context, u tgUpdate, onLink func(context.Context, *store.Client)) {
	if u.Message == nil {
		return
	}
	text := strings.TrimSpace(u.Message.Text)
	if !strings.HasPrefix(text, "/start") {
		return
	}
	chatID := strconv.FormatInt(u.Message.Chat.ID, 10)
	fields := strings.Fields(text)
	if len(fields) < 2 {
		_ = t.SendMessageTo(ctx, chatID,
			"Откройте персональную ссылку, которую дал вам администратор, чтобы получать сюда свой VPN-конфиг.")
		return
	}
	c, err := t.store.LinkClientTelegram(ctx, fields[1], chatID, u.Message.From.Username)
	if err != nil {
		_ = t.SendMessageTo(ctx, chatID, "Ссылка недействительна. Попросите у администратора новую.")
		return
	}
	_ = t.SendMessageTo(ctx, chatID,
		fmt.Sprintf("✅ Готово, %s! Теперь вы будете получать сюда свой VPN-конфиг.", html.EscapeString(c.Name)))
	if onLink != nil {
		onLink(ctx, c)
	}
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
