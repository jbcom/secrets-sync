package lock

import (
	"context"
	"errors"
	"hash/fnv"
	"time"

	log "github.com/sirupsen/logrus"
)

// Locker is the minimal lock surface leader election needs. Refresh extends the
// lease so a long-running leader does not let its lease expire mid-work.
type Locker interface {
	Acquire(ctx context.Context) error
	Refresh(ctx context.Context) error
	Release(ctx context.Context) error
}

// LeaderConfig configures the leader-election loop.
type LeaderConfig struct {
	// RetryInterval is how long to wait before re-attempting acquisition when
	// the lock is held by another replica. Defaults to 15s.
	RetryInterval time.Duration
	// HeartbeatInterval is how often the held lease is refreshed while
	// onElected runs. It should be well under the lock's TTL. Defaults to 20s.
	HeartbeatInterval time.Duration
}

// RunAsLeader blocks until it wins the lock, runs onElected once while
// heartbeating the lease to keep it from expiring, then releases the lock. While
// the lock is held by another replica it retries on RetryInterval until ctx is
// cancelled. It returns the error from onElected, or ctx.Err() if cancelled
// before winning.
func RunAsLeader(ctx context.Context, locker Locker, cfg LeaderConfig, onElected func(ctx context.Context) error) error {
	interval := cfg.RetryInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	heartbeat := cfg.HeartbeatInterval
	if heartbeat <= 0 {
		heartbeat = 20 * time.Second
	}

	for {
		err := locker.Acquire(ctx)
		switch {
		case err == nil:
			return runElected(ctx, locker, heartbeat, onElected)
		case errors.Is(err, ErrLockHeld):
			// Another replica holds a live lease; wait and retry.
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

// runElected runs onElected with a heartbeat goroutine refreshing the lease, and
// always releases the lock afterward (via a fresh context so a cancelled parent
// still frees it for the next replica).
//
// onElected runs with a work context that is cancelled if the lease heartbeat
// fails, so a leader that loses its lease (and could thus be superseded by
// another replica) stops working rather than risking split-brain.
func runElected(ctx context.Context, locker Locker, heartbeat time.Duration, onElected func(ctx context.Context) error) error {
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()

	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		t := time.NewTicker(heartbeat)
		defer t.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-workCtx.Done():
				return
			case <-t.C:
				if err := locker.Refresh(workCtx); err != nil {
					// Losing the lease means another replica may take over; cancel
					// the work so we don't continue as a stale leader.
					log.WithError(err).Error("Leader lease refresh failed; cancelling leadership")
					cancelWork()
					return
				}
			}
		}
	}()

	runErr := onElected(workCtx)
	close(stopHeartbeat)
	<-heartbeatDone

	relCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if relErr := locker.Release(relCtx); relErr != nil {
		log.WithError(relErr).Warn("Leader failed to release lock; it will expire via TTL")
	}
	return runErr
}

// PartitionOwner reports whether this replica owns a given work item under a
// simple stable hash partitioning across replicaCount replicas. Each item is
// assigned to exactly one replica index, so N replicas can each process a
// disjoint slice without a coordinator. replicaIndex is 0-based.
//
// A non-positive replicaCount or an out-of-range replicaIndex is treated
// defensively: with replicaCount <= 1 the sole replica owns everything; an
// index >= replicaCount (a misconfiguration) owns nothing rather than panicking.
func PartitionOwner(item string, replicaIndex, replicaCount int) bool {
	if replicaCount <= 1 {
		return true
	}
	if replicaIndex < 0 || replicaIndex >= replicaCount {
		return false
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
