package protocols

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

var testSrv = Server{Host: "example.com"}

func testClient() Client {
	return Client{
		Name:     "alice",
		UUID:     "11111111-2222-3333-4444-555555555555",
		Password: "s3cr3t-pass",
	}
}

func mustGet(t *testing.T, name string) Protocol {
	t.Helper()
	p, ok := Get(name)
	if !ok {
		t.Fatalf("Get(%q) returned no adapter", name)
	}
	return p
}

func TestEngines(t *testing.T) {
	cases := map[string]Engine{
		"vless":       EngineXray,
		"vmess":       EngineXray,
		"trojan":      EngineXray,
		"shadowsocks": EngineXray,
		"hysteria2":   EngineHysteria,
	}
	for name, want := range cases {
		p := mustGet(t, name)
		if got := p.Engine(); got != want {
			t.Errorf("%s: Engine()=%q, want %q", name, got, want)
		}
		if p.Name() != name {
			t.Errorf("%s: Name()=%q, want %q", name, p.Name(), name)
		}
	}
}

func realityInbound() Inbound {
	return Inbound{
		Tag:      "in-vless",
		Protocol: "vless",
		Port:     443,
		Remark:   "vless-reality",
		Settings: map[string]any{"flow": "xtls-rprx-vision"},
		StreamSettings: map[string]any{
			"network":  "tcp",
			"security": "reality",
			"realitySettings": map[string]any{
				"dest":        "www.yahoo.com:443",
				"serverNames": []any{"www.yahoo.com"},
				"privateKey":  "PRIV",
				"publicKey":   "PUB",
				"shortIds":    []any{"abcd1234"},
				"fingerprint": "chrome",
			},
		},
	}
}

func TestVLESS(t *testing.T) {
	p := mustGet(t, "vless")
	c := testClient()
	in := realityInbound()

	raw, err := p.BuildInbound(in, []Client{c})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("BuildInbound produced invalid JSON: %s", raw)
	}
	var frag map[string]any
	if err := json.Unmarshal(raw, &frag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	settings := frag["settings"].(map[string]any)
	clients := settings["clients"].([]any)
	if len(clients) != 1 {
		t.Fatalf("want 1 client, got %d", len(clients))
	}
	if id := clients[0].(map[string]any)["id"]; id != c.UUID {
		t.Errorf("client id=%v, want %v", id, c.UUID)
	}

	link, err := p.BuildLink(testSrv, in, c)
	if err != nil {
		t.Fatalf("BuildLink: %v", err)
	}
	if !strings.HasPrefix(link, "vless://") {
		t.Errorf("link missing vless:// prefix: %s", link)
	}
	if !strings.Contains(link, c.UUID) {
		t.Errorf("link missing uuid: %s", link)
	}
	if !strings.Contains(link, "security=reality") {
		t.Errorf("link missing security=reality: %s", link)
	}
}

