package amneziawg

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateKeyRoundTrip(t *testing.T) {
	priv, pub, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(priv)
	if err != nil || len(raw) != 32 {
		t.Fatalf("private key not 32 bytes: %v", err)
	}
	// Public key is deterministic from the private key.
	pub2, err := PublicFromPrivate(priv)
	if err != nil || pub2 != pub {
		t.Fatalf("public not deterministic: %v (%q != %q)", err, pub2, pub)
	}
	// Two keypairs differ.
	priv2, _, _ := GenerateKey()
	if priv2 == priv {
		t.Fatal("two generated keys are identical")
	}
}

func TestHostIP(t *testing.T) {
	cases := map[int]string{1: "10.9.9.1", 2: "10.9.9.2", 254: "10.9.9.254"}
	for idx, want := range cases {
		got, err := HostIP("10.9.9.0/24", idx)
		if err != nil || got != want {
			t.Fatalf("HostIP(%d) = %q, %v; want %q", idx, got, err, want)
		}
	}
	if _, err := HostIP("10.9.9.0/24", 999); err == nil {
		t.Error("expected out-of-subnet error for index 999")
	}
}

func testIface(t *testing.T) Interface {
	t.Helper()
	priv, pub, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return Interface{
		PrivateKey: priv, PublicKey: pub, ListenPort: 51820,
		Subnet: "10.9.9.0/24", MTU: 1280, DNS: "1.1.1.1", NAT: true,
		Params: DefaultParams(),
	}
}

func TestServerAndClientConfigMatchObfuscation(t *testing.T) {
	iface := testIface(t)
	cpriv, cpub, _ := GenerateKey()
	addr, _ := HostIP(iface.Subnet, 2)
	peer := Peer{Name: "phone", PrivateKey: cpriv, PublicKey: cpub, Address: addr + "/32"}

	srv, err := ServerConfig(iface, []Peer{peer})
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	cli := ClientConfig(iface, peer, "203.0.113.7:51820")

	// Server side.
	for _, want := range []string{"[Interface]", "Address = 10.9.9.1/24", "ListenPort = 51820",
		"Jc = 3", "H1 = 1020325451", "PostUp", "[Peer]", "# phone",
		"PublicKey = " + cpub, "AllowedIPs = 10.9.9.2/32"} {
		if !strings.Contains(srv, want) {
			t.Errorf("server config missing %q", want)
		}
	}
	// The server must NOT carry the client-only CPS packet (I1..I5).
	if strings.Contains(srv, "I1 =") {
		t.Error("server config should not include I1 (client-only CPS)")
	}

	// Client side (I1 is AmneziaWG 2.0's fake-DNS special-junk template).
	for _, want := range []string{"Address = 10.9.9.2/32", "DNS = 1.1.1.1", "MTU = 1280",
		"I1 = <r 2><b 0x8580", "PublicKey = " + iface.PublicKey, "Endpoint = 203.0.113.7:51820",
		"AllowedIPs = 0.0.0.0/0", "PersistentKeepalive = 25"} {
		if !strings.Contains(cli, want) {
			t.Errorf("client config missing %q", want)
		}
	}

	// The obfuscation set must be byte-identical on both ends (handshake contract).
	for _, want := range []string{"Jc = 3", "Jmin = 10", "Jmax = 30",
		"S1 = 15", "S2 = 18", "S3 = 20", "S4 = 23", "H1 = 1020325451", "H4 = 2528465083"} {
		if !strings.Contains(srv, want) || !strings.Contains(cli, want) {
			t.Errorf("obfuscation param %q not identical on both sides", want)
		}
	}
}

func TestQCompressRoundTrip(t *testing.T) {
	orig := []byte(`{"hello":"мир","n":42}`)
	packed := qCompress(orig)
	// 4-byte big-endian length header.
	if len(packed) < 4 || int(packed[3]) != len(orig) {
		t.Fatalf("bad length header: %v", packed[:4])
	}
	got, err := qUncompress(packed)
	if err != nil {
		t.Fatalf("uncompress: %v", err)
	}
	if string(got) != string(orig) {
		t.Fatalf("round-trip mismatch: %q", got)
	}
}

func TestVpnLinkDecodable(t *testing.T) {
	iface := testIface(t)
	cpriv, cpub, _ := GenerateKey()
	addr, _ := HostIP(iface.Subnet, 2)
	peer := Peer{PrivateKey: cpriv, PublicKey: cpub, Address: addr + "/32"}

	link, err := VpnLink(VpnLinkInput{Iface: iface, Peer: peer, Host: "203.0.113.7", Description: "My phone"})
	if err != nil {
		t.Fatalf("vpn link: %v", err)
	}
	if !strings.HasPrefix(link, "vpn://") {
		t.Fatalf("missing vpn:// prefix: %q", link[:16])
	}

	// Decode the transport back to the container and assert structure. The link
	// is base64url with padding stripped, so decode with RawURLEncoding.
	blob, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "vpn://"))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	raw, err := qUncompress(blob)
	if err != nil {
		t.Fatalf("qUncompress: %v", err)
	}
	var container struct {
		HostName         string `json:"hostName"`
		DefaultContainer string `json:"defaultContainer"`
		Containers       []struct {
			Container string `json:"container"`
			AWG       struct {
				LastConfig string `json:"last_config"`
			} `json:"awg"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(raw, &container); err != nil {
		t.Fatalf("container json: %v", err)
	}
	if container.HostName != "203.0.113.7" || container.DefaultContainer != "amnezia-awg2" {
		t.Fatalf("unexpected container: %+v", container)
	}
	if len(container.Containers) != 1 || container.Containers[0].Container != "amnezia-awg2" {
		t.Fatalf("want 1 amnezia-awg2 container, got %+v", container.Containers)
	}
	// Obfuscation params use SHORT keys with STRING values inside last_config.
	var last struct {
		Jc                   string `json:"Jc"`
		H1                   string `json:"H1"`
		IsObfuscationEnabled bool   `json:"isObfuscationEnabled"`
		Config               string `json:"config"`
	}
	if err := json.Unmarshal([]byte(container.Containers[0].AWG.LastConfig), &last); err != nil {
		t.Fatalf("last_config json: %v", err)
	}
	if last.Jc != strconv.Itoa(iface.Params.Jc) || last.H1 != iface.Params.H1 {
		t.Fatalf("last_config params mismatch: %+v", last)
	}
	if !last.IsObfuscationEnabled || !strings.Contains(last.Config, "PrivateKey = "+cpriv) {
		t.Fatal("last_config not wired to the client conf")
	}
}
