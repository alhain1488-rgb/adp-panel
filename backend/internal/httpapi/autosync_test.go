package httpapi

import (
	"context"
	"testing"
	"time"
)

// waitFor polls cond until it is true or the timeout elapses.
func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// TestAutoSync_OnInboundChange verifies that editing inbounds auto-pushes config
// to an installed node — no manual sync step. (The non-installed skip path is
// covered by sync.TestSync_NotProvisioned.) A "systemctl restart" can only come
// from a sync here, so its presence proves auto-sync fired.
func TestAutoSync_OnInboundChange(t *testing.T) {
	env := newTestEnv(t)
	seedAdmin(t, env)
	token := loginToken(t, env)
	srv := createServer(t, env, token, "n")
	if err := env.store.SetProvision(context.Background(), srv.ID, "installed", "", ""); err != nil {
		t.Fatal(err)
	}
	createInbound(t, env, token, srv.ID, map[string]any{
		"tag": "v1", "protocol": "vless", "port": 8443,
		"settings": map[string]any{"decryption": "none"},
	})
	if !waitFor(3*time.Second, func() bool { return env.runner.Ran("systemctl restart") }) {
		t.Error("expected auto-sync to push config to the installed node after inbound create")
	}
}
