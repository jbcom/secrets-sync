package pipeline

import (
	"context"
	"fmt"
	"testing"
)

// fakeBundleStore is an in-memory BundleStore with controllable failures.
type fakeBundleStore struct {
	data      map[string]map[string]map[string]interface{}
	failWrite bool
	failRead  bool
}

func newFakeBundleStore() *fakeBundleStore {
	return &fakeBundleStore{data: map[string]map[string]map[string]interface{}{}}
}

func bundleKey(target, id string) string { return target + "/" + id }

func (f *fakeBundleStore) WriteMergedBundle(_ context.Context, target, id string, secrets map[string]interface{}) error {
	if f.failWrite {
		return fmt.Errorf("write failed")
	}
	conv := map[string]map[string]interface{}{}
	for k, v := range secrets {
		if m, ok := v.(map[string]interface{}); ok {
			conv[k] = m
		}
	}
	f.data[bundleKey(target, id)] = conv
	return nil
}

func (f *fakeBundleStore) ReadMergedBundle(_ context.Context, target, id string) (map[string]map[string]interface{}, error) {
	if f.failRead {
		return nil, fmt.Errorf("read failed")
	}
	d, ok := f.data[bundleKey(target, id)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return d, nil
}

func TestReplicationWritesToPrimaryAndReplicas(t *testing.T) {
	ctx := context.Background()
	primary, r1, r2 := newFakeBundleStore(), newFakeBundleStore(), newFakeBundleStore()
	rs := NewReplicatingBundleStore(primary, r1, r2)

	secrets := map[string]interface{}{"app": map[string]interface{}{"k": "v"}}
	if err := rs.WriteMergedBundle(ctx, "prod", "b1", secrets); err != nil {
		t.Fatalf("write: %v", err)
	}
	for name, s := range map[string]*fakeBundleStore{"primary": primary, "r1": r1, "r2": r2} {
		if _, ok := s.data[bundleKey("prod", "b1")]; !ok {
			t.Fatalf("%s did not receive the bundle", name)
		}
	}
}

func TestReplicationPrimaryFailureAborts(t *testing.T) {
	primary := newFakeBundleStore()
	primary.failWrite = true
	rs := NewReplicatingBundleStore(primary, newFakeBundleStore())
	if err := rs.WriteMergedBundle(context.Background(), "t", "b", map[string]interface{}{}); err == nil {
		t.Fatal("primary write failure must abort")
	}
}

func TestReplicationReplicaFailureBestEffort(t *testing.T) {
	primary, bad := newFakeBundleStore(), newFakeBundleStore()
	bad.failWrite = true
	rs := NewReplicatingBundleStore(primary, bad) // RequireAllReplicas=false
	if err := rs.WriteMergedBundle(context.Background(), "t", "b", map[string]interface{}{}); err != nil {
		t.Fatalf("best-effort replica failure should not abort: %v", err)
	}
}

func TestReplicationRequireAllReplicas(t *testing.T) {
	primary, bad := newFakeBundleStore(), newFakeBundleStore()
	bad.failWrite = true
	rs := NewReplicatingBundleStore(primary, bad)
	rs.RequireAllReplicas = true
	if err := rs.WriteMergedBundle(context.Background(), "t", "b", map[string]interface{}{}); err == nil {
		t.Fatal("RequireAllReplicas should propagate replica failure")
	}
}

func TestReplicationReadFallsBackToReplica(t *testing.T) {
	ctx := context.Background()
	primary, replica := newFakeBundleStore(), newFakeBundleStore()
	// Only the replica has the data; primary read fails.
	_ = replica.WriteMergedBundle(ctx, "t", "b", map[string]interface{}{"x": map[string]interface{}{"k": "v"}})
	primary.failRead = true
	rs := NewReplicatingBundleStore(primary, replica)

	got, err := rs.ReadMergedBundle(ctx, "t", "b")
	if err != nil {
		t.Fatalf("read should fall back to replica: %v", err)
	}
	if got["x"]["k"] != "v" {
		t.Fatalf("unexpected data from replica: %v", got)
	}
}

func TestReplicationReadAllFail(t *testing.T) {
	primary, replica := newFakeBundleStore(), newFakeBundleStore()
	primary.failRead = true
	replica.failRead = true
	rs := NewReplicatingBundleStore(primary, replica)
	if _, err := rs.ReadMergedBundle(context.Background(), "t", "b"); err == nil {
		t.Fatal("read should fail when primary and all replicas fail")
	}
}
