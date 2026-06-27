package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestStaticKeyRoundTrip(t *testing.T) {
	c, err := NewStaticKeyCipher(mustKey(t))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()
	plain := []byte(`{"secret":"value","n":42}`)

	ct, err := c.Encrypt(ctx, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, []byte("value")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	got, err := c.Decrypt(ctx, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round-trip mismatch: %s", got)
	}
}

func TestStaticKeyWrongKeyFails(t *testing.T) {
	ctx := context.Background()
	c1, _ := NewStaticKeyCipher(mustKey(t))
	c2, _ := NewStaticKeyCipher(mustKey(t))
	ct, _ := c1.Encrypt(ctx, []byte("hello"))
	if _, err := c2.Decrypt(ctx, ct); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

func TestStaticKeyTamperFails(t *testing.T) {
	ctx := context.Background()
	c, _ := NewStaticKeyCipher(mustKey(t))
	ct, _ := c.Encrypt(ctx, []byte("hello world"))
	// Flip a byte in the ciphertext tail.
	ct[len(ct)-1] ^= 0xFF
	if _, err := c.Decrypt(ctx, ct); err == nil {
		t.Fatal("GCM must reject tampered ciphertext")
	}
}

func TestStaticKeyBadLength(t *testing.T) {
	if _, err := NewStaticKeyCipher([]byte("short")); err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}

func TestEmptyAndShortEnvelopes(t *testing.T) {
	c, _ := NewStaticKeyCipher(mustKey(t))
	ctx := context.Background()
	if _, err := c.Decrypt(ctx, []byte{}); err == nil {
		t.Fatal("empty envelope must error")
	}
	if _, err := c.Decrypt(ctx, []byte{0, 0, 0, 5}); err == nil {
		t.Fatal("wrapped-key length beyond buffer must error")
	}
}
