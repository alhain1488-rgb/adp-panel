//go:build integration

// Integration checks that adapter-generated config is accepted by the real
// engines. Requires Docker (Colima) and network. Run with:
//
//	go test -tags=integration -run Integration ./internal/protocols/ -v
package protocols

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	xrayImage    = "ghcr.io/xtls/xray-core:latest"
	singBoxImage = "ghcr.io/sagernet/sing-box:latest"
)

// writeConfig writes cfg to a fresh dir (under the package dir so Colima mounts
// it) and returns the absolute dir path.
func writeConfig(t *testing.T, cfg map[string]any) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "validate-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(abs, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

func writeConfigInto(t *testing.T, dir string, cfg map[string]any) {
	t.Helper()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
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

func dockerRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("docker", append([]string{"run", "--rm"}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func intClient() Client {
	return Client{Name: "tester", UUID: "b3f1a9c2-4d5e-4f7a-8b1c-2e3d4f5a6b7c", Password: "Xk92Lm4Qp7Zr"}
}

func realityStream(t *testing.T) map[string]any {
	t.Helper()
	keys, err := GenerateRealityKeys()
	if err != nil {
		t.Fatal(err)
	}
	sid, _ := GenerateShortID()
	return map[string]any{
		"network": "tcp", "security": "reality",
		"realitySettings": map[string]any{
			"dest": "www.yahoo.com:443", "serverNames": []any{"www.yahoo.com"},
			"privateKey": keys.PrivateKey, "publicKey": keys.PublicKey,
			"shortIds": []any{sid}, "fingerprint": "chrome",
		},
	}
}

func frag(t *testing.T, in Inbound) map[string]any {
	t.Helper()
	p, ok := Get(in.Protocol)
	if !ok {
		t.Fatalf("no adapter for %s", in.Protocol)
	}
	raw, err := p.BuildInbound(in, []Client{intClient()})
	if err != nil {
		t.Fatalf("BuildInbound %s: %v", in.Protocol, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("fragment %s not JSON: %v", in.Protocol, err)
	}
	return m
}

// TestIntegration_XrayConfigAccepted builds VLESS Reality + VMess + Trojan and
// asserts `xray -test` accepts the assembled config.
func TestIntegration_XrayConfigAccepted(t *testing.T) {
	inbounds := []map[string]any{
		frag(t, Inbound{Tag: "vless-reality", Protocol: "vless", Port: 443,
			Settings: map[string]any{"flow": "xtls-rprx-vision"}, StreamSettings: realityStream(t)}),
		frag(t, Inbound{Tag: "vmess-tcp", Protocol: "vmess", Port: 10001,
			StreamSettings: map[string]any{"network": "tcp", "security": "none"}}),
		frag(t, Inbound{Tag: "trojan-tcp", Protocol: "trojan", Port: 10002,
			StreamSettings: map[string]any{"network": "tcp", "security": "none"}}),
	}
	cfg := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  inbounds,
		"outbounds": []map[string]any{{"protocol": "freedom"}},
	}
	dir := writeConfig(t, cfg)
	out, err := dockerRun(t, "-v", dir+":/work:ro", xrayImage, "-test", "-config", "/work/config.json")
	if err != nil {
		t.Fatalf("xray -test rejected config: %v\n%s", err, out)
	}
	t.Logf("xray -test output:\n%s", out)
}

// TestIntegration_SingBoxConfigAccepted builds a Hysteria2 inbound and asserts
// `sing-box check` accepts the assembled config.
func TestIntegration_SingBoxConfigAccepted(t *testing.T) {
	hy := frag(t, Inbound{Tag: "hy2", Protocol: "hysteria2", Port: 36712,
		Settings:       map[string]any{"up": "100 mbps", "down": "200 mbps"},
		StreamSettings: map[string]any{"security": "tls", "tlsSettings": map[string]any{"serverName": "node.example.com", "insecure": true}},
	})
	cfg := map[string]any{
		"log":       map[string]any{"level": "warn"},
		"inbounds":  []map[string]any{hy},
		"outbounds": []map[string]any{{"type": "direct"}},
	}
	dir := writeConfig(t, cfg)
	// sing-box check actually reads the TLS cert files — generate a real
	// self-signed cert in the mounted dir and point the config at it.
	genSelfSigned(t, dir)
	tls := hy["tls"].(map[string]any)
	tls["certificate_path"] = "/work/self.crt"
	tls["key_path"] = "/work/self.key"
	writeConfigInto(t, dir, cfg)
	out, err := dockerRun(t, "-v", dir+":/work:ro", singBoxImage, "check", "-c", "/work/config.json")
	if err != nil {
		t.Fatalf("sing-box check rejected config: %v\n%s", err, out)
	}
	t.Logf("sing-box check output:\n%s", out)
}
