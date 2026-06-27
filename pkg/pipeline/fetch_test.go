package pipeline

import (
	"context"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeSource is an in-memory SourceBackend for exercising the generic fetch
// path. listNames controls exactly what ListSecrets returns so we can model
// both flat (AWS-style) and fully-qualified (Vault-style) name schemes.
type fakeSource struct {
	drv       driver.DriverName
	store     map[string][]byte // keyed by the path GetSecret receives
	listNames []string
}

func (f *fakeSource) Init(context.Context) error { return nil }
func (f *fakeSource) Driver() driver.DriverName  { return f.drv }
func (f *fakeSource) GetPath() string            { return "" }
func (f *fakeSource) Close() error               { return nil }

func (f *fakeSource) ListSecrets(context.Context, string) ([]string, error) {
	return f.listNames, nil
}

func (f *fakeSource) GetSecret(_ context.Context, path string) ([]byte, error) {
	return f.store[path], nil
}

func (f *fakeSource) WriteSecret(context.Context, metav1.ObjectMeta, string, []byte) ([]byte, error) {
	return nil, nil
}
func (f *fakeSource) DeleteSecret(context.Context, string) error { return nil }

func TestFetchSourceSecrets_FlatNames(t *testing.T) {
	// AWS-style: path is "", names are bare, GetSecret receives the bare name.
	src := &fakeSource{
		drv:       driver.DriverNameAws,
		listNames: []string{"app/db", "app/api"},
		store: map[string][]byte{
			"app/db":  []byte(`{"u":"p"}`),
			"app/api": []byte(`{"key":"v"}`),
		},
	}
	got, err := (&Pipeline{}).fetchSourceSecrets(context.Background(), src, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 secrets, got %d: %v", len(got), got)
	}
	if m, ok := got["app/db"].(map[string]interface{}); !ok || m["u"] != "p" {
		t.Fatalf("bad value for app/db: %v", got["app/db"])
	}
}

func TestFetchSourceSecrets_QualifiedNames(t *testing.T) {
	// Vault-style: names are fully qualified under the scope; keys are
	// scope-relative; GetSecret receives the full path.
	src := &fakeSource{
		drv:       driver.DriverNameVault,
		listNames: []string{"kv/app/db", "kv/app/nested/api"},
		store: map[string][]byte{
			"kv/app/db":         []byte(`{"u":"p"}`),
			"kv/app/nested/api": []byte(`{"key":"v"}`),
		},
	}
	got, err := (&Pipeline{}).fetchSourceSecrets(context.Background(), src, "kv/app")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, ok := got["db"]; !ok {
		t.Fatalf("expected scope-relative key \"db\", got keys: %v", keys(got))
	}
	if _, ok := got["nested/api"]; !ok {
		t.Fatalf("expected scope-relative key \"nested/api\", got keys: %v", keys(got))
	}
}

func TestFetchSourceSecrets_SiblingPrefixBoundary(t *testing.T) {
	// Regression: scope "kv/app" must NOT swallow a sibling under
	// "kv/application". The boundary check requires a trailing slash, so the
	// sibling is treated as a bare name (read via prefix-join) and keyed by its
	// full name rather than a mangled "lication/..." key.
	src := &fakeSource{
		drv:       driver.DriverNameVault,
		listNames: []string{"kv/app/db", "kv/application/db"},
		store: map[string][]byte{
			"kv/app/db":                []byte(`{"in":"scope"}`),
			"kv/app/kv/application/db": []byte(`{"sibling":"true"}`),
		},
	}
	got, err := (&Pipeline{}).fetchSourceSecrets(context.Background(), src, "kv/app")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, ok := got["db"]; !ok {
		t.Fatalf("in-scope secret should key to \"db\"; keys=%v", keys(got))
	}
	// The sibling must NOT produce a mangled "lication/db" key.
	if _, bad := got["lication/db"]; bad {
		t.Fatalf("sibling prefix was mangled into \"lication/db\": %v", keys(got))
	}
}

func keys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
