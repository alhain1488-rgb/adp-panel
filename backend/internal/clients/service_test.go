package clients

import (
	"context"
	"path/filepath"
	"testing"

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
