package lock

import (
	"context"
	"hash/fnv"
	"time"
)

// Locker is the minimal lock surface leader election needs.
type Locker interface {
	Acquire(ctx context.Context) error
	Release(ctx context.Context) error
}

// LeaderConfig configures the leader-election loop.
type LeaderConfig struct {
	// RetryInterval is how long to wait before re-attempting acquisition when
	// the lock is held by another replica. Defaults to 15s.
	RetryInterval time.Duration
}

// RunAsLeader blocks until it wins the lock, runs onElected once with a context
// that is cancelled when the function returns, then releases the lock. While the
// lock is held by another replica it retries on RetryInterval until ctx is
// cancelled. It returns the error from onElected, or ctx.Err() if cancelled
// before winning.
func RunAsLeader(ctx context.Context, locker Locker, cfg LeaderConfig, onElected func(ctx context.Context) error) error {
	interval := cfg.RetryInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}

	for {
		err := locker.Acquire(ctx)
		switch {
		case err == nil:
			// Won leadership. Run, then always release.
			runErr := onElected(ctx)
			// Use a fresh short context for release so a cancelled ctx still
			// frees the lock for the next replica.
			relCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = locker.Release(relCtx)
			cancel()
			return runErr
		case err == ErrLockHeld:
			// Another replica holds it; wait and retry.
		default:
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// PartitionOwner reports whether this replica owns a given work item under a
// simple stable hash partitioning across replicaCount replicas. Each item is
// assigned to exactly one replica index, so N replicas can each process a
// disjoint slice without a coordinator. replicaIndex is 0-based.
func PartitionOwner(item string, replicaIndex, replicaCount int) bool {
	if replicaCount <= 1 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(item))
	return int(h.Sum32()%uint32(replicaCount)) == replicaIndex
}

// PartitionItems returns the subset of items owned by this replica.
func PartitionItems(items []string, replicaIndex, replicaCount int) []string {
	if replicaCount <= 1 {
		return items
	}
	var owned []string
	for _, it := range items {
		if PartitionOwner(it, replicaIndex, replicaCount) {
			owned = append(owned, it)
		}
	}
	return owned
}
