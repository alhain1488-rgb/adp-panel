package backup

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adp/panel/internal/crypto"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/store"
)

func newTelegramForTest(t *testing.T, apiBase string) *Telegram {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	dbPath := filepath.Join(t.TempDir(), "panel.db")
	sdb, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sdb.Close() })

	c, err := crypto.NewCipher(key)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	svc := NewService(sdb, dbPath, key, "test")
	tg := NewTelegram(svc, store.New(sdb), c, slog.New(slog.NewTextHandler(io.Discard, nil)))
	tg.apiBase = apiBase
	return tg
}

func TestTelegramConfigRoundTripKeepsSecrets(t *testing.T) {
	ctx := context.Background()
	tg := newTelegramForTest(t, "https://example.invalid")

	if err := tg.SetConfig(ctx, TelegramInput{
		Enabled: true, Token: "BOT:TOKEN", ChatID: "42", Passphrase: "passphrase12", IntervalHours: 6,
	}); err != nil {
		t.Fatalf("set config: %v", err)
	}

	st, err := tg.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Enabled || !st.HasToken || !st.HasPassphrase || st.ChatID != "42" || st.IntervalHours != 6 {
		t.Fatalf("unexpected status: %+v", st)
	}

	// Token is stored encrypted, never in plaintext.
	raw, _ := tg.store.GetSetting(ctx, keyTgTokenEnc)
	if raw == "" || raw == "BOT:TOKEN" {
		t.Fatalf("token not encrypted at rest: %q", raw)
	}

	// A blank token on update keeps the existing one; other fields still change.
	if err := tg.SetConfig(ctx, TelegramInput{Enabled: false, ChatID: "99", IntervalHours: 12}); err != nil {
		t.Fatalf("second set: %v", err)
	}
	tok, chat, _, err := tg.credentials(ctx)
	if err != nil {
		t.Fatalf("credentials: %v", err)
	}
	if tok != "BOT:TOKEN" || chat != "99" {
		t.Fatalf("kept token=%q chat=%q", tok, chat)
	}
	st, _ = tg.Status(ctx)
	if st.Enabled {
		t.Fatal("should be disabled after update")
	}
}

func TestTelegramRunNowSendsAndRecords(t *testing.T) {
	ctx := context.Background()

	var gotPath, gotChat, gotDocName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = r.ParseMultipartForm(8 << 20)
		gotChat = r.FormValue("chat_id")
		if f, hdr, err := r.FormFile("document"); err == nil {
			gotDocName = hdr.Filename
			_ = f.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	defer srv.Close()

	tg := newTelegramForTest(t, srv.URL)
	if err := tg.SetConfig(ctx, TelegramInput{
		Enabled: true, Token: "BOT123", ChatID: "555", Passphrase: "passphrase12", IntervalHours: 24,
	}); err != nil {
		t.Fatalf("set config: %v", err)
	}

	if err := tg.RunNow(ctx); err != nil {
		t.Fatalf("run now: %v", err)
	}
	if gotPath != "/botBOT123/sendDocument" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotChat != "555" {
		t.Fatalf("chat = %q", gotChat)
	}
	if !strings.HasSuffix(gotDocName, ".adpbak") {
		t.Fatalf("document name = %q", gotDocName)
	}
	st, _ := tg.Status(ctx)
	if !st.LastOK || st.LastError != "" || st.LastAt == "" {
		t.Fatalf("expected success recorded, got %+v", st)
	}
}

func TestTelegramRunNowRecordsFailure(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":false,"description":"chat not found"}`)
	}))
	defer srv.Close()

	tg := newTelegramForTest(t, srv.URL)
	_ = tg.SetConfig(ctx, TelegramInput{
		Enabled: true, Token: "BOT123", ChatID: "555", Passphrase: "passphrase12", IntervalHours: 24,
	})

	err := tg.RunNow(ctx)
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("expected telegram error, got %v", err)
	}
	st, _ := tg.Status(ctx)
	if st.LastOK || !strings.Contains(st.LastError, "chat not found") {
		t.Fatalf("expected failure recorded, got %+v", st)
	}
}

func TestTelegramRunNowNeedsConfig(t *testing.T) {
	tg := newTelegramForTest(t, "https://example.invalid")
	if err := tg.RunNow(context.Background()); err == nil {
		t.Fatal("expected error when token/chat/passphrase are unset")
	}
}
