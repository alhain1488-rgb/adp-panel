package clients

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/amneziawg"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/store"
)

func newSvc(t *testing.T) (*Service, *store.Store, context.Context) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := store.New(database)
	return NewService(st), st, context.Background()
}

func TestCreate_GeneratesUniqueCredentials(t *testing.T) {
	svc, _, ctx := newSvc(t)
	a, err := svc.Create(ctx, Input{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Create(ctx, Input{Name: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if a.UUID == "" || a.Password == "" || a.SubscriptionToken == "" {
		t.Fatalf("blank credential: %+v", a)
	}
	if a.UUID == b.UUID || a.Password == b.Password || a.SubscriptionToken == b.SubscriptionToken {
		t.Error("credentials collide between clients")
	}
	if !a.Enabled {
		t.Error("new client should be enabled")
	}
}

func TestTelegramLinkToken_CreatedAndStable(t *testing.T) {
	svc, _, ctx := newSvc(t)
	a, err := svc.Create(ctx, Input{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// Create seeds a token so the deep link works without an extra round-trip.
	if a.TelegramLinkToken == "" {
		t.Fatal("Create should seed a telegram link token")
	}
	tok, err := svc.EnsureTelegramLinkToken(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tok != a.TelegramLinkToken {
		t.Fatalf("ensure changed the token: %q vs %q", tok, a.TelegramLinkToken)
	}
	// Distinct clients get distinct tokens.
	b, _ := svc.Create(ctx, Input{Name: "b"})
	if b.TelegramLinkToken == a.TelegramLinkToken {
		t.Fatal("link tokens collide between clients")
	}
}

func TestSubscriptionConfig_URLAndQR(t *testing.T) {
	svc, _, ctx := newSvc(t)
	c, err := svc.Create(ctx, Input{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	url, qr, err := svc.SubscriptionConfig(ctx, c.ID, "https://panel.example/")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://panel.example/sub/" + c.SubscriptionToken
	if url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
	if len(qr) == 0 || string(qr[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("qr is not a PNG (%d bytes)", len(qr))
	}
	// No base URL configured → a clear error, no panic.
	if _, _, err := svc.SubscriptionConfig(ctx, c.ID, ""); err == nil {
		t.Fatal("expected error when subBase is empty")
	}
}

func TestWGKeypair_GeneratedOnceAndStable(t *testing.T) {
	svc, _, ctx := newSvc(t)
	c, err := svc.Create(ctx, Input{Name: "wg"})
	if err != nil {
		t.Fatal(err)
	}

	priv, pub, err := svc.WGKeypair(ctx, c.ID)
	if err != nil || priv == "" || pub == "" {
		t.Fatalf("keypair: %v (%q/%q)", err, priv, pub)
	}
	// The stored public key derives from the stored private key.
	if got, err := amneziawg.PublicFromPrivate(priv); err != nil || got != pub {
		t.Fatalf("public does not match private: %v", err)
	}
	// A second call returns the SAME persisted keypair (not a fresh one).
	priv2, pub2, err := svc.WGKeypair(ctx, c.ID)
	if err != nil || priv2 != priv || pub2 != pub {
		t.Fatalf("keypair not stable across calls: %v", err)
	}
	// Rotating the subscription token must NOT change the keypair.
	if _, err := svc.RotateToken(ctx, c.ID); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	priv3, _, _ := svc.WGKeypair(ctx, c.ID)
	if priv3 != priv {
		t.Fatal("keypair changed after token rotation")
	}
	// Different clients get different keypairs.
	c2, _ := svc.Create(ctx, Input{Name: "wg2"})
	privB, _, _ := svc.WGKeypair(ctx, c2.ID)
	if privB == priv {
		t.Fatal("two clients share a keypair")
	}
}

func TestLinks_BuildsPerActiveInbound(t *testing.T) {
	svc, st, ctx := newSvc(t)
	srv, _ := st.CreateServer(ctx, store.ServerParams{Name: "n", Host: "example.net", SSHPort: 22, SSHUser: "root", SSHAuthMethod: "password"})
	in, _ := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "v", Protocol: "vless", Port: 443, Enabled: true,
		SettingsJSON: `{"decryption":"none"}`, StreamSettingsJSON: `{"network":"tcp","security":"none"}`,
	})
	c, _ := svc.Create(ctx, Input{Name: "c"})
	if err := st.SetClientInbounds(ctx, c.ID, []int64{in.ID}); err != nil {
		t.Fatal(err)
	}

	links, err := svc.Links(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1", len(links))
	}
	if links[0].Protocol != "vless" || links[0].URI == "" {
		t.Errorf("link = %+v", links[0])
	}
}

func TestRotateToken_Changes(t *testing.T) {
	svc, _, ctx := newSvc(t)
	c, _ := svc.Create(ctx, Input{Name: "c"})
	old := c.SubscriptionToken
	rot, err := svc.RotateToken(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rot.SubscriptionToken == old {
		t.Error("token unchanged after rotate")
	}
}

func TestCreateSelfSignup(t *testing.T) {
	svc, st, ctx := newSvc(t)
	srv, err := st.CreateServer(ctx, store.ServerParams{
		Name: "n", Host: "example.net", SSHPort: 22, SSHUser: "root", SSHAuthMethod: "password",
	})
	if err != nil {
		t.Fatal(err)
	}
	on1, err := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "on1", Protocol: "vless", Listen: "0.0.0.0", Port: 443, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	on2, err := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "on2", Protocol: "trojan", Listen: "0.0.0.0", Port: 8443, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// A disabled inbound must not be handed out.
	if _, err := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "off", Protocol: "vmess", Listen: "0.0.0.0", Port: 9443, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}

	c, err := svc.CreateSelfSignup(ctx, "tg:@someone", "555001", "someone")
	if err != nil {
		t.Fatal(err)
	}

	// Until they pay they must hold no access at all: disabled, unmanaged, no
	// subscription — and therefore absent from every engine config.
	if c.Enabled {
		t.Fatal("a self-signed-up client must start disabled")
	}
	if c.ActiveUntil != "" || c.BillingManaged {
		t.Fatalf("unexpected subscription state: until=%q managed=%v", c.ActiveUntil, c.BillingManaged)
	}
	if c.BillingExempt {
		t.Fatal("a paying signup must not get lifetime free access")
	}
	if c.TelegramChatID != "555001" {
		t.Fatalf("chat id = %q, want 555001", c.TelegramChatID)
	}
	if c.UUID == "" || c.Password == "" || c.SubscriptionToken == "" {
		t.Fatalf("blank credential: %+v", c)
	}

	// Granted exactly the enabled inbounds.
	ids, err := svc.GrantedInboundIDs(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]bool{on1.ID: true, on2.ID: true}
	if len(ids) != len(want) {
		t.Fatalf("granted %v, want exactly the enabled inbounds %v", ids, want)
	}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("granted inbound %d, which is disabled", id)
		}
	}

	// The bot finds them by chat id, so a second /start reuses this client.
	found, err := st.GetClientByTelegramChatID(ctx, "555001")
	if err != nil || found.ID != c.ID {
		t.Fatalf("lookup by chat id = (%v, %v), want client %d", found, err, c.ID)
	}

	// Disabled clients are excluded from what sync pushes to a node.
	grantees, err := st.ListInboundGrantedClients(ctx, on1.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range grantees {
		if g.ID == c.ID {
			t.Fatal("an unpaid signup leaked into the node config")
		}
	}
}
