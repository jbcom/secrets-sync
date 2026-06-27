// Package crypto provides client-side envelope encryption for secrets-sync's
// merge store, so secret bundles are encrypted before they ever reach the
// storage backend (zero-knowledge mode). Keys are either KMS-managed or
// user-supplied (32-byte AES-256 keys).
package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

// Cipher encrypts and decrypts opaque byte payloads. Implementations must be
// safe for concurrent use.
type Cipher interface {
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

// sealFormat encodes an envelope as:
//
//	[4-byte big-endian wrapped-key length][wrapped key][12-byte nonce][AES-GCM ciphertext]
//
// For a static key the wrapped-key section is empty (length 0). For KMS the
// wrapped key is the KMS-encrypted data key.
func seal(dataKey, wrappedKey, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 4+len(wrappedKey)+len(nonce)+len(ct))
	binary.BigEndian.PutUint32(out[:4], uint32(len(wrappedKey)))
	off := 4
	off += copy(out[off:], wrappedKey)
	off += copy(out[off:], nonce)
	copy(out[off:], ct)
	return out, nil
}

// openParts splits an envelope into its wrapped key and the
// nonce+ciphertext remainder.
func openParts(envelope []byte) (wrappedKey, rest []byte, err error) {
	if len(envelope) < 4 {
		return nil, nil, fmt.Errorf("crypto: envelope too short")
	}
	kl := binary.BigEndian.Uint32(envelope[:4])
	if 4+int(kl) > len(envelope) {
		return nil, nil, fmt.Errorf("crypto: envelope wrapped-key length out of range")
	}
	return envelope[4 : 4+kl], envelope[4+kl:], nil
}

// openWithKey decrypts the nonce+ciphertext remainder with the given data key.
func openWithKey(dataKey, rest []byte) ([]byte, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(rest) < ns {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ct := rest[:ns], rest[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return pt, nil
}

// StaticKeyCipher encrypts with a user-supplied 32-byte AES-256 key. No key is
// wrapped into the envelope; the same key must be supplied to decrypt.
type StaticKeyCipher struct {
	key []byte
}

// NewStaticKeyCipher validates the key length and returns a cipher.
func NewStaticKeyCipher(key []byte) (*StaticKeyCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: static key must be 32 bytes (AES-256), got %d", len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &StaticKeyCipher{key: k}, nil
}

func (c *StaticKeyCipher) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	return seal(c.key, nil, plaintext)
}

func (c *StaticKeyCipher) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	_, rest, err := openParts(ciphertext)
	if err != nil {
		return nil, err
	}
	return openWithKey(c.key, rest)
}