func TestVMESS(t *testing.T) {
	p := mustGet(t, "vmess")
	c := testClient()
	in := Inbound{
		Tag:      "in-vmess",
		Protocol: "vmess",
		Port:     8443,
		Remark:   "vmess-ws",
		Settings: map[string]any{"security": "auto"},
		StreamSettings: map[string]any{
			"network":  "ws",
			"security": "tls",
			"wsSettings": map[string]any{
				"path": "/vm",
				"host": "cdn.example.com",
			},
			"tlsSettings": map[string]any{"serverName": "cdn.example.com"},
		},
	}

	raw, err := p.BuildInbound(in, []Client{c})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid JSON: %s", raw)
	}
	var frag map[string]any
	if err := json.Unmarshal(raw, &frag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	settings := frag["settings"].(map[string]any)
	clients := settings["clients"].([]any)
	if id := clients[0].(map[string]any)["id"]; id != c.UUID {
		t.Errorf("client id=%v, want %v", id, c.UUID)
	}

	link, err := p.BuildLink(testSrv, in, c)
	if err != nil {
		t.Fatalf("BuildLink: %v", err)
	}
	if !strings.HasPrefix(link, "vmess://") {
		t.Fatalf("link missing vmess:// prefix: %s", link)
	}
	blob := strings.TrimPrefix(link, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(decoded, &obj); err != nil {
		t.Fatalf("decoded blob not JSON: %v", err)
	}
	if obj["id"] != c.UUID {
		t.Errorf("vmess id=%v, want %v", obj["id"], c.UUID)
	}
	if obj["net"] != "ws" {
		t.Errorf("vmess net=%v, want ws", obj["net"])
	}
	if obj["tls"] != "tls" {
		t.Errorf("vmess tls=%v, want tls", obj["tls"])
	}
}

func TestTrojan(t *testing.T) {
	p := mustGet(t, "trojan")
	c := testClient()
	in := Inbound{
		Tag:      "in-trojan",
		Protocol: "trojan",
		Port:     443,
		Remark:   "trojan-tls",
		Settings: map[string]any{},
		StreamSettings: map[string]any{
			"network":     "tcp",
			"security":    "tls",
			"tlsSettings": map[string]any{"serverName": "example.com"},
		},
	}

	raw, err := p.BuildInbound(in, []Client{c})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid JSON: %s", raw)
	}
	var frag map[string]any
	if err := json.Unmarshal(raw, &frag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	settings := frag["settings"].(map[string]any)
	clients := settings["clients"].([]any)
	if pw := clients[0].(map[string]any)["password"]; pw != c.Password {
		t.Errorf("client password=%v, want %v", pw, c.Password)
	}

	link, err := p.BuildLink(testSrv, in, c)
	if err != nil {
		t.Fatalf("BuildLink: %v", err)
	}
	if !strings.HasPrefix(link, "trojan://") {
		t.Errorf("link missing trojan:// prefix: %s", link)
	}
	if !strings.Contains(link, c.Password) {
		t.Errorf("link missing password: %s", link)
	}
}

func TestShadowsocks(t *testing.T) {
	p := mustGet(t, "shadowsocks")
	c := testClient()
	in := Inbound{
		Tag:      "in-ss",
		Protocol: "shadowsocks",
		Port:     8388,
		Remark:   "ss",
		Settings: map[string]any{"method": "aes-256-gcm", "network": "tcp,udp"},
	}

	raw, err := p.BuildInbound(in, []Client{c})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid JSON: %s", raw)
	}
	var frag map[string]any
	if err := json.Unmarshal(raw, &frag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	settings := frag["settings"].(map[string]any)
	if settings["method"] != "aes-256-gcm" {
		t.Errorf("method=%v, want aes-256-gcm", settings["method"])
	}
	if settings["password"] != c.Password {
		t.Errorf("password=%v, want %v", settings["password"], c.Password)
	}

	link, err := p.BuildLink(testSrv, in, c)
	if err != nil {
		t.Fatalf("BuildLink: %v", err)
	}
	if !strings.HasPrefix(link, "ss://") {
		t.Errorf("link missing ss:// prefix: %s", link)
	}
}

func TestHysteria2(t *testing.T) {
	p := mustGet(t, "hysteria2")
	c := testClient()
	in := Inbound{
		Tag:      "in-hy2",
		Protocol: "hysteria2",
		Port:     443,
		Remark:   "hy2",
		Settings: map[string]any{
			"up":   "100 mbps",
			"down": float64(200),
			"obfs": map[string]any{"type": "salamander", "password": "obfspw"},
		},
		StreamSettings: map[string]any{
			"tlsSettings": map[string]any{"serverName": "hy.example.com", "insecure": true},
		},
	}

	raw, err := p.BuildInbound(in, []Client{c})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid JSON: %s", raw)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["type"] != "hysteria2" {
		t.Errorf("type=%v, want hysteria2", obj["type"])
	}
	users := obj["users"].([]any)
	if pw := users[0].(map[string]any)["password"]; pw != c.Password {
		t.Errorf("user password=%v, want %v", pw, c.Password)
	}
	if obj["up_mbps"].(float64) != 100 {
		t.Errorf("up_mbps=%v, want 100", obj["up_mbps"])
	}
	if obj["down_mbps"].(float64) != 200 {
		t.Errorf("down_mbps=%v, want 200", obj["down_mbps"])
	}
	if obfs, ok := obj["obfs"].(map[string]any); !ok || obfs["type"] != "salamander" {
		t.Errorf("obfs not set to salamander: %v", obj["obfs"])
	}

	link, err := p.BuildLink(testSrv, in, c)
	if err != nil {
		t.Fatalf("BuildLink: %v", err)
	}
	if !strings.HasPrefix(link, "hysteria2://") {
		t.Errorf("link missing hysteria2:// prefix: %s", link)
	}
	if !strings.Contains(link, c.Password) {
		t.Errorf("link missing password: %s", link)
	}
	if !strings.Contains(link, "obfs=salamander") {
		t.Errorf("link missing obfs: %s", link)
	}
}

func TestParseMbps(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{"100 mbps", 100},
		{float64(200), 200},
		{int(50), 50},
		{"", 0},
		{"abc", 0},
		{nil, 0},
	}
	for _, tc := range cases {
		if got := parseMbps(tc.in); got != tc.want {
			t.Errorf("parseMbps(%v)=%d, want %d", tc.in, got, tc.want)
		}
	}
}
