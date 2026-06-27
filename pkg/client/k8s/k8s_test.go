package k8s

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeSecrets is an in-memory SecretsAPI.
type fakeSecrets struct {
	store map[string]*corev1.Secret
}

func newFakeSecrets() *fakeSecrets { return &fakeSecrets{store: map[string]*corev1.Secret{}} }

func notFound(name string) error {
	return apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

func (f *fakeSecrets) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	s, ok := f.store[name]
	if !ok {
		return nil, notFound(name)
	}
	return s.DeepCopy(), nil
}

func (f *fakeSecrets) Create(_ context.Context, s *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
	if _, ok := f.store[s.Name]; ok {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, s.Name)
	}
	f.store[s.Name] = s.DeepCopy()
	return s, nil
}

func (f *fakeSecrets) Update(_ context.Context, s *corev1.Secret, _ metav1.UpdateOptions) (*corev1.Secret, error) {
	f.store[s.Name] = s.DeepCopy()
	return s, nil
}

func (f *fakeSecrets) Delete(_ context.Context, name string, _ metav1.DeleteOptions) error {
	if _, ok := f.store[name]; !ok {
		return notFound(name)
	}
	delete(f.store, name)
	return nil
}

func (f *fakeSecrets) List(_ context.Context, _ metav1.ListOptions) (*corev1.SecretList, error) {
	out := &corev1.SecretList{}
	for _, s := range f.store {
		out.Items = append(out.Items, *s)
	}
	return out, nil
}

func newTestClient() (*Client, *fakeSecrets) {
	fake := newFakeSecrets()
	c := &Client{Namespace: "app", SecretType: corev1.SecretTypeOpaque, api: fake}
	return c, fake
}

func TestNewDefaults(t *testing.T) {
	c, err := New(driver.BackendSpec{Driver: driver.DriverNameKubernetes})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Namespace != "default" {
		t.Fatalf("expected default namespace, got %q", c.Namespace)
	}
	if c.SecretType != corev1.SecretTypeOpaque {
		t.Fatalf("expected Opaque, got %q", c.SecretType)
	}
}

func TestNewWithOptions(t *testing.T) {
	c, err := New(driver.BackendSpec{
		Driver:  driver.DriverNameKubernetes,
		Path:    "team-a",
		Options: map[string]any{"secret_type": "kubernetes.io/tls", "labels": map[string]string{"managed-by": "secrets-sync"}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Namespace != "team-a" || c.SecretType != corev1.SecretTypeTLS {
		t.Fatalf("options not applied: ns=%q type=%q", c.Namespace, c.SecretType)
	}
	if c.Labels["managed-by"] != "secrets-sync" {
		t.Fatalf("labels not applied: %v", c.Labels)
	}
}

func TestWriteCreatesThenUpdates(t *testing.T) {
	ctx := context.Background()
	c, fake := newTestClient()

	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "app/db", []byte(`{"user":"alice","port":5432}`)); err != nil {
		t.Fatalf("create write: %v", err)
	}
	name := secretName("app/db")
	sec := fake.store[name]
	if sec == nil {
		t.Fatalf("secret %q not created; have %v", name, fake.store)
	}
	if string(sec.Data["user"]) != "alice" {
		t.Fatalf("string value not stored verbatim: %q", sec.Data["user"])
	}
	if string(sec.Data["port"]) != "5432" {
		t.Fatalf("non-string value should be JSON-encoded: %q", sec.Data["port"])
	}

	// Second write updates in place.
	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "app/db", []byte(`{"user":"bob"}`)); err != nil {
		t.Fatalf("update write: %v", err)
	}
	if string(fake.store[name].Data["user"]) != "bob" {
		t.Fatalf("update did not apply: %q", fake.store[name].Data["user"])
	}
	if len(fake.store) != 1 {
		t.Fatalf("update should not create a second secret: %v", fake.store)
	}
}

func TestGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient()
	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "app/db", []byte(`{"user":"alice"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := c.GetSecret(ctx, "app/db")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["user"] != "alice" {
		t.Fatalf("round-trip mismatch: %v", got)
	}
}

func TestDeleteIdempotent(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient()
	if err := c.DeleteSecret(ctx, "missing"); err != nil {
		t.Fatalf("delete of missing secret must be nil, got %v", err)
	}
}

func TestSecretNameCleanPassthrough(t *testing.T) {
	// A name that is already DNS-1123 clean and in-bounds is used verbatim.
	for _, in := range []string{"app-db", "config", "a1-b2-c3"} {
		if got := secretName(in); got != in {
			t.Fatalf("clean name %q should pass through, got %q", in, got)
		}
	}
}

func TestSecretNameCollisionResistance(t *testing.T) {
	// Inputs that sanitize to the same base must NOT collide.
	names := map[string]bool{}
	for _, in := range []string{"app/db", "app_db", "app.db", "App/DB"} {
		n := secretName(in)
		if names[n] {
			t.Fatalf("collision: %q produced an already-seen name %q", in, n)
		}
		names[n] = true
		if n == "app-db" {
			t.Fatalf("lossy input %q must be disambiguated, got bare %q", in, n)
		}
	}
}

func TestSecretNameLengthBounded(t *testing.T) {
	long := strings.Repeat("a", 400)
	got := secretName(long)
	if len(got) > maxSecretNameLen {
		t.Fatalf("name exceeds %d: len=%d", maxSecretNameLen, len(got))
	}
}

func TestSecretNameNeverEmpty(t *testing.T) {
	for _, in := range []string{"", "/", "---", "..."} {
		if got := secretName(in); got == "" {
			t.Fatalf("secretName(%q) returned empty", in)
		}
	}
}

func TestSecretTypeImmutable(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient() // Opaque
	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "x", []byte(`{"a":"b"}`)); err != nil {
		t.Fatalf("initial write: %v", err)
	}
	// Switch type and rewrite — must error clearly, not silently corrupt.
	c.SecretType = corev1.SecretTypeTLS
	_, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "x", []byte(`{"a":"b"}`))
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable-type error, got %v", err)
	}
}

func TestWriteCreateRaceFallsThroughToUpdate(t *testing.T) {
	ctx := context.Background()
	_, fake := newTestClient()
	name := secretName("x")
	// Simulate a concurrent creator: the secret already exists, but our client
	// did not observe it (so create path is taken and hits AlreadyExists).
	fake.store[name] = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "app"}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"old": []byte("v")}}
	racingClient := &Client{Namespace: "app", SecretType: corev1.SecretTypeOpaque, api: &raceOnceAPI{SecretsAPI: fake}}
	if _, err := racingClient.WriteSecret(ctx, metav1.ObjectMeta{}, "x", []byte(`{"new":"w"}`)); err != nil {
		t.Fatalf("race write should fall through to update: %v", err)
	}
	if string(fake.store[name].Data["new"]) != "w" {
		t.Fatalf("update did not apply after race: %v", fake.store[name].Data)
	}
}

// raceOnceAPI reports NotFound on the first Get so the create path is taken,
// then delegates normally — modeling a TOCTOU where another writer wins.
type raceOnceAPI struct {
	SecretsAPI
	gets int
}

func (r *raceOnceAPI) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
	r.gets++
	if r.gets == 1 {
		return nil, notFound(name)
	}
	return r.SecretsAPI.Get(ctx, name, opts)
}

func TestListSecrets(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient()
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "a", []byte(`{"x":"1"}`))
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "b", []byte(`{"y":"2"}`))
	names, err := c.ListSecrets(ctx, "")
	if err != nil || len(names) != 2 {
		t.Fatalf("list: names=%v err=%v", names, err)
	}
}
