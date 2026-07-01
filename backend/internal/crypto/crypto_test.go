package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestCipher_RoundTrip(t *testing.T) {
	c, err := NewCipher(key(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"", "hunter2", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n", "юникод 🔒"} {
		enc, err := c.Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		if enc == plain && plain != "" {
			t.Errorf("ciphertext equals plaintext for %q", plain)
		}
		dec, err := c.Decrypt(enc)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if dec != plain {
			t.Errorf("round trip = %q, want %q", dec, plain)
		}
	}
}

func TestCipher_NonceRandomized(t *testing.T) {
	c, _ := NewCipher(key(t))
	a, _ := c.Encrypt("same")
	b, _ := c.Encrypt("same")
	if a == b {
		t.Error("expected different ciphertexts for identical plaintext (random nonce)")
	}
}

func TestCipher_WrongKeyFails(t *testing.T) {
	c1, _ := NewCipher(key(t))
	c2, _ := NewCipher(key(t))
	enc, _ := c1.Encrypt("secret")
	if _, err := c2.Decrypt(enc); err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestCipher_TamperFails(t *testing.T) {
	k := key(t)
	c, _ := NewCipher(k)
	enc, _ := c.Encrypt("secret")
	// flip a byte in the base64 payload region
	b := []byte(enc)
	b[len(b)-2] ^= 0x01
	if _, err := c.Decrypt(string(b)); err == nil {
		t.Error("tampered ciphertext should fail authentication")
	}
}

func TestNewCipher_BadKey(t *testing.T) {
	if _, err := NewCipher(make([]byte, 16)); err == nil {
		t.Error("expected error for 16-byte key")
	}
}

func TestPassword_HashVerify(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal([]byte(hash), []byte("correct horse")) {
		t.Error("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "correct horse") {
		t.Error("verify should succeed for correct password")
	}
	if VerifyPassword(hash, "wrong") {
		t.Error("verify should fail for wrong password")
	}
}
