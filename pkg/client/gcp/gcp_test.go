package gcp

import (
	"context"
	"strings"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeAPI is an in-memory secretsAPI.
type fakeAPI struct {
	store    map[string][]byte
	creates  int
	versions int
}

func newFakeAPI() *fakeAPI { return &fakeAPI{store: map[string][]byte{}} }

func (f *fakeAPI) CreateSecret(_ context.Context, _ string, id string) error {
	f.creates++
	if _, ok := f.store[id]; !ok {
		f.store[id] = nil // container exists, no version yet
	}
	return nil
}

func (f *fakeAPI) AddVersion(_ context.Context, _ string, id string, data []byte) error {
	f.versions++
	f.store[id] = data
	return nil
}

func (f *fakeAPI) Access(_ context.Context, _ string, id string) ([]byte, error) {
	v, ok := f.store[id]
	if !ok || v == nil {
		return nil, context.Canceled // arbitrary error for "not found"
	}
	return v, nil
}

func (f *fakeAPI) Delete(_ context.Context, _ string, id string) error {
	delete(f.store, id)
	return nil
}

func (f *fakeAPI) List(_ context.Context, _ string) ([]string, error) {
	out := make([]string, 0, len(f.store))
	for k := range f.store {
		out = append(out, k)
	}
	return out, nil
}

func (f *fakeAPI) Close() error { return nil }

func newTestClient() (*Client, *fakeAPI) {
	fake := newFakeAPI()
	c := &Client{Project: "proj", api: fake}
	_ = c.Init(context.Background())
	return c, fake
}

func TestNewRequiresProject(t *testing.T) {
	if _, err := New(driver.BackendSpec{Driver: driver.DriverNameGCP}); err == nil {
		t.Fatal("expected error when project missing")
	}
	c, err := New(driver.BackendSpec{Driver: driver.DriverNameGCP, Path: "proj"})
	if err != nil || c.Project != "proj" {
		t.Fatalf("New: c=%+v err=%v", c, err)
	}
}

func TestWriteCreatesContainerThenVersion(t *testing.T) {
	ctx := context.Background()
	c, fake := newTestClient()
	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "app/db", []byte(`{"u":"p"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if fake.creates != 1 || fake.versions != 1 {
		t.Fatalf("expected 1 create + 1 version, got creates=%d versions=%d", fake.creates, fake.versions)
	}
	if string(fake.store[secretID("app/db")]) != `{"u":"p"}` {
		t.Fatalf("name sanitization or store wrong: %v", fake.store)
	}

	// Second write reuses the container (CreateSecret idempotent) and adds a
	// new version.
	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "app/db", []byte(`{"u":"q"}`)); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if fake.versions != 2 {
		t.Fatalf("expected 2 versions after second write, got %d", fake.versions)
	}
}

func TestGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient()
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "k", []byte(`{"a":"b"}`))
	got, err := c.GetSecret(ctx, "k")
	if err != nil || string(got) != `{"a":"b"}` {
		t.Fatalf("get: got=%s err=%v", got, err)
	}
}

func TestListAndDelete(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient()
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "a", []byte(`{"x":"1"}`))
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "b", []byte(`{"y":"2"}`))
	names, err := c.ListSecrets(ctx, "")
	if err != nil || len(names) != 2 {
		t.Fatalf("list: names=%v err=%v", names, err)
	}
	if err := c.DeleteSecret(ctx, "a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	names, _ = c.ListSecrets(ctx, "")
	if len(names) != 1 {
		t.Fatalf("expected 1 after delete, got %v", names)
	}
}

func TestSecretIDCleanPassthrough(t *testing.T) {
	// Underscores and hyphens are valid in Secret Manager IDs, so these pass
	// through unchanged.
	for _, in := range []string{"app-db", "App_Config", "a1_b2-c3"} {
		if got := secretID(in); got != in {
			t.Fatalf("clean id %q should pass through, got %q", in, got)
		}
	}
}

func TestSecretIDCollisionResistance(t *testing.T) {
	names := map[string]bool{}
	for _, in := range []string{"a/b", "a.b", "a b"} {
		n := secretID(in)
		if n == "a-b" {
			t.Fatalf("lossy input %q must be disambiguated, got bare %q", in, n)
		}
		if names[n] {
			t.Fatalf("collision: %q produced an already-seen id %q", in, n)
		}
		names[n] = true
	}
}

func TestSecretIDLengthBoundedAndNonEmpty(t *testing.T) {
	if got := secretID(strings.Repeat("a", 400)); len(got) > maxGCPIDLen {
		t.Fatalf("id exceeds %d: len=%d", maxGCPIDLen, len(got))
	}
	for _, in := range []string{"", "/", "---"} {
		if secretID(in) == "" {
			t.Fatalf("secretID(%q) returned empty", in)
		}
	}
}
