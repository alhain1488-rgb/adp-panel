package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/db"
)

func newStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return New(database), context.Background()
}

func seedServerWithInbounds(t *testing.T, st *Store, ctx context.Context) (serverID, in1, in2 int64) {
	t.Helper()
	srv, err := st.CreateServer(ctx, ServerParams{Name: "n", Host: "h", SSHPort: 22, SSHUser: "root", SSHAuthMethod: "password"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.CreateInbound(ctx, srv.ID, InboundParams{Tag: "a", Protocol: "vless", Port: 443, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.CreateInbound(ctx, srv.ID, InboundParams{Tag: "b", Protocol: "hysteria2", Port: 36712, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return srv.ID, a.ID, b.ID
}

func TestClient_CRUDAndToggle(t *testing.T) {
	st, ctx := newStore(t)
	c, err := st.CreateClient(ctx, ClientParams{
		Name: "alice", UUID: "u-1", Password: "pw", SubscriptionToken: "tok-1", Enabled: true, Remark: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled || c.Name != "alice" {
		t.Fatalf("client = %+v", c)
	}

	got, err := st.GetClientByToken(ctx, "tok-1")
	if err != nil || got.ID != c.ID {
		t.Fatalf("by token: %v %+v", err, got)
	}

	if _, err := st.SetClientEnabled(ctx, c.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetClient(ctx, c.ID)
	if got.Enabled {
		t.Error("expected disabled")
	}

	rot, err := st.RotateClientToken(ctx, c.ID, "tok-2")
	if err != nil {
		t.Fatal(err)
	}
	if rot.SubscriptionToken != "tok-2" {
		t.Errorf("token = %q", rot.SubscriptionToken)
	}
	if _, err := st.GetClientByToken(ctx, "tok-1"); err != ErrNotFound {
		t.Errorf("old token still resolves: %v", err)
	}

	if err := st.DeleteClient(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetClient(ctx, c.ID); err != ErrNotFound {
		t.Errorf("delete: %v", err)
	}
}

func TestClient_TelegramLinking(t *testing.T) {
	st, ctx := newStore(t)
	c, err := st.CreateClient(ctx, ClientParams{
		Name: "carol", UUID: "u", Password: "p", SubscriptionToken: "tk", Enabled: true,
		TelegramLinkToken: "tok-abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.TelegramLinkToken != "tok-abc" || c.TelegramChatID != "" {
		t.Fatalf("fresh client tg = %+v", c)
	}

	// A bare/empty token must never match a client (guards against /start with no payload).
	if _, err := st.LinkClientTelegram(ctx, "", "123", "eve"); err != ErrNotFound {
		t.Fatalf("empty token linked: %v", err)
	}
	if _, err := st.LinkClientTelegram(ctx, "does-not-exist", "123", "eve"); err != ErrNotFound {
		t.Fatalf("unknown token linked: %v", err)
	}

	linked, err := st.LinkClientTelegram(ctx, "tok-abc", "555001", "carol_tg")
	if err != nil {
		t.Fatal(err)
	}
	if linked.TelegramChatID != "555001" || linked.TelegramUsername != "carol_tg" {
		t.Fatalf("linked = %+v", linked)
	}

	// Reverse lookup by chat id (used to answer in-bot config requests).
	byChat, err := st.GetClientByTelegramChatID(ctx, "555001")
	if err != nil || byChat.ID != c.ID {
		t.Fatalf("by chat = %+v, %v", byChat, err)
	}
	if _, err := st.GetClientByTelegramChatID(ctx, ""); err != ErrNotFound {
		t.Fatalf("empty chat matched: %v", err)
	}
	if _, err := st.GetClientByTelegramChatID(ctx, "404"); err != ErrNotFound {
		t.Fatalf("unknown chat matched: %v", err)
	}

	cleared, err := st.ClearClientTelegram(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.TelegramChatID != "" || cleared.TelegramUsername != "" {
		t.Fatalf("cleared = %+v", cleared)
	}
	// The link token survives an unlink so the same deep link keeps working.
	if cleared.TelegramLinkToken != "tok-abc" {
		t.Fatalf("link token lost on unlink: %q", cleared.TelegramLinkToken)
	}
}

func TestClient_GrantsAndActiveInbounds(t *testing.T) {
	st, ctx := newStore(t)
	_, in1, in2 := seedServerWithInbounds(t, st, ctx)
	c, _ := st.CreateClient(ctx, ClientParams{Name: "bob", UUID: "u", Password: "p", SubscriptionToken: "tk", Enabled: true})

	if err := st.SetClientInbounds(ctx, c.ID, []int64{in1, in2}); err != nil {
		t.Fatal(err)
	}
	ids, _ := st.GrantedInboundIDs(ctx, c.ID)
	if len(ids) != 2 {
		t.Fatalf("granted ids = %v", ids)
	}

	active, err := st.ListActiveClientInbounds(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Fatalf("active = %d, want 2", len(active))
	}
	if active[0].ServerHost != "h" {
		t.Errorf("server host = %q", active[0].ServerHost)
	}

	// Disabling an inbound removes it from the active (subscription) set.
	if _, err := st.UpdateInbound(ctx, in2, InboundParams{Tag: "b", Protocol: "hysteria2", Port: 36712, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	active, _ = st.ListActiveClientInbounds(ctx, c.ID)
	if len(active) != 1 {
		t.Fatalf("active after disable = %d, want 1", len(active))
	}

	// Re-granting a subset replaces the previous grants.
	if err := st.SetClientInbounds(ctx, c.ID, []int64{in1}); err != nil {
		t.Fatal(err)
	}
	ids, _ = st.GrantedInboundIDs(ctx, c.ID)
	if len(ids) != 1 || ids[0] != in1 {
		t.Fatalf("regrant ids = %v", ids)
	}
}

func TestClient_InboundGrantedClientsFiltersDisabled(t *testing.T) {
	st, ctx := newStore(t)
	_, in1, _ := seedServerWithInbounds(t, st, ctx)

	on, _ := st.CreateClient(ctx, ClientParams{Name: "on", UUID: "u1", Password: "p1", SubscriptionToken: "t1", Enabled: true})
	off, _ := st.CreateClient(ctx, ClientParams{Name: "off", UUID: "u2", Password: "p2", SubscriptionToken: "t2", Enabled: false})
	_ = st.SetClientInbounds(ctx, on.ID, []int64{in1})
	_ = st.SetClientInbounds(ctx, off.ID, []int64{in1})

	got, err := st.ListInboundGrantedClients(ctx, in1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "on" {
		t.Fatalf("granted clients = %+v, want only the enabled one", got)
	}
}
