package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// fakeKMS simulates envelope wrapping: the "wrapped" blob is the plaintext key
// with a fixed prefix, so Decrypt can recover the data key deterministically
// without real cryptography. This exercises the cipher's envelope plumbing.
type fakeKMS struct {
	genCalls int
	decCalls int
}

var wrapPrefix = []byte("WRAP:")

func (f *fakeKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	f.genCalls++
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	wrapped := append(append([]byte{}, wrapPrefix...), key...)
	return &kms.GenerateDataKeyOutput{Plaintext: key, CiphertextBlob: wrapped}, nil
}

func (f *fakeKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	f.decCalls++
	blob := in.CiphertextBlob
	if !bytes.HasPrefix(blob, wrapPrefix) {
		return nil, fmt.Errorf("bad wrapped blob")
	}
	return &kms.DecryptOutput{Plaintext: blob[len(wrapPrefix):]}, nil
}

func TestKMSCipherRoundTrip(t *testing.T) {
	fake := &fakeKMS{}
	c, err := NewKMSCipher(fake, "alias/test")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()
	plain := []byte(`{"db":"password"}`)

	ct, err := c.Encrypt(ctx, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, []byte("password")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	got, err := c.Decrypt(ctx, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round-trip mismatch: %s", got)
	}
	if fake.genCalls != 1 || fake.decCalls != 1 {
		t.Fatalf("expected 1 gen + 1 dec, got gen=%d dec=%d", fake.genCalls, fake.decCalls)
	}
}

func TestKMSCipherRequiresKeyID(t *testing.T) {
	if _, err := NewKMSCipher(&fakeKMS{}, ""); err == nil {
		t.Fatal("expected error for empty key id")
	}
}

func TestKMSCipherRejectsStaticEnvelope(t *testing.T) {
	// An envelope with no wrapped key (static-key produced) must not decrypt
	// via KMS — it has no data key to unwrap.
	static, _ := NewStaticKeyCipher(make([]byte, 32))
	ct, _ := static.Encrypt(context.Background(), []byte("x"))
	kmsC, _ := NewKMSCipher(&fakeKMS{}, "alias/test")
	if _, err := kmsC.Decrypt(context.Background(), ct); err == nil {
		t.Fatal("KMS decrypt of a static-key envelope must fail")
	}
}
