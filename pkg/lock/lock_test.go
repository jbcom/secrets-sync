package lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// fakeS3Lock simulates S3 conditional-create semantics in memory.
type fakeS3Lock struct {
	mu      sync.Mutex
	objects map[string]bool
}

func newFakeS3Lock() *fakeS3Lock { return &fakeS3Lock{objects: map[string]bool{}} }

type preconditionErr struct{}

func (preconditionErr) Error() string                 { return "precondition failed" }
func (preconditionErr) ErrorCode() string             { return "PreconditionFailed" }
func (preconditionErr) ErrorMessage() string          { return "precondition failed" }
func (preconditionErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func (f *fakeS3Lock) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := *in.Key
	if in.IfNoneMatch != nil && f.objects[key] {
		return nil, preconditionErr{}
	}
	f.objects[key] = true
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3Lock) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func TestS3LockAcquireReleaseCycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeS3Lock()
	a := NewS3Lock(api, "b", "locks/sync", "pod-a")
	b := NewS3Lock(api, "b", "locks/sync", "pod-b")

	if err := a.Acquire(ctx); err != nil {
		t.Fatalf("pod-a acquire: %v", err)
	}
	// pod-b cannot acquire while held.
	if err := b.Acquire(ctx); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("pod-b should see ErrLockHeld, got %v", err)
	}
	// Release frees it.
	if err := a.Release(ctx); err != nil {
		t.Fatalf("pod-a release: %v", err)
	}
	if err := b.Acquire(ctx); err != nil {
		t.Fatalf("pod-b acquire after release: %v", err)
	}
}

func TestS3LockReleaseIdempotent(t *testing.T) {
	api := newFakeS3Lock()
	l := NewS3Lock(api, "b", "k", "p")
	if err := l.Release(context.Background()); err != nil {
		t.Fatalf("releasing an unheld lock should be a no-op: %v", err)
	}
}

func TestRunAsLeaderWinsAndReleases(t *testing.T) {
	api := newFakeS3Lock()
	l := NewS3Lock(api, "b", "k", "leader")
	ran := false
	err := RunAsLeader(context.Background(), l, LeaderConfig{}, func(ctx context.Context) error {
		ran = true
		// Lock should be held while running.
		if !api.objects["k"] {
			t.Fatal("lock not held during onElected")
		}
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("leader should run and succeed: ran=%v err=%v", ran, err)
	}
	// Lock released after.
	if api.objects["k"] {
		t.Fatal("lock should be released after onElected")
	}
}

func TestRunAsLeaderWaitsForLock(t *testing.T) {
	api := newFakeS3Lock()
	// Pre-acquire the lock as another holder.
	other := NewS3Lock(api, "b", "k", "other")
	_ = other.Acquire(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	l := NewS3Lock(api, "b", "k", "me")
	err := RunAsLeader(ctx, l, LeaderConfig{RetryInterval: 20 * time.Millisecond}, func(context.Context) error {
		t.Fatal("should never be elected while lock is held")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded waiting for held lock, got %v", err)
	}
}

func TestPartitioningIsDisjointAndComplete(t *testing.T) {
	items := []string{"prod-a", "prod-b", "stg-a", "stg-b", "dev-a", "dev-b", "sandbox-1", "sandbox-2"}
	const replicas = 3

	seen := map[string]int{}
	for r := 0; r < replicas; r++ {
		for _, it := range PartitionItems(items, r, replicas) {
			seen[it]++
		}
	}
	// Every item owned by exactly one replica.
	for _, it := range items {
		if seen[it] != 1 {
			t.Fatalf("item %q owned by %d replicas, want exactly 1", it, seen[it])
		}
	}
}

func TestPartitionSingleReplicaOwnsAll(t *testing.T) {
	items := []string{"a", "b", "c"}
	if got := PartitionItems(items, 0, 1); len(got) != 3 {
		t.Fatalf("single replica should own all items, got %v", got)
	}
}
