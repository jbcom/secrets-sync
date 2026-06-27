package driver

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeBackend is a minimal in-memory backend used to verify the interface
// hierarchy compiles and composes as intended.
type fakeBackend struct {
	name DriverName
	path string
	data map[string][]byte
}

func newFakeBackend(name DriverName) *fakeBackend {
	return &fakeBackend{name: name, data: map[string][]byte{}}
}

func (f *fakeBackend) Init(context.Context) error { return nil }
func (f *fakeBackend) Driver() DriverName         { return f.name }
func (f *fakeBackend) GetPath() string            { return f.path }
func (f *fakeBackend) Close() error               { return nil }

func (f *fakeBackend) ListSecrets(_ context.Context, _ string) ([]string, error) {
	names := make([]string, 0, len(f.data))
	for k := range f.data {
		names = append(names, k)
	}
	return names, nil
}

func (f *fakeBackend) GetSecret(_ context.Context, path string) ([]byte, error) {
	return f.data[path], nil
}

func (f *fakeBackend) WriteSecret(_ context.Context, _ metav1.ObjectMeta, path string, secret []byte) ([]byte, error) {
	f.data[path] = secret
	return secret, nil
}

func (f *fakeBackend) DeleteSecret(_ context.Context, path string) error {
	delete(f.data, path)
	return nil
}

// fakeMergeStore is a minimal in-memory MergeStore for contract verification.
type fakeMergeStore struct {
	data map[string]map[string]map[string]interface{} // target -> secret -> data
}

func newFakeMergeStore() *fakeMergeStore {
	return &fakeMergeStore{data: map[string]map[string]map[string]interface{}{}}
}

func (m *fakeMergeStore) WriteSecret(_ context.Context, target, secret string, data map[string]interface{}) error {
	if m.data[target] == nil {
		m.data[target] = map[string]map[string]interface{}{}
	}
	m.data[target][secret] = data
	return nil
}

func (m *fakeMergeStore) ReadSecret(_ context.Context, target, secret string) (map[string]interface{}, error) {
	t, ok := m.data[target]
	if !ok {
		return nil, fmt.Errorf("target %q not found", target)
	}
	d, ok := t[secret]
	if !ok {
		return nil, fmt.Errorf("secret %q not found", secret)
	}
	return d, nil
}

func (m *fakeMergeStore) ListSecrets(_ context.Context, target string) ([]string, error) {
	out := make([]string, 0, len(m.data[target]))
	for k := range m.data[target] {
		out = append(out, k)
	}
	return out, nil
}

func (m *fakeMergeStore) DeleteSecret(_ context.Context, target, secret string) error {
	if t := m.data[target]; t != nil {
		delete(t, secret)
	}
	return nil
}

// Compile-time guarantees about the interface hierarchy.
var (
	_ Backend       = (*fakeBackend)(nil)
	_ SourceBackend = (*fakeBackend)(nil)
	_ TargetBackend = (*fakeBackend)(nil)
	_ MergeStore    = (*fakeMergeStore)(nil)
)

func TestTargetBackendIsAlsoSource(t *testing.T) {
	var tb TargetBackend = newFakeBackend(DriverNameAws)
	// A TargetBackend must be usable wherever a SourceBackend is expected so the
	// sync phase can read back current state for diffing.
	var sb SourceBackend = tb
	if sb.Driver() != DriverNameAws {
		t.Fatalf("expected driver %q, got %q", DriverNameAws, sb.Driver())
	}
}

func TestBackendRoundTrip(t *testing.T) {
	ctx := context.Background()
	b := newFakeBackend(DriverNameVault)
	if err := b.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer b.Close()

	if _, err := b.WriteSecret(ctx, metav1.ObjectMeta{Name: "x"}, "app/db", []byte(`{"u":"p"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := b.GetSecret(ctx, "app/db")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != `{"u":"p"}` {
		t.Fatalf("unexpected payload: %s", got)
	}
	names, err := b.ListSecrets(ctx, "")
	if err != nil || len(names) != 1 {
		t.Fatalf("list: names=%v err=%v", names, err)
	}
	if err := b.DeleteSecret(ctx, "app/db"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMergeStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	var ms MergeStore = newFakeMergeStore()

	if err := ms.WriteSecret(ctx, "prod", "app/db", map[string]interface{}{"u": "p"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ms.ReadSecret(ctx, "prod", "app/db")
	if err != nil || got["u"] != "p" {
		t.Fatalf("read: got=%v err=%v", got, err)
	}
	names, err := ms.ListSecrets(ctx, "prod")
	if err != nil || len(names) != 1 {
		t.Fatalf("list: names=%v err=%v", names, err)
	}
	// Reads scoped per-target: unknown target/secret must error, not return nil.
	if _, err := ms.ReadSecret(ctx, "staging", "app/db"); err == nil {
		t.Fatal("expected error reading from unknown target")
	}
	if err := ms.DeleteSecret(ctx, "prod", "app/db"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := ms.ReadSecret(ctx, "prod", "app/db"); err == nil {
		t.Fatal("expected error reading deleted secret")
	}
}
