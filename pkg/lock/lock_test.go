package lock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// fakeS3Lock simulates S3 conditional-create + read/delete semantics in memory.
type fakeS3Lock struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeS3Lock() *fakeS3Lock { return &fakeS3Lock{objects: map[string][]byte{}} }

type preconditionErr struct{}

func (preconditionErr) Error() string                 { return "precondition failed" }
func (preconditionErr) ErrorCode() string             { return "PreconditionFailed" }
func (preconditionErr) ErrorMessage() string          { return "precondition failed" }
func (preconditionErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func (f *fakeS3Lock) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := *in.Key
	if in.IfNoneMatch != nil {
		if _, exists := f.objects[key]; exists {
			return nil, preconditionErr{}
		}
	}
	body := []byte{}
	if in.Body != nil {
		b, _ := io.ReadAll(in.Body)
		body = b
	}
	f.objects[key] = body
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3Lock) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, ok := f.objects[*in.Key]
	if !ok {
		return nil, fmt.Errorf("NoSuchKey")
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func (f *fakeS3Lock) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeS3Lock) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func TestS3LockAcquireReleaseCycle(t *testing.T) {
	ctx := context.Background()
	api := newFakeS3Lock()
	a := NewS3Lock(api, "b", "locks/sync", "pod-a", time.Minute)
	b := NewS3Lock(api, "b", "locks/sync", "pod-b", time.Minute)

	if err := a.Acquire(ctx); err != nil {
		t.Fatalf("pod-a acquire: %v", err)
	}
	// pod-b cannot acquire while a live lease is held.
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

func TestS3LockExpiredLeaseTakeover(t *testing.T) {
	ctx := context.Background()
	api := newFakeS3Lock()
	// pod-a acquires with a clock fixed in the past so its lease is already
	// expired from pod-b's perspective.
	past := time.Now().Add(-time.Hour)
	a := NewS3Lock(api, "b", "k", "pod-a", time.Minute)
	a.now = func() time.Time { return past }
	if err := a.Acquire(ctx); err != nil {
		t.Fatalf("pod-a acquire: %v", err)
	}

	// pod-b, with the real clock, sees the expired lease and takes it over.
	b := NewS3Lock(api, "b", "k", "pod-b", time.Minute)
	if err := b.Acquire(ctx); err != nil {
		t.Fatalf("pod-b should take over expired lease, got %v", err)
	}
}

func TestS3LockReleaseIdempotent(t *testing.T) {
	api := newFakeS3Lock()
	l := NewS3Lock(api, "b", "k", "p", time.Minute)
	if err := l.Release(context.Background()); err != nil {
		t.Fatalf("releasing an unheld lock should be a no-op: %v", err)
	}
}

func TestRunAsLeaderWinsAndReleases(t *testing.T) {
	api := newFakeS3Lock()
	l := NewS3Lock(api, "b", "k", "leader", time.Minute)
	ran := false
	err := RunAsLeader(context.Background(), l, LeaderConfig{}, func(ctx context.Context) error {
		ran = true
		if !api.has("k") {
			t.Fatal("lock not held during onElected")
		}
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("leader should run and succeed: ran=%v err=%v", ran, err)
	}
	if api.has("k") {
		t.Fatal("lock should be released after onElected")
	}
}

func TestRunAsLeaderWaitsForLock(t *testing.T) {
	api := newFakeS3Lock()
	// Pre-acquire the lock as another holder with a long lease.
	other := NewS3Lock(api, "b", "k", "other", time.Hour)
	_ = other.Acquire(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	l := NewS3Lock(api, "b", "k", "me", time.Hour)
	err := RunAsLeader(ctx, l, LeaderConfig{RetryInterval: 20 * time.Millisecond}, func(context.Context) error {
		t.Fatal("should never be elected while a live lease is held")
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
