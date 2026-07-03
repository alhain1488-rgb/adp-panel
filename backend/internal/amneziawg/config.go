package amneziawg

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// StoredInterface is the AmneziaWG interface config as persisted in an inbound's
// settings_json (the listen port comes from the inbound's own port column).
type StoredInterface struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	Subnet     string `json:"subnet"`
	MTU        int    `json:"mtu"`
	DNS        string `json:"dns"`
	NAT        bool   `json:"nat"`
	Params     Params `json:"params"`
}

// ParseInterface reconstructs an Interface from an inbound's settings JSON and
// its listen port.
func ParseInterface(settingsJSON string, listenPort int) (Interface, error) {
	var si StoredInterface
	if err := json.Unmarshal([]byte(settingsJSON), &si); err != nil {
		return Interface{}, fmt.Errorf("amneziawg: parse interface settings: %w", err)
	}
	if si.PrivateKey == "" || si.Subnet == "" {
		return Interface{}, fmt.Errorf("amneziawg: interface missing private_key or subnet")
	}
	if si.PublicKey == "" {
		pub, err := PublicFromPrivate(si.PrivateKey)
		if err != nil {
			return Interface{}, err
		}
		si.PublicKey = pub
	}
	return Interface{
		PrivateKey: si.PrivateKey, PublicKey: si.PublicKey, ListenPort: listenPort,
		Subnet: si.Subnet, MTU: si.MTU, DNS: si.DNS, NAT: si.NAT, Params: si.Params,
	}, nil
}

// Params is the shared AmneziaWG 2.0 obfuscation set for one interface (inbound).
// It MUST be identical on the server and every client on that inbound. H1..H4 are
// strings so a 2.0 range ("100000-800000") or a single value both work. I1..I5
// are the CPS "signature" packets (client side); I1 is the usual one.
type Params struct {
	Jc   int    `json:"jc"`
	Jmin int    `json:"jmin"`
	Jmax int    `json:"jmax"`
	S1   int    `json:"s1"`
	S2   int    `json:"s2"`
	S3   int    `json:"s3"` // new in 2.0 (cookie padding)
	S4   int    `json:"s4"` // new in 2.0 (data padding)
	H1   string `json:"h1"`
	H2   string `json:"h2"`
	H3   string `json:"h3"`
	H4   string `json:"h4"`
	I1   string `json:"i1,omitempty"`
	I2   string `json:"i2,omitempty"`
	I3   string `json:"i3,omitempty"`
	I4   string `json:"i4,omitempty"`
	I5   string `json:"i5,omitempty"`
}

// DefaultParams returns AmneziaWG 2.0's canonical default obfuscation set, taken
// verbatim from AmneziaVPN's own defaults (amnezia-client protocols_defs, tag
// 4.8.19.0): Jc/Jmin/Jmax, S1..S4, the four magic headers H1..H4, and the I1
// "special junk" template — a crafted packet that mimics an iCloud DNS lookup.
// Using AmneziaVPN's exact values maximizes interop with the AmneziaVPN app. The
// set MUST stay identical on the server and every client of an inbound.
func DefaultParams() Params {
	return Params{
		Jc: 3, Jmin: 10, Jmax: 30,
		S1: 15, S2: 18, S3: 20, S4: 23,
		// H3 = underload header, H4 = transport header (Amnezia's field order).
		H1: "1020325451", H2: "3288052141", H3: "1766607858", H4: "2528465083",
		I1: "<r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>",
	}
}

// Interface is one AmneziaWG inbound: the server side of a WireGuard interface
// plus the shared 2.0 obfuscation params.
type Interface struct {
	PrivateKey string // server private key (base64)
	PublicKey  string // server public key (base64)
	ListenPort int
	Subnet     string // tunnel subnet CIDR, e.g. "10.9.9.0/24"
	MTU        int    // client MTU, e.g. 1280
	DNS        string // client DNS, e.g. "1.1.1.1"
	NAT        bool   // emit PostUp/PostDown MASQUERADE on the server
	Params     Params
}

// Peer is one client on an interface.
type Peer struct {
	Name         string
	PrivateKey   string // client private key (base64) — only needed for the client .conf
	PublicKey    string // client public key (base64) — used in the server [Peer]
	Address      string // client tunnel address, e.g. "10.9.9.2/32"
	PresharedKey string // optional; identical on both ends when set
}

