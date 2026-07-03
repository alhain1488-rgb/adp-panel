package amneziawg

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// vpn:// deep link for the AmneziaVPN app. Layout (per Amnezia's amnezia-client
// import path): "vpn://" + base64url( qCompress( containerJSON ) ), where Qt's
// qCompress = 4-byte big-endian uncompressed-size header followed by a zlib
// stream. The inner container carries a stringified "last_config".
//
// NOTE: the last_config field NAMES below are synthesized from Amnezia docs and
// community reverse-engineering; the exact spellings (esp. S3/S4 and H2/H4 and
// the base64 padding) should be verified against a real AmneziaVPN 2.0 export
// before this is relied on in production. The codec (qCompress) IS well-defined
// and round-trips in tests.

// qCompress mimics Qt's qCompress: 4-byte big-endian original length + zlib.
func qCompress(data []byte) []byte {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	buf.Write(hdr[:])
	zw := zlib.NewWriter(&buf)
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

// awgLastConfig is the inner "last_config" object (stringified inside the
// container). Field names are the verify-me set (see file note).
type awgLastConfig struct {
	Config                       string `json:"config"` // the raw client .conf text
	Hostname                     string `json:"hostName"`
	Port                         int    `json:"port"`
	ClientPrivKey                string `json:"client_priv_key"`
	ClientPubKey                 string `json:"client_pub_key"`
	ServerPubKey                 string `json:"server_pub_key"`
	PSK                          string `json:"psk_key,omitempty"`
	Address                      string `json:"client_ip"`
	JunkPacketCount              int    `json:"junkPacketCount"`
	JunkPacketMinSize            int    `json:"junkPacketMinSize"`
	JunkPacketMaxSize            int    `json:"junkPacketMaxSize"`
	InitPacketJunkSize           int    `json:"initPacketJunkSize"`
	ResponsePacketJunkSize       int    `json:"responsePacketJunkSize"`
	CookieReplyPacketJunkSize    int    `json:"cookieReplyPacketJunkSize"`
	TransportPacketJunkSize      int    `json:"transportPacketJunkSize"`
	InitPacketMagicHeader        string `json:"initPacketMagicHeader"`
	ResponsePacketMagicHeader    string `json:"responsePacketMagicHeader"`
	CookieReplyPacketMagicHeader string `json:"cookieReplyPacketMagicHeader"`
	TransportPacketMagicHeader   string `json:"transportPacketMagicHeader"`
	SpecialJunk1                 string `json:"specialJunk1,omitempty"`
	SpecialJunk2                 string `json:"specialJunk2,omitempty"`
	SpecialJunk3                 string `json:"specialJunk3,omitempty"`
	SpecialJunk4                 string `json:"specialJunk4,omitempty"`
	SpecialJunk5                 string `json:"specialJunk5,omitempty"`
	IsObfuscationEnabled         bool   `json:"isObfuscationEnabled"`
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

	last := awgLastConfig{
		Config:                       clientConf,
		Hostname:                     in.Host,
		Port:                         in.Iface.ListenPort,
		ClientPrivKey:                in.Peer.PrivateKey,
		ClientPubKey:                 in.Peer.PublicKey,
		ServerPubKey:                 in.Iface.PublicKey,
		PSK:                          in.Peer.PresharedKey,
		Address:                      in.Peer.Address,
		JunkPacketCount:              p.Jc,
		JunkPacketMinSize:            p.Jmin,
		JunkPacketMaxSize:            p.Jmax,
		InitPacketJunkSize:           p.S1,
		ResponsePacketJunkSize:       p.S2,
		CookieReplyPacketJunkSize:    p.S3,
		TransportPacketJunkSize:      p.S4,
		InitPacketMagicHeader:        p.H1,
		ResponsePacketMagicHeader:    p.H2,
		CookieReplyPacketMagicHeader: p.H3,
		TransportPacketMagicHeader:   p.H4,
		SpecialJunk1:                 p.I1,
		SpecialJunk2:                 p.I2,
		SpecialJunk3:                 p.I3,
		SpecialJunk4:                 p.I4,
		SpecialJunk5:                 p.I5,
		IsObfuscationEnabled:         true,
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
	container := map[string]any{
		"hostName":         in.Host,
		"port":             in.Iface.ListenPort,
		"description":      in.Description,
		"defaultContainer": "amnezia-awg",
		"dns1":             dns1,
		"dns2":             dns2,
		"containers": []map[string]any{{
			"container": "amnezia-awg",
			"awg": map[string]any{
				"last_config":     string(lastBytes),
				"port":            fmt.Sprintf("%d", in.Iface.ListenPort),
				"transport_proto": "udp",
			},
		}},
	}
	raw, err := json.Marshal(container)
	if err != nil {
		return "", err
	}
	return "vpn://" + base64.URLEncoding.EncodeToString(qCompress(raw)), nil
}
