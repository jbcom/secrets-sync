package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeTarget is an in-memory TargetBackend for rollback tests.
type fakeTarget struct {
	drv   driver.DriverName
	store map[string][]byte
}

func newFakeTarget() *fakeTarget {
	return &fakeTarget{drv: "fake", store: map[string][]byte{}}
}

func (f *fakeTarget) Init(context.Context) error { return nil }
func (f *fakeTarget) Driver() driver.DriverName  { return f.drv }
func (f *fakeTarget) GetPath() string            { return "" }
func (f *fakeTarget) Close() error               { return nil }

func (f *fakeTarget) ListSecrets(context.Context, string) ([]string, error) {
	out := make([]string, 0, len(f.store))
	for k := range f.store {
		out = append(out, k)
	}
	return out, nil
}
func (f *fakeTarget) GetSecret(_ context.Context, name string) ([]byte, error) {
	v, ok := f.store[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return v, nil
}
func (f *fakeTarget) WriteSecret(_ context.Context, _ metav1.ObjectMeta, name string, val []byte) ([]byte, error) {
	f.store[name] = val
	return nil, nil
}
func (f *fakeTarget) DeleteSecret(_ context.Context, name string) error {
	delete(f.store, name)
	return nil
}

func TestRollbackDisabledIsNoop(t *testing.T) {
	p := &Pipeline{config: &Config{}} // rollback not enabled
	snap, err := p.snapshotForRollback(context.Background(), newFakeTarget())
	if err != nil || snap != nil {
		t.Fatalf("disabled rollback should snapshot nil: snap=%v err=%v", snap, err)
	}
}

func TestRollbackRestoresAndDeletes(t *testing.T) {
	ctx := context.Background()
	tgt := newFakeTarget()
	tgt.store["keep"] = []byte("original")

	p := &Pipeline{config: &Config{Pipeline: PipelineSettings{Rollback: RollbackConfig{Enabled: true}}}}
	snap, err := p.snapshotForRollback(ctx, tgt)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// Simulate a partial sync: mutate the existing secret and create a new one.
	tgt.store["keep"] = []byte("corrupted")
	tgt.store["created"] = []byte("new")

	if err := p.rollback(ctx, tgt, "t", snap); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if string(tgt.store["keep"]) != "original" {
		t.Fatalf("mutated secret not restored: %q", tgt.store["keep"])
	}
	if _, exists := tgt.store["created"]; exists {
		t.Fatal("secret created during failed sync should be deleted on rollback")
	}
}

// failReadTarget lists a secret but fails to read it, to exercise the
// partial-snapshot guard.
type failReadTarget struct{ *fakeTarget }

func (f *failReadTarget) ListSecrets(context.Context, string) ([]string, error) {
	return []string{"existing"}, nil
}
func (f *failReadTarget) GetSecret(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("read denied")
}

func TestSnapshotFailsOnReadError(t *testing.T) {
	p := &Pipeline{config: &Config{Pipeline: PipelineSettings{Rollback: RollbackConfig{Enabled: true}}}}
	tgt := &failReadTarget{fakeTarget: newFakeTarget()}
	// A read failure during snapshot must error (so rollback is skipped) rather
	// than capture a partial snapshot that would mis-delete existing secrets.
	if _, err := p.snapshotForRollback(context.Background(), tgt); err == nil {
		t.Fatal("snapshot should fail when a secret cannot be read")
	}
}

func TestRollbackMaxSecretsCap(t *testing.T) {
	ctx := context.Background()
	tgt := newFakeTarget()
	tgt.store["a"] = []byte("1")
	tgt.store["b"] = []byte("2")
	tgt.store["c"] = []byte("3")

	p := &Pipeline{config: &Config{Pipeline: PipelineSettings{Rollback: RollbackConfig{Enabled: true, MaxSecrets: 2}}}}
	if _, err := p.snapshotForRollback(ctx, tgt); err == nil {
		t.Fatal("snapshot should fail when target exceeds max_secrets")
	}
}

func TestRollbackNilSnapshotNoop(t *testing.T) {
	p := &Pipeline{config: &Config{}}
	if err := p.rollback(context.Background(), newFakeTarget(), "t", nil); err != nil {
		t.Fatalf("nil snapshot rollback should be a no-op: %v", err)
	}
}
