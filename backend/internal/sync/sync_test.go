package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adp/panel/internal/amneziawg"
	"github.com/adp/panel/internal/db"
	"github.com/adp/panel/internal/protocols"
	"github.com/adp/panel/internal/ssh"
	"github.com/adp/panel/internal/ssh/sshtest"
	"github.com/adp/panel/internal/store"
)

// fakeKeyer returns deterministic per-client keys for AmneziaWG peer tests.
type fakeKeyer struct{}

func (fakeKeyer) WGKeypair(_ context.Context, id int64) (string, string, error) {
	return fmt.Sprintf("PRIV%d", id), fmt.Sprintf("PUB%d", id), nil
}

// fakeConnector hands out a preconfigured runner (Sync ignores the returned server).
type fakeConnector struct {
	runner ssh.Runner
	err    error
}

func (f *fakeConnector) Connect(_ context.Context, _ int64) (ssh.Runner, *store.Server, error) {
	return f.runner, nil, f.err
}

// newFixture builds an installed server with one xray inbound and one hysteria2
// inbound, a client, and grants to both.
func newFixture(t *testing.T) (*store.Store, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := store.New(database)
	ctx := context.Background()

	srv, err := st.CreateServer(ctx, store.ServerParams{
		Name: "n1", Host: "203.0.113.9", SSHPort: 22, SSHUser: "root", SSHAuthMethod: "password",
		XrayConfigPath: "/usr/local/etc/xray/config.json", XrayServiceName: "xray",
		HysteriaConfigPath: "/etc/sing-box/config.json", HysteriaServiceName: "sing-box",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProvision(ctx, srv.ID, "installed", "", ""); err != nil {
		t.Fatal(err)
	}

	vless, err := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "vless-tcp", Protocol: "vless", Port: 443, Enabled: true,
		SettingsJSON:       `{"decryption":"none"}`,
		StreamSettingsJSON: `{"network":"tcp","security":"none"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	hy, err := st.CreateInbound(ctx, srv.ID, store.InboundParams{
		Tag: "hy2", Protocol: "hysteria2", Port: 36712, Enabled: true,
		SettingsJSON: `{"up":"100 mbps","down":"200 mbps"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := st.CreateClient(ctx, store.ClientParams{
		Name: "alice", UUID: "11111111-2222-3333-4444-555555555555",
		Password: "secretpw", SubscriptionToken: "tok123", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetClientInbounds(ctx, c.ID, []int64{vless.ID, hy.ID}); err != nil {
		t.Fatal(err)
	}
	return st, srv.ID
}

func TestSync_PushesBothEngines(t *testing.T) {
	st, serverID := newFixture(t)
	runner := &sshtest.MockRunner{} // nil handler → every command exits 0
	svc := NewService(st, &fakeConnector{runner: runner}, nil)

	results, err := svc.Sync(context.Background(), serverID)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for _, r := range results {
		if !r.Changed || r.Skipped {
			t.Errorf("engine %s: changed=%v skipped=%v", r.Engine, r.Changed, r.Skipped)
		}
	}
	// Both engines were written, validated, and restarted.
	for _, want := range []string{
		"cat > '/usr/local/etc/xray/config.json'",
		"cat > '/etc/sing-box/config.json'",
		"xray -test -config '/usr/local/etc/xray/config.json'",
		"sing-box check -c '/etc/sing-box/config.json'",
		"systemctl restart 'xray'",
		"systemctl restart 'sing-box'",
	} {
		if !runner.Ran(want) {
			t.Errorf("expected command to run: %s", want)
		}
	}

	// last_sync recorded without error.
	srv, _ := st.GetServer(context.Background(), serverID)
	if srv.LastSyncAt == "" || srv.LastSyncError != "" {
		t.Errorf("last_sync_at=%q err=%q", srv.LastSyncAt, srv.LastSyncError)
	}
}

func TestSync_Idempotent(t *testing.T) {
	st, serverID := newFixture(t)
	svc := NewService(st, &fakeConnector{runner: &sshtest.MockRunner{}}, nil)

	if _, err := svc.Sync(context.Background(), serverID); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	// Second sync with an unchanged config must skip both engines (no restart).
	runner2 := &sshtest.MockRunner{}
	svc2 := NewService(st, &fakeConnector{runner: runner2}, nil)
	results, err := svc2.Sync(context.Background(), serverID)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	for _, r := range results {
		if r.Changed || !r.Skipped {
			t.Errorf("engine %s not skipped on unchanged config", r.Engine)
		}
	}
	if runner2.Ran("systemctl restart") {
		t.Error("restart should not run when config is unchanged")
	}
}

func TestSync_ValidationFailureRestoresBackup(t *testing.T) {
	st, serverID := newFixture(t)
	runner := &sshtest.MockRunner{
		Handler: func(cmd, _ string) (ssh.Result, error) {
			if strings.Contains(cmd, "xray -test") {
				return ssh.Result{ExitCode: 1, Stderr: "bad config"}, nil
			}
			return ssh.Result{}, nil
		},
	}
	svc := NewService(st, &fakeConnector{runner: runner}, nil)

	_, err := svc.Sync(context.Background(), serverID)
	if err == nil {
		t.Fatal("expected sync error on invalid config")
	}
	if !strings.Contains(err.Error(), "rejected by engine") {
		t.Errorf("error = %v", err)
	}
	// Backup was restored and the service was NOT restarted.
	if !runner.Ran("cp '/usr/local/etc/xray/config.json.adp.bak' '/usr/local/etc/xray/config.json'") {
		t.Error("expected backup restore")
	}
	if runner.Ran("systemctl restart 'xray'") {
		t.Error("service must not restart when validation fails")
	}
	srv, _ := st.GetServer(context.Background(), serverID)
	if srv.LastSyncError == "" {
		t.Error("expected last_sync_error to be recorded")
	}
}

func TestSync_NotProvisioned(t *testing.T) {
	st, serverID := newFixture(t)
	// Roll the node back to a not-installed state.
	if err := st.SetProvision(context.Background(), serverID, "pending", "", ""); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, &fakeConnector{runner: &sshtest.MockRunner{}}, nil)
	if _, err := svc.Sync(context.Background(), serverID); err != ErrNotProvisioned {
		t.Fatalf("err = %v, want ErrNotProvisioned", err)
	}
}

func TestBuildConfigs_ShapeAndClients(t *testing.T) {
	st, serverID := newFixture(t)
	svc := NewService(st, &fakeConnector{runner: &sshtest.MockRunner{}}, nil)
	srv, _ := st.GetServer(context.Background(), serverID)

	plans, err := svc.plan(context.Background(), srv)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("plans = %d, want 2", len(plans))
	}
	for _, p := range plans {
		body := string(p.content)
		if !strings.Contains(body, `"inbounds"`) {
			t.Errorf("%s config missing inbounds: %s", p.engine, body)
		}
		// The granted client's credential must appear in the assembled config.
		switch p.engine {
		case "xray":
			if !strings.Contains(body, "11111111-2222-3333-4444-555555555555") {
				t.Errorf("xray config missing client uuid: %s", body)
			}
		case "hysteria":
			if !strings.Contains(body, "secretpw") {
				t.Errorf("hysteria config missing client password: %s", body)
			}
		}
	}
}

func TestBuildAWGPlan(t *testing.T) {
	spriv, spub, err := amneziawg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	settings := fmt.Sprintf(`{"private_key":%q,"public_key":%q,"subnet":"10.9.9.0/24",`+
		`"mtu":1280,"dns":"1.1.1.1","nat":true,"params":{"jc":4,"jmin":40,"jmax":90,`+
		`"s1":50,"s2":40,"s3":12,"s4":8,"h1":"111","h2":"222","h3":"333","h4":"444","i1":"<r 128>"}}`,
		spriv, spub)
	in := store.Inbound{ID: 7, Tag: "awg", Protocol: "amneziawg", Port: 51820, SettingsJSON: settings}
	grantees := []store.Client{{ID: 5, Name: "phone"}}

	svc := &Service{keyer: fakeKeyer{}}
	plan, err := svc.buildAWGPlan(context.Background(), in, grantees)
	if err != nil {
		t.Fatalf("buildAWGPlan: %v", err)
	}
	if plan.path != "/etc/amnezia/amneziawg/awg7.conf" || plan.service != "awg-quick@awg7" || plan.key != "amneziawg:awg7" {
		t.Fatalf("plan routing: path=%q service=%q key=%q", plan.path, plan.service, plan.key)
	}
	c := string(plan.content)
	for _, want := range []string{
		"[Interface]", "PrivateKey = " + spriv, "Address = 10.9.9.1/24", "ListenPort = 51820",
		"Jc = 4", "H1 = 111", "PostUp", // server obfuscation + NAT
		"[Peer]", "# phone", "PublicKey = PUB5", "AllowedIPs = 10.9.9.6/32", // client id 5 -> .6
	} {
		if !strings.Contains(c, want) {
			t.Errorf("server .conf missing %q", want)
		}
	}
	// The client-only CPS packet must not leak into the server interface.
	if strings.Contains(c, "I1 = ") {
		t.Error("server .conf should not carry I1")
	}
}

func TestReconcileAWG_TearsDownOrphans(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st := store.New(database)
	ctx := context.Background()

	// awg7 is still desired; awg99's inbound was removed. Seed both idempotency
	// markers so we can prove only the orphan's is cleared.
	_ = st.SetSetting(ctx, "sync:1:amneziawg:awg7", "hash7")
	_ = st.SetSetting(ctx, "sync:1:amneziawg:awg99", "hash99")

	runner := &sshtest.MockRunner{
		Handler: func(cmd, _ string) (ssh.Result, error) {
			if strings.Contains(cmd, "ls -1 '/etc/amnezia/amneziawg'") {
				// A desired iface, an orphan, and a non-matching stray file.
				return ssh.Result{Stdout: "awg7.conf\nawg99.conf\nkeep-me.txt\n"}, nil
			}
			return ssh.Result{}, nil
		},
	}
	svc := NewService(st, &fakeConnector{runner: runner}, fakeKeyer{})
	plans := []enginePlan{{engine: protocols.EngineAmneziaWG, service: "awg-quick@awg7"}}

	svc.reconcileAWG(ctx, runner, 1, plans)

	// Orphan awg99 is stopped, disabled and its config removed.
	if !runner.Ran("systemctl disable --now awg-quick@awg99") {
		t.Error("orphan awg99 was not torn down")
	}
	if !runner.Ran("rm -f '/etc/amnezia/amneziawg/awg99.conf'") {
		t.Error("orphan awg99 config was not removed")
	}
	// The desired interface and the stray file are left untouched.
	if runner.Ran("disable --now awg-quick@awg7") {
		t.Error("desired awg7 must not be torn down")
	}
	if runner.Ran("keep-me.txt") {
		t.Error("non-awg files must never be touched")
	}
	// Only the orphan's idempotency marker is cleared.
	if v, _ := st.GetSetting(ctx, "sync:1:amneziawg:awg99"); v != "" {
		t.Errorf("orphan marker not cleared: %q", v)
	}
	if v, _ := st.GetSetting(ctx, "sync:1:amneziawg:awg7"); v != "hash7" {
		t.Errorf("desired marker was wrongly cleared: %q", v)
	}
}
