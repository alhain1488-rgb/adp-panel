package protocols

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// RealityKeys is an X25519 keypair for VLESS Reality, encoded the way xray
// expects (base64 raw-url).
type RealityKeys struct {
	PrivateKey string
	PublicKey  string
}

// GenerateRealityKeys creates a fresh X25519 keypair.
func GenerateRealityKeys() (RealityKeys, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return RealityKeys{}, fmt.Errorf("protocols: reality keygen: %w", err)
	}
	return RealityKeys{
		PrivateKey: base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		PublicKey:  base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
	}, nil
}

// GenerateShortID returns a random Reality short id (8 bytes → 16 hex chars).
func GenerateShortID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
