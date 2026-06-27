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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// Cipher encrypts and decrypts opaque byte payloads. Implementations must be
// safe for concurrent use.
type Cipher interface {
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

const (
	saltLen   = 32
	nonceLen  = 12
	hkdfInfo  = "secrets-sync envelope v1"
	gcmTagLen = 16
)

// deriveKey derives a unique 32-byte AES-256 encryption key from a root key and
// a per-envelope random salt via HKDF-SHA256. Deriving a fresh key per envelope
// means the (key, nonce) pair is never reused even when the same root/data key
// encrypts many bundles, eliminating the GCM birthday-bound nonce-reuse risk.
func deriveKey(rootKey, salt []byte) ([]byte, error) {
	r := hkdf.New(sha256.New, rootKey, salt, []byte(hkdfInfo))
	out := make([]byte, 32)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("crypto: derive key: %w", err)
	}
	return out, nil
}

// seal encodes an envelope as:
//
//	[4-byte big-endian wrapped-key length][wrapped key][32-byte salt][12-byte nonce][AES-GCM ciphertext]
//
// For a static key the wrapped-key section is empty (length 0); for KMS it holds
// the KMS-encrypted data key. The actual AES key is HKDF-derived from rootKey
// and the random salt, so each envelope uses a distinct key.
func seal(rootKey, wrappedKey, plaintext []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("crypto: read salt: %w", err)
	}
	encKey, err := deriveKey(rootKey, salt)
	if err != nil {
		return nil, err
	}
	defer zero(encKey)

	block, err := aes.NewCipher(encKey)
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

	out := make([]byte, 4+len(wrappedKey)+saltLen+len(nonce)+len(ct))
	binary.BigEndian.PutUint32(out[:4], uint32(len(wrappedKey)))
	off := 4
	off += copy(out[off:], wrappedKey)
	off += copy(out[off:], salt)
	off += copy(out[off:], nonce)
	copy(out[off:], ct)
	return out, nil
}

// openParts splits an envelope into its wrapped key and the
// salt+nonce+ciphertext remainder, using uint64 arithmetic and a minimum-length
// check so a crafted length prefix cannot overflow or pass a too-short body.
func openParts(envelope []byte) (wrappedKey, rest []byte, err error) {
	if len(envelope) < 4 {
		return nil, nil, fmt.Errorf("crypto: envelope too short")
	}
	kl := uint64(binary.BigEndian.Uint32(envelope[:4]))
	// rest must hold at least salt + nonce + GCM tag (an empty plaintext still
	// produces a tag).
	minRest := uint64(saltLen + nonceLen + gcmTagLen)
	if 4+kl+minRest > uint64(len(envelope)) {
		return nil, nil, fmt.Errorf("crypto: envelope wrapped-key length out of range")
	}
	return envelope[4 : 4+kl], envelope[4+kl:], nil
}

// openWithKey derives the per-envelope key from rootKey and the embedded salt,
// then decrypts the nonce+ciphertext remainder.
func openWithKey(rootKey, rest []byte) ([]byte, error) {
	if len(rest) < saltLen+nonceLen {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	salt := rest[:saltLen]
	body := rest[saltLen:]

	encKey, err := deriveKey(rootKey, salt)
	if err != nil {
		return nil, err
	}
	defer zero(encKey)

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(body) < ns {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ct := body[:ns], body[ns:]
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
	wrapped, rest, err := openParts(ciphertext)
	if err != nil {
		return nil, err
	}
	// A non-empty wrapped key means this envelope was produced by the KMS cipher;
	// reject it with an actionable error rather than failing opaquely on GCM auth.
	if len(wrapped) != 0 {
		return nil, fmt.Errorf("crypto: envelope has a wrapped data key (encrypted with KMS, not a static key)")
	}
	return openWithKey(c.key, rest)
}

// zero overwrites a byte slice to limit how long sensitive key material lingers
// in memory.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
