//go:build integration

// Integration proves that the config the SYNC engine assembles from stored
// inbounds + granted clients is accepted by the real engines. Requires Docker
// (Colima) and network. Run with:
//
//	go test -tags=integration -run Integration ./internal/sync/ -v
package sync

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adp/panel/internal/protocols"
)

const (
	xrayImage    = "ghcr.io/xtls/xray-core:latest"
	singBoxImage = "ghcr.io/sagernet/sing-box:latest"
)

func dockerRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("docker", append([]string{"run", "--rm"}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// mountDir writes bytes as config.json in a fresh dir under the package (so
// Colima can mount it) and returns the absolute path.
func mountDir(t *testing.T, content []byte) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "sync-validate-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(abs, "config.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

func genSelfSigned(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
		"-keyout", filepath.Join(dir, "self.key"), "-out", filepath.Join(dir, "self.crt"),
		"-days", "1", "-subj", "/CN=node.example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("openssl: %v\n%s", err, out)
	}
}

// planFor returns the assembled config bytes for a given engine from a live
// store fixture (the exact bytes sync would push).
func planFor(t *testing.T, engine protocols.Engine) []byte {
	t.Helper()
	st, serverID := newFixture(t)
	svc := NewService(st, &fakeConnector{})
	srv, _ := st.GetServer(context.Background(), serverID)
	plans, err := svc.plan(context.Background(), srv)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plans {
		if p.engine == engine {
			return p.content
		}
	}
	t.Fatalf("no plan for engine %s", engine)
	return nil
}

// TestIntegration_SyncXrayConfigAccepted asserts the sync-assembled xray config
// passes `xray -test`.
func TestIntegration_SyncXrayConfigAccepted(t *testing.T) {
	content := planFor(t, protocols.EngineXray)
	dir := mountDir(t, content)
	out, err := dockerRun(t, "-v", dir+":/work:ro", xrayImage, "-test", "-config", "/work/config.json")
	if err != nil {
		t.Fatalf("xray -test rejected sync config: %v\n%s", err, out)
	}
	t.Logf("xray -test output:\n%s", out)
}

// TestIntegration_SyncSingBoxConfigAccepted asserts the sync-assembled sing-box
// config passes `sing-box check` (with its TLS cert paths pointed at a real
// self-signed cert in the mounted dir).
func TestIntegration_SyncSingBoxConfigAccepted(t *testing.T) {
	content := planFor(t, protocols.EngineHysteria)

	var cfg map[string]any
	if err := json.Unmarshal(content, &cfg); err != nil {
		t.Fatal(err)
	}
	inbounds, _ := cfg["inbounds"].([]any)
	if len(inbounds) == 0 {
		t.Fatal("no hysteria inbounds in sync config")
	}
	in0 := inbounds[0].(map[string]any)
	tls, _ := in0["tls"].(map[string]any)
	tls["certificate_path"] = "/work/self.crt"
	tls["key_path"] = "/work/self.key"
	patched, _ := json.MarshalIndent(cfg, "", "  ")

	dir := mountDir(t, patched)
	genSelfSigned(t, dir)
	out, err := dockerRun(t, "-v", dir+":/work:ro", singBoxImage, "check", "-c", "/work/config.json")
	if err != nil {
		t.Fatalf("sing-box check rejected sync config: %v\n%s", err, out)
	}
	t.Logf("sing-box check output:\n%s", out)
}
