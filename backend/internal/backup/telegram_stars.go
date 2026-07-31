package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// StarLedger is the bot's own Telegram Stars position: what Telegram is holding
// for it and how it got there. Paid Stars accrue to the bot, not to the panel and
// not to the operator's personal balance, so this is the only place the money is
// visible before it is withdrawn through Fragment.
type StarLedger struct {
	// Configured reports whether a bot token is set at all; without one the rest
	// is meaningless rather than zero.
	Configured bool `json:"configured"`
	// BalanceStars is the withdrawable-in-principle balance. Telegram holds newly
	// earned Stars for 21 days (the refund window), so part of this may not be
	// withdrawable yet — the Bot API does not break that down.
	BalanceStars int64             `json:"balance_stars"`
	Transactions []StarTransaction `json:"transactions"`
}

// StarTransaction is one movement on the bot's Star balance.
type StarTransaction struct {
	ID        string `json:"id"`
	Stars     int64  `json:"stars"`
	Incoming  bool   `json:"incoming"` // false = a refund or withdrawal leaving the bot
	CreatedAt string `json:"created_at"`
	// Peer is the counterparty as Telegram reports it ("user", "fragment", …).
	Peer string `json:"peer,omitempty"`
}

// StarBalance reads the bot's Star balance and recent transactions. Returns a
// zero-value ledger with Configured=false when no bot token is set.
func (t *Telegram) StarBalance(ctx context.Context, limit int) (StarLedger, error) {
	token, err := t.tokenOnly(ctx)
	if err != nil || token == "" {
		return StarLedger{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	var bal struct {
		Amount int64 `json:"amount"`
	}
	if err := t.callBotAPI(ctx, token, "getMyStarBalance", nil, &bal); err != nil {
		return StarLedger{}, err
	}

	var txs struct {
		Transactions []struct {
			ID     string `json:"id"`
			Amount int64  `json:"amount"`
			Date   int64  `json:"date"`
			Source *struct {
				Type string `json:"type"`
			} `json:"source"`
			Receiver *struct {
				Type string `json:"type"`
			} `json:"receiver"`
		} `json:"transactions"`
	}
	if err := t.callBotAPI(ctx, token, "getStarTransactions",
		map[string]any{"limit": limit}, &txs); err != nil {
		return StarLedger{}, err
	}

	out := StarLedger{Configured: true, BalanceStars: bal.Amount}
	out.Transactions = make([]StarTransaction, 0, len(txs.Transactions))
	for _, x := range txs.Transactions {
		// Telegram marks direction by which side is populated: a filled "source"
		// means Stars came in, a filled "receiver" means they left.
		tx := StarTransaction{
			ID:        x.ID,
			Stars:     x.Amount,
			Incoming:  x.Source != nil,
			CreatedAt: time.Unix(x.Date, 0).UTC().Format(time.RFC3339),
		}
		switch {
		case x.Source != nil:
			tx.Peer = x.Source.Type
		case x.Receiver != nil:
			tx.Peer = x.Receiver.Type
		}
		out.Transactions = append(out.Transactions, tx)
	}
	return out, nil
}

// callBotAPI performs one Bot API call and decodes its "result" into out.
func (t *Telegram) callBotAPI(ctx context.Context, token, method string, payload map[string]any, out any) error {
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	} else {
		body = []byte("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/%s", t.apiBase, token, method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("telegram: bad response to %s: %w", method, err)
	}
	if !env.OK {
		if env.Description != "" {
			return fmt.Errorf("telegram: %s: %s", method, env.Description)
		}
		return fmt.Errorf("telegram: %s failed (HTTP %d)", method, resp.StatusCode)
	}
	if out == nil || len(env.Result) == 0 {
		return nil
	}
	return json.Unmarshal(env.Result, out)
}
