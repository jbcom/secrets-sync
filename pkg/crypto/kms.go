package crypto

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// kmsAPI is the subset of the KMS client the cipher uses, abstracted for tests.
type kmsAPI interface {
	GenerateDataKey(ctx context.Context, in *kms.GenerateDataKeyInput, opts ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error)
	Decrypt(ctx context.Context, in *kms.DecryptInput, opts ...func(*kms.Options)) (*kms.DecryptOutput, error)
}

// KMSCipher performs envelope encryption: each Encrypt asks KMS for a fresh
// AES-256 data key, encrypts the payload locally with it, and stores the
// KMS-wrapped data key alongside the ciphertext. Decrypt asks KMS to unwrap the
// data key, then decrypts locally. The plaintext data key never persists.
type KMSCipher struct {
	api   kmsAPI
	keyID string
}

// NewKMSCipher builds a KMS-backed cipher for the given KMS key ID/ARN/alias.
func NewKMSCipher(api kmsAPI, keyID string) (*KMSCipher, error) {
	if keyID == "" {
		return nil, fmt.Errorf("crypto: kms key id is required")
	}
	return &KMSCipher{api: api, keyID: keyID}, nil
}

// encryptionContext binds ciphertext to this key so KMS rejects a decrypt with a
// mismatched context. For symmetric keys KMS otherwise ignores KeyId, so this is
// what actually enforces cross-key binding.
func (c *KMSCipher) encryptionContext() map[string]string {
	return map[string]string{"service": "secrets-sync", "key-id": c.keyID}
}

// Encrypt wraps a fresh data key via KMS and seals the payload with it.
func (c *KMSCipher) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	out, err := c.api.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:             aws.String(c.keyID),
		KeySpec:           kmstypes.DataKeySpecAes256,
		EncryptionContext: c.encryptionContext(),
	})
	if err != nil {
		return nil, fmt.Errorf("crypto: kms generate data key: %w", err)
	}
	// Zero the plaintext data key explicitly after seal returns, rather than via
	// defer, so the lifetime of the key material is obvious and minimal.
	sealed, sealErr := seal(out.Plaintext, out.CiphertextBlob, plaintext)
	zero(out.Plaintext)
	if sealErr != nil {
		return nil, sealErr
	}
	return sealed, nil
}

// Decrypt unwraps the embedded data key via KMS and opens the payload.
func (c *KMSCipher) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	wrapped, rest, err := openParts(ciphertext)
	if err != nil {
		return nil, err
	}
	if len(wrapped) == 0 {
		return nil, fmt.Errorf("crypto: envelope has no wrapped key (was it encrypted with a static key?)")
	}
	out, err := c.api.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob:    wrapped,
		KeyId:             aws.String(c.keyID),
		EncryptionContext: c.encryptionContext(),
	})
	if err != nil {
		return nil, fmt.Errorf("crypto: kms decrypt data key: %w", err)
	}
	opened, openErr := openWithKey(out.Plaintext, rest)
	zero(out.Plaintext)
	return opened, openErr
}
