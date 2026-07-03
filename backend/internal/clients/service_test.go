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
