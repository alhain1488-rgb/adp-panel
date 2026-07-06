package backup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adp/panel/internal/store"
)

// decodeUpdate builds a tgUpdate from JSON so tests needn't spell out the
// anonymous Message struct.
func decodeUpdate(t *testing.T, raw string) tgUpdate {
	t.Helper()
	var u tgUpdate
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	return u
}

func TestHandleUpdate_LinksAndDelivers(t *testing.T) {
	ctx := context.Background()
	var messages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			b, _ := io.ReadAll(r.Body)
			messages = append(messages, string(b))
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	tg := newTelegramForTest(t, srv.URL)
	if err := tg.SetConfig(ctx, TelegramInput{
		Enabled: true, Token: "BOT:TOKEN", ChatID: "1", Passphrase: "passphrase12", IntervalHours: 6,
	}); err != nil {
		t.Fatal(err)
	}
	c, err := tg.store.CreateClient(ctx, store.ClientParams{
		Name: "carol", UUID: "u", Password: "p", SubscriptionToken: "tk", Enabled: true,
		TelegramLinkToken: "deep-tok",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Valid deep link → binds chat, confirms, and fires onLink.
	var delivered *store.Client
	tg.handleUpdate(ctx,
		decodeUpdate(t, `{"update_id":1,"message":{"text":"/start deep-tok","chat":{"id":555001},"from":{"username":"carol_tg"}}}`),
		func(_ context.Context, cc *store.Client) { delivered = cc })
	if delivered == nil || delivered.ID != c.ID {
		t.Fatalf("onLink not called for the right client: %+v", delivered)
	}
	got, _ := tg.store.GetClient(ctx, c.ID)
	if got.TelegramChatID != "555001" || got.TelegramUsername != "carol_tg" {
		t.Fatalf("chat not stored: %+v", got)
	}
	if len(messages) == 0 || !strings.Contains(messages[len(messages)-1], "carol") {
		t.Fatalf("expected a confirmation mentioning the client, got %v", messages)
	}

	// Unknown token → no onLink, no chat bound to anyone.
	called := false
	tg.handleUpdate(ctx,
		decodeUpdate(t, `{"update_id":2,"message":{"text":"/start nope","chat":{"id":999}}}`),
		func(_ context.Context, _ *store.Client) { called = true })
	if called {
		t.Fatal("onLink should not fire for an unknown token")
	}

	// Bare /start (no payload) → help message, no linking.
	before := len(messages)
	tg.handleUpdate(ctx,
		decodeUpdate(t, `{"update_id":3,"message":{"text":"/start","chat":{"id":42}}}`),
		func(_ context.Context, _ *store.Client) { t.Fatal("onLink should not fire for bare /start") })
	if len(messages) != before+1 {
		t.Fatalf("expected one help message, got %d new", len(messages)-before)
	}

	// Non-command messages are ignored entirely.
	tg.handleUpdate(ctx,
		decodeUpdate(t, `{"update_id":4,"message":{"text":"hello","chat":{"id":42}}}`),
		func(_ context.Context, _ *store.Client) { t.Fatal("onLink should not fire for plain text") })
}

func TestHandleUpdate_ConfigRequest(t *testing.T) {
	ctx := context.Background()
	var messages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			b, _ := io.ReadAll(r.Body)
			messages = append(messages, string(b))
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	tg := newTelegramForTest(t, srv.URL)
	_ = tg.SetConfig(ctx, TelegramInput{
		Enabled: true, Token: "BOT:TOKEN", ChatID: "1", Passphrase: "passphrase12", IntervalHours: 6,
	})
	c, err := tg.store.CreateClient(ctx, store.ClientParams{
		Name: "dave", UUID: "u", Password: "p", SubscriptionToken: "tk", Enabled: true, TelegramLinkToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tg.store.LinkClientTelegram(ctx, "tok", "900", "dave_tg"); err != nil {
		t.Fatal(err)
	}

	deliveries := 0
	deliver := func(_ context.Context, cc *store.Client) {
		if cc.ID == c.ID {
			deliveries++
		}
	}

	// /config, the reply-keyboard button, and a bare /start from the linked chat
	// all re-deliver the config.
	for _, raw := range []string{
		`{"update_id":1,"message":{"text":"/config","chat":{"id":900}}}`,
		`{"update_id":2,"message":{"text":"🔄 Получить конфиг","chat":{"id":900}}}`,
		`{"update_id":3,"message":{"text":"/start","chat":{"id":900}}}`,
	} {
		tg.handleUpdate(ctx, decodeUpdate(t, raw), deliver)
	}
	if deliveries != 3 {
		t.Fatalf("deliveries = %d, want 3", deliveries)
	}

	// /config from an unlinked chat delivers nothing and explains how to link.
	before := len(messages)
	tg.handleUpdate(ctx,
		decodeUpdate(t, `{"update_id":4,"message":{"text":"/config","chat":{"id":111}}}`),
		func(_ context.Context, _ *store.Client) { t.Fatal("must not deliver to an unlinked chat") })
	if len(messages) != before+1 {
		t.Fatalf("expected one 'not linked' message, got %d new", len(messages)-before)
	}
}

func TestSendClientConfig_AttachesKeyboard(t *testing.T) {
	ctx := context.Background()
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	defer srv.Close()

	tg := newTelegramForTest(t, srv.URL)
	_ = tg.SetConfig(ctx, TelegramInput{
		Enabled: true, Token: "BOT:TOKEN", ChatID: "1", Passphrase: "passphrase12", IntervalHours: 6,
	})
	if err := tg.SendClientConfig(ctx, "900", "dave", "https://x/sub/tk", []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "reply_markup") || !strings.Contains(body, "resize_keyboard") {
		t.Fatalf("config photo missing reply keyboard: %q", body)
	}
	if !strings.Contains(body, "<code>") {
		t.Fatalf("config caption missing tap-to-copy link: %q", body)
	}
}

func TestGetUpdates_AdvancesOffset(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"ok":true,"result":[
			{"update_id":41,"message":{"text":"/start a","chat":{"id":5}}},
			{"update_id":42}
		]}`)
	}))
	defer srv.Close()

	ups, next, err := getUpdates(context.Background(), http.DefaultClient, srv.URL, "TOK", 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(ups) != 2 {
		t.Fatalf("updates = %d", len(ups))
	}
	if next != 43 {
		t.Fatalf("next offset = %d, want 43", next)
	}
	if !strings.Contains(gotQuery, "offset=40") {
		t.Fatalf("request did not carry the offset: %q", gotQuery)
	}
}

func TestGetUpdates_SurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ok":false,"description":"Unauthorized"}`)
	}))
	defer srv.Close()

	_, next, err := getUpdates(context.Background(), http.DefaultClient, srv.URL, "TOK", 7)
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected API error, got %v", err)
	}
	if next != 7 {
		t.Fatalf("offset must not advance on error, got %d", next)
	}
}
