package subscription

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adp/panel/internal/clients"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/store"
)

func setup(t *testing.T) (*Service, *clients.Service, *store.Store, context.Context) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := store.New(database)
	cs := clients.NewService(st)
	return NewService(st, cs), cs, st, context.Background()
}

func TestBuild_EncodesActiveInbounds(t *testing.T) {
	sub, cs, st, ctx := setup(t)
	srv, _ := st.CreateServer(ctx, store.ServerParams{Name: "n", Host: "h.example", SSHPort: 22, SSHUser: "root", SSHAuthMethod: "password"})
	in, _ := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "v", Protocol: "vless", Port: 443, Enabled: true,
		SettingsJSON: `{"decryption":"none"}`, StreamSettingsJSON: `{"network":"tcp","security":"none"}`,
	})
	c, _ := cs.Create(ctx, clients.Input{Name: "a"})
	_ = st.SetClientInbounds(ctx, c.ID, []int64{in.ID})

	body, err := sub.Build(ctx, c.SubscriptionToken)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("not base64: %v", err)
	}
	if !strings.HasPrefix(string(raw), "vless://") {
		t.Errorf("decoded = %q", string(raw))
	}
}

func TestBuild_DisabledClientIsEmpty(t *testing.T) {
	sub, cs, st, ctx := setup(t)
	c, _ := cs.Create(ctx, clients.Input{Name: "a"})
	if _, err := st.SetClientEnabled(ctx, c.ID, false); err != nil {
		t.Fatal(err)
	}
	body, err := sub.Build(ctx, c.SubscriptionToken)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		t.Errorf("disabled subscription = %q, want empty", body)
	}
}

func TestBuild_UnknownTokenNotFound(t *testing.T) {
	sub, _, _, ctx := setup(t)
	if _, err := sub.Build(ctx, "nope"); err != store.ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
