package crypto

import (
	"bytes"
	"context"
	"testing"
)

func TestPQRoundTrip(t *testing.T) {
	c, seed, err := GeneratePQKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if seed == "" {
		t.Fatal("expected a non-empty seed")
	}
	ctx := context.Background()
	plain := []byte(`{"db":"password","port":5432}`)

	ct, err := c.Encrypt(ctx, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, []byte("password")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	// Sanity: a non-envelope input must error rather than panic.
	if _, derr := c.Decrypt(ctx, plain[:0]); derr == nil {
		t.Fatal("decrypting a non-envelope should error")
	}
	got, err := c.Decrypt(ctx, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round-trip mismatch: %s", got)
	}
}

func TestPQSeedReconstruction(t *testing.T) {
	c1, seed, _ := GeneratePQKey()
	ct, _ := c1.Encrypt(context.Background(), []byte("hello"))

	// A cipher rebuilt from the seed decrypts what the original encrypted.
	c2, err := NewPQCipherFromSeed(seed)
	if err != nil {
		t.Fatalf("from seed: %v", err)
	}
	got, err := c2.Decrypt(context.Background(), ct)
	if err != nil || string(got) != "hello" {
		t.Fatalf("seed-rebuilt cipher: got=%s err=%v", got, err)
	}
}

func TestPQEncryptOnlyFromPublicKey(t *testing.T) {
	full, _, _ := GeneratePQKey()
	pub := full.PublicKey()

	enc, err := NewPQEncryptor(pub)
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	ct, err := enc.Encrypt(context.Background(), []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// The encrypt-only cipher cannot decrypt.
	if _, derr := enc.Decrypt(context.Background(), ct); derr == nil {
		t.Fatal("encrypt-only cipher must not decrypt")
	}
	// The full cipher can.
	got, err := full.Decrypt(context.Background(), ct)
	if err != nil || string(got) != "secret" {
		t.Fatalf("full cipher decrypt: got=%s err=%v", got, err)
	}
}

func TestPQWrongKeyFails(t *testing.T) {
	a, _, _ := GeneratePQKey()
	b, _, _ := GeneratePQKey()
	ct, _ := a.Encrypt(context.Background(), []byte("x"))
	if _, err := b.Decrypt(context.Background(), ct); err == nil {
		t.Fatal("decrypting with a different key pair must fail")
	}
}

func TestPQBadSeed(t *testing.T) {
	if _, err := NewPQCipherFromSeed("not-base64!!!"); err == nil {
		t.Fatal("expected error for invalid seed")
	}
}