// HostIP returns the index-th host address of subnet (index 1 == the .1 host).
// Used to lay the server at .1 and clients at .2, .3, …
func HostIP(subnet string, index int) (string, error) {
	ip, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("amneziawg: subnet: %w", err)
	}
	base := ipnet.IP.To4()
	if base == nil {
		return "", fmt.Errorf("amneziawg: only IPv4 subnets are supported")
	}
	_ = ip
	v := binary.BigEndian.Uint32(base) + uint32(index) //nolint:gosec // index is small and bounded by the caller
	var out [4]byte
	binary.BigEndian.PutUint32(out[:], v)
	got := net.IP(out[:])
	if !ipnet.Contains(got) {
		return "", fmt.Errorf("amneziawg: host index %d is outside %s", index, subnet)
	}
	return got.String(), nil
}

type lines struct{ b strings.Builder }

func (l *lines) add(k, v string) {
	if v == "" {
		return
	}
	l.b.WriteString(k)
	l.b.WriteString(" = ")
	l.b.WriteString(v)
	l.b.WriteByte('\n')
}
func (l *lines) addInt(k string, v int) { l.add(k, fmt.Sprintf("%d", v)) }
func (l *lines) raw(s string)           { l.b.WriteString(s); l.b.WriteByte('\n') }
func (l *lines) String() string         { return l.b.String() }

// writeObfuscation appends the shared params. Jc/Jmin/Jmax/S1..S4 are always
// written (0 = off); H1..H4 are written when set. I1..I5 are client-only.
func writeObfuscation(l *lines, p Params, withCPS bool) {
	l.addInt("Jc", p.Jc)
	l.addInt("Jmin", p.Jmin)
	l.addInt("Jmax", p.Jmax)
	l.addInt("S1", p.S1)
	l.addInt("S2", p.S2)
	l.addInt("S3", p.S3)
	l.addInt("S4", p.S4)
	l.add("H1", p.H1)
	l.add("H2", p.H2)
	l.add("H3", p.H3)
	l.add("H4", p.H4)
	if withCPS {
		l.add("I1", p.I1)
		l.add("I2", p.I2)
		l.add("I3", p.I3)
		l.add("I4", p.I4)
		l.add("I5", p.I5)
	}
}

// natCommands returns PostUp/PostDown lines that forward and MASQUERADE client
// traffic out the node's default-route interface (detected at up/down time).
func natCommands() (up, down string) {
	egress := `$(ip route list default | awk '{print $5; exit}')`
	up = "iptables -I FORWARD -i %i -j ACCEPT; iptables -I FORWARD -o %i -j ACCEPT; " +
		"iptables -t nat -A POSTROUTING -o " + egress + " -j MASQUERADE"
	down = "iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; " +
		"iptables -t nat -D POSTROUTING -o " + egress + " -j MASQUERADE"
	return up, down
}

// ServerConfig renders the node-side awg .conf: one [Interface] plus a [Peer]
// per granted client.
func ServerConfig(iface Interface, peers []Peer) (string, error) {
	serverIP, err := HostIP(iface.Subnet, 1)
	if err != nil {
		return "", err
	}
	_, ipnet, err := net.ParseCIDR(iface.Subnet)
	if err != nil {
		return "", err
	}
	maskLen, _ := ipnet.Mask.Size()

	var l lines
	l.raw("[Interface]")
	l.add("PrivateKey", iface.PrivateKey)
	l.add("Address", fmt.Sprintf("%s/%d", serverIP, maskLen))
	l.addInt("ListenPort", iface.ListenPort)
	writeObfuscation(&l, iface.Params, false)
	if iface.NAT {
		up, down := natCommands()
		l.add("PostUp", up)
		l.add("PostDown", down)
	}
	for _, pr := range peers {
		l.raw("")
		l.raw("[Peer]")
		if pr.Name != "" {
			l.raw("# " + pr.Name)
		}
		l.add("PublicKey", pr.PublicKey)
		l.add("PresharedKey", pr.PresharedKey)
		l.add("AllowedIPs", pr.Address)
	}
	return l.String(), nil
}

// ClientConfig renders one client's .conf for the AmneziaWG app. endpoint is
// "host:port".
func ClientConfig(iface Interface, peer Peer, endpoint string) string {
	mtu := iface.MTU
	if mtu == 0 {
		mtu = 1280
	}
	var l lines
	l.raw("[Interface]")
	l.add("PrivateKey", peer.PrivateKey)
	l.add("Address", peer.Address)
	l.add("DNS", iface.DNS)
	l.addInt("MTU", mtu)
	writeObfuscation(&l, iface.Params, true)
	l.raw("")
	l.raw("[Peer]")
	l.add("PublicKey", iface.PublicKey)
	l.add("PresharedKey", peer.PresharedKey)
	l.add("Endpoint", endpoint)
	l.add("AllowedIPs", "0.0.0.0/0")
	l.add("PersistentKeepalive", "25")
	return l.String()
}
