package amneziawg

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// vpn:// deep link for the AmneziaVPN app. Layout (per amnezia-client's export
// path, exportController.cpp @ tag 4.8.19.0):
//
//	"vpn://" + base64url_nopad( qCompress( containerJSON ) )
//
// where qCompress = Qt's qCompress(data, 8) = a 4-byte big-endian
// uncompressed-size header followed by a zlib stream, base64 is URL-safe with
// trailing '=' omitted, and the inner container carries a stringified
// "last_config". The schema below is verified field-by-field against the
// amnezia-client source (see comments); on import, ImportController accepts the
// config as soon as it contains a "containers" array (importController.cpp:322),
// so config_version and the ssh credentials are deliberately omitted.

// qCompress mimics Qt's qCompress(data, 8): 4-byte big-endian original length +
// a zlib stream at level 8 (level only affects size, not decodability).
func qCompress(data []byte) []byte {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	buf.Write(hdr[:])
	zw, _ := zlib.NewWriterLevel(&buf, 8)
	_, _ = zw.Write(data)
	_ = zw.Close()
	return buf.Bytes()
}

// qUncompress reverses qCompress (used by tests / import).
func qUncompress(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("amneziawg: qCompress data too short")
	}
	zr, err := zlib.NewReader(bytes.NewReader(data[4:]))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return io.ReadAll(zr)
}

// awgLastConfig is the inner "last_config" object (stringified inside the awg
// container). Field names and value types are verified against amnezia-client:
// base fields from wireguard_configurator.cpp:206-224, obfuscation short keys
// from awg_configurator.cpp:34-53 + protocols_defs.h:70-85. Every obfuscation
// value is a JSON STRING (they originate from a QMap<QString,QString>); only
// isObfuscationEnabled is a real bool, port is an int, and allowed_ips is an
// array. The obfuscation params live INSIDE this object, not as siblings of it.
type awgLastConfig struct {
	Config              string   `json:"config"` // the raw client .conf text
	Hostname            string   `json:"hostName"`
	Port                int      `json:"port"`
	ClientPrivKey       string   `json:"client_priv_key"`
	ClientIP            string   `json:"client_ip"`
	ClientPubKey        string   `json:"client_pub_key"`
	PSK                 string   `json:"psk_key"`
	ServerPubKey        string   `json:"server_pub_key"`
	MTU                 string   `json:"mtu"`
	PersistentKeepAlive string   `json:"persistent_keep_alive"`
	AllowedIPs          []string `json:"allowed_ips"`
	ClientID            string   `json:"clientId"`

	// AmneziaWG 2.0 obfuscation set — SHORT keys, string values.
	Jc   string `json:"Jc"`
	Jmin string `json:"Jmin"`
	Jmax string `json:"Jmax"`
	S1   string `json:"S1"`
	S2   string `json:"S2"`
	S3   string `json:"S3"` // 2.0
	S4   string `json:"S4"` // 2.0
	H1   string `json:"H1"`
	H2   string `json:"H2"`
	H3   string `json:"H3"`
	H4   string `json:"H4"`
	I1   string `json:"I1"`
	I2   string `json:"I2"`
	I3   string `json:"I3"`
	I4   string `json:"I4"`
	I5   string `json:"I5"`

	// Present in Amnezia's WG→AWG import path; harmless-and-safer to set true so
	// obfuscation is never silently treated as disabled.
	IsObfuscationEnabled bool `json:"isObfuscationEnabled"`
}

// VpnLinkInput bundles what a vpn:// needs beyond the Interface/Peer.
type VpnLinkInput struct {
	Iface       Interface
	Peer        Peer
	Host        string // public host/IP for the client Endpoint
	Description string // display name shown in the app
	DNS2        string
}

// VpnLink builds the AmneziaVPN vpn:// deep link for one client.
func VpnLink(in VpnLinkInput) (string, error) {
	endpoint := fmt.Sprintf("%s:%d", in.Host, in.Iface.ListenPort)
	clientConf := ClientConfig(in.Iface, in.Peer, endpoint)
	p := in.Iface.Params

	mtu := in.Iface.MTU
	if mtu == 0 {
		mtu = 1280
	}
	// client_ip is the bare tunnel address (no CIDR mask).
	clientIP := in.Peer.Address
	if i := strings.IndexByte(clientIP, '/'); i >= 0 {
		clientIP = clientIP[:i]
	}

	last := awgLastConfig{
		Config:               clientConf,
		Hostname:             in.Host,
		Port:                 in.Iface.ListenPort,
		ClientPrivKey:        in.Peer.PrivateKey,
		ClientIP:             clientIP,
		ClientPubKey:         in.Peer.PublicKey,
		PSK:                  in.Peer.PresharedKey,
		ServerPubKey:         in.Iface.PublicKey,
		MTU:                  strconv.Itoa(mtu),
		PersistentKeepAlive:  "25",
		AllowedIPs:           []string{"0.0.0.0/0", "::/0"},
		ClientID:             in.Peer.PublicKey,
		Jc:                   strconv.Itoa(p.Jc),
		Jmin:                 strconv.Itoa(p.Jmin),
		Jmax:                 strconv.Itoa(p.Jmax),
		S1:                   strconv.Itoa(p.S1),
		S2:                   strconv.Itoa(p.S2),
		S3:                   strconv.Itoa(p.S3),
		S4:                   strconv.Itoa(p.S4),
		H1:                   p.H1,
		H2:                   p.H2,
		H3:                   p.H3,
		H4:                   p.H4,
		I1:                   p.I1,
		I2:                   p.I2,
		I3:                   p.I3,
		I4:                   p.I4,
		I5:                   p.I5,
		IsObfuscationEnabled: true,
	}
	lastBytes, err := json.Marshal(last)
	if err != nil {
		return "", err
	}

	dns1 := in.Iface.DNS
	if dns1 == "" {
		dns1 = "1.1.1.1"
	}
	dns2 := in.DNS2
	if dns2 == "" {
		dns2 = "1.0.0.1"
	}
	// "amnezia-awg2" is the AWG 2.0 container id (amnezia-awg is 1.x); both use
	// the "awg" protocol sub-object. userName/password/port are intentionally
	// absent at the top level (exportController.cpp:82-84 strips them).
	container := map[string]any{
		"containers": []map[string]any{{
			"container": "amnezia-awg2",
			"awg": map[string]any{
				"last_config":     string(lastBytes),
				"port":            fmt.Sprintf("%d", in.Iface.ListenPort),
				"transport_proto": "udp",
			},
		}},
		"defaultContainer": "amnezia-awg2",
		"description":      in.Description,
		"dns1":             dns1,
		"dns2":             dns2,
		"hostName":         in.Host,
	}
	raw, err := json.Marshal(container)
	if err != nil {
		return "", err
	}
	return "vpn://" + base64.RawURLEncoding.EncodeToString(qCompress(raw)), nil
}
