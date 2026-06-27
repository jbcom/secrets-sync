package pipeline

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestBuildBundleCipherStaticKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	t.Setenv("SS_TEST_KEY", base64.StdEncoding.EncodeToString(key))

	c, err := buildBundleCipher(aws.Config{}, &EncryptionConfig{Enabled: true, KeyEnv: "SS_TEST_KEY"})
	if err != nil {
		t.Fatalf("build static cipher: %v", err)
	}
	// Prove the constructed cipher actually round-trips.
	ct, err := c.Encrypt(context.Background(), []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pt, err := c.Decrypt(context.Background(), ct)
	if err != nil || string(pt) != "secret" {
		t.Fatalf("round-trip: pt=%q err=%v", pt, err)
	}
}

func TestBuildBundleCipherKMS(t *testing.T) {
	c, err := buildBundleCipher(aws.Config{Region: "us-east-1"}, &EncryptionConfig{Enabled: true, KMSKeyID: "alias/x"})
	if err != nil || c == nil {
		t.Fatalf("build kms cipher: c=%v err=%v", c, err)
	}
}

func TestBuildBundleCipherErrors(t *testing.T) {
	// Neither key source set.
	if _, err := buildBundleCipher(aws.Config{}, &EncryptionConfig{Enabled: true}); err == nil {
		t.Fatal("expected error when no key source configured")
	}
	// key_env points at an empty/missing variable.
	if _, err := buildBundleCipher(aws.Config{}, &EncryptionConfig{Enabled: true, KeyEnv: "SS_MISSING_KEY"}); err == nil {
		t.Fatal("expected error for empty key_env")
	}
	// key_env holds non-base64.
	t.Setenv("SS_BAD_KEY", "not-base64-!!!")
	if _, err := buildBundleCipher(aws.Config{}, &EncryptionConfig{Enabled: true, KeyEnv: "SS_BAD_KEY"}); err == nil {
		t.Fatal("expected error for invalid base64 key")
	}
}
