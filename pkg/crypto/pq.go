package crypto

import (
	"context"
	"crypto/mlkem"
	"encoding/base64"
	"fmt"
)

// PQCipher provides post-quantum (quantum-resistant) encryption-at-rest using
// ML-KEM-768 (NIST FIPS 203) for key encapsulation combined with AES-256-GCM
// for the bulk payload. Each Encrypt encapsulates a fresh shared secret against
// the encapsulation (public) key; the KEM ciphertext is stored in the envelope's
// wrapped-key slot and the shared secret is HKDF-mixed (via seal/openWithKey) to
// derive the AES key. Only the holder of the decapsulation (secret) key can
// recover the shared secret, even against a quantum adversary.
type PQCipher struct {
	// ek is required for Encrypt; dk is required for Decrypt. A cipher built
	// from a secret key can do both; one built from a public key can only
	// encrypt.
	ek *mlkem.EncapsulationKey768
	dk *mlkem.DecapsulationKey768
}

// GeneratePQKey generates a new ML-KEM-768 key pair and returns a cipher capable
// of both encryption and decryption, plus the seed needed to reconstruct the
// decapsulation key later (store it as a secret). The seed is base64-encoded.
func GeneratePQKey() (*PQCipher, string, error) {
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, "", fmt.Errorf("crypto: generate ML-KEM key: %w", err)
	}
	seed := base64.StdEncoding.EncodeToString(dk.Bytes())
	return &PQCipher{ek: dk.EncapsulationKey(), dk: dk}, seed, nil
}

// NewPQCipherFromSeed reconstructs a full (encrypt+decrypt) cipher from a
// base64-encoded decapsulation-key seed produced by GeneratePQKey.
func NewPQCipherFromSeed(seedB64 string) (*PQCipher, error) {
	seed, err := base64.StdEncoding.DecodeString(seedB64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode ML-KEM seed: %w", err)
	}
	dk, err := mlkem.NewDecapsulationKey768(seed)
	if err != nil {
		return nil, fmt.Errorf("crypto: load ML-KEM key: %w", err)
	}
	return &PQCipher{ek: dk.EncapsulationKey(), dk: dk}, nil
}

// NewPQEncryptor builds an encrypt-only cipher from a base64-encoded
// encapsulation (public) key, so an encrypting party need not hold the secret.
func NewPQEncryptor(ekB64 string) (*PQCipher, error) {
	raw, err := base64.StdEncoding.DecodeString(ekB64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode ML-KEM public key: %w", err)
	}
	ek, err := mlkem.NewEncapsulationKey768(raw)
	if err != nil {
		return nil, fmt.Errorf("crypto: load ML-KEM public key: %w", err)
	}
	return &PQCipher{ek: ek}, nil
}

// PublicKey returns the base64-encoded encapsulation key for distribution to
// encrypt-only parties.
func (c *PQCipher) PublicKey() string {
	return base64.StdEncoding.EncodeToString(c.ek.Bytes())
}

// Encrypt encapsulates a fresh shared secret and seals the payload with it.
func (c *PQCipher) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	if c.ek == nil {
		return nil, fmt.Errorf("crypto: PQ cipher has no encapsulation key")
	}
	sharedKey, kemCiphertext := c.ek.Encapsulate()
	defer zero(sharedKey)
	// The KEM ciphertext goes in the wrapped-key slot; sharedKey is the root key
	// HKDF-derived inside seal.
	return seal(sharedKey, kemCiphertext, plaintext)
}

// Decrypt decapsulates the shared secret from the envelope's KEM ciphertext and
// opens the payload.
func (c *PQCipher) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	if c.dk == nil {
		return nil, fmt.Errorf("crypto: PQ cipher has no decapsulation key (encrypt-only)")
	}
	kemCiphertext, rest, err := openParts(ciphertext)
	if err != nil {
		return nil, err
	}
	if len(kemCiphertext) == 0 {
		return nil, fmt.Errorf("crypto: PQ envelope has no KEM ciphertext")
	}
	sharedKey, err := c.dk.Decapsulate(kemCiphertext)
	if err != nil {
		return nil, fmt.Errorf("crypto: ML-KEM decapsulate: %w", err)
	}
	defer zero(sharedKey)
	return openWithKey(sharedKey, rest)
}
