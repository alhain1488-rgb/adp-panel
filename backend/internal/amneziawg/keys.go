// Package amneziawg builds AmneziaWG 2.0 configuration: Curve25519 keys, tunnel
// IP allocation, the server/client WireGuard-format .conf (with the 2.0
// obfuscation params), and the AmneziaVPN vpn:// deep link.
//
// AmneziaWG is a WireGuard fork: standard [Interface]/[Peer] syntax plus extra
// obfuscation keys (Jc/Jmin/Jmax, S1..S4, H1..H4, I1..I5). Those obfuscation
// params are a property of the interface (one AmneziaWG inbound) and MUST be
// byte-identical on the server and every client, so they are generated once per
// inbound and stamped into every peer's config. Keys/addresses are per-peer.
package amneziawg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// GenerateKey returns a fresh Curve25519 private/public keypair, base64-encoded
// in the WireGuard convention (standard base64 of the 32-byte scalar/point).
func GenerateKey() (priv string, pub string, err error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return "", "", err
	}
	// Curve25519 private-key clamping (same as WireGuard).
	k[0] &= 248
	k[31] &= 127
	k[31] |= 64
	priv = base64.StdEncoding.EncodeToString(k[:])
	pub, err = PublicFromPrivate(priv)
	if err != nil {
		return "", "", err
	}
	return priv, pub, nil
}

// PublicFromPrivate derives the base64 public key from a base64 private key.
func PublicFromPrivate(priv string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(priv)
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("amneziawg: invalid private key")
	}
	pub, err := curve25519.X25519(raw, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("amneziawg: derive public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

// GenPSK returns a fresh 32-byte preshared key, base64-encoded (optional; when
// used it must be identical on both peers).
func GenPSK() (string, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k[:]), nil
}
