package pipeline

import (
	"context"
	"fmt"

	"github.com/jbcom/secrets-sync/pkg/driver"
	log "github.com/sirupsen/logrus"
)

// buildReplicatedStore constructs a ReplicatingBundleStore wrapping the primary
// S3 store with a replica store per configured region. It returns nil when no
// replica regions are configured. A replica that fails to initialize is logged
// and skipped rather than aborting the whole pipeline.
func (p *Pipeline) buildReplicatedStore(ctx context.Context, cfg *MergeStoreS3, primary *S3MergeStore) *ReplicatingBundleStore {
	if cfg == nil || len(cfg.ReplicaRegions) == 0 {
		return nil
	}
	var replicas []driver.BundleStore
	for _, region := range cfg.ReplicaRegions {
		replicaCfg := *cfg
		replicaCfg.ReplicaRegions = nil // avoid recursive replication
		store, err := NewS3MergeStoreWithRuntimeAuth(ctx, &replicaCfg, region, p.runtimeAWSAuth())
		if err != nil {
			log.WithError(err).WithField("region", region).Warn("Failed to init replica bundle store; skipping")
			continue
		}
		replicas = append(replicas, store)
	}
	if len(replicas) == 0 {
		return nil
	}
	rs := NewReplicatingBundleStore(primary, replicas...)
	rs.RequireAllReplicas = cfg.RequireAllReplicas
	return rs
}

// ReplicatingBundleStore writes every merged bundle to a primary bundle store
// and, synchronously, to one or more replica stores in other regions, giving
// cross-region durability and read-locality. Reads come from the primary and
// fall back to replicas in order, so a primary-region outage still serves
// bundles for the sync phase.
type ReplicatingBundleStore struct {
	primary  driver.BundleStore
	replicas []driver.BundleStore
	// RequireAllReplicas, when true, fails a write if any replica write fails.
	// When false (default), replica failures are logged and the write succeeds
	// as long as the primary write succeeds.
	RequireAllReplicas bool
}

// Compile-time assertion.
var _ driver.BundleStore = (*ReplicatingBundleStore)(nil)

// NewReplicatingBundleStore composes a primary with zero or more replicas.
func NewReplicatingBundleStore(primary driver.BundleStore, replicas ...driver.BundleStore) *ReplicatingBundleStore {
	return &ReplicatingBundleStore{primary: primary, replicas: replicas}
}

// WriteMergedBundle writes to the primary first (a primary failure aborts), then
// to each replica.
func (r *ReplicatingBundleStore) WriteMergedBundle(ctx context.Context, targetName, bundleID string, secrets map[string]interface{}) error {
	if err := r.primary.WriteMergedBundle(ctx, targetName, bundleID, secrets); err != nil {
		return fmt.Errorf("replication: primary write: %w", err)
	}
	for i, replica := range r.replicas {
		if err := replica.WriteMergedBundle(ctx, targetName, bundleID, secrets); err != nil {
			if r.RequireAllReplicas {
				return fmt.Errorf("replication: replica %d write: %w", i, err)
			}
			log.WithError(err).WithField("replica", i).Warn("Replica bundle write failed; continuing")
		}
	}
	return nil
}

// ReadMergedBundle reads from the primary, falling back to each replica in order
// on failure (regional outage or missing object).
func (r *ReplicatingBundleStore) ReadMergedBundle(ctx context.Context, targetName, bundleID string) (map[string]map[string]interface{}, error) {
	data, err := r.primary.ReadMergedBundle(ctx, targetName, bundleID)
	if err == nil {
		return data, nil
	}
	lastErr := err
	for i, replica := range r.replicas {
		data, rerr := replica.ReadMergedBundle(ctx, targetName, bundleID)
		if rerr == nil {
			log.WithField("replica", i).Debug("Served bundle from replica after primary miss")
			return data, nil
		}
		lastErr = rerr
	}
	return nil, fmt.Errorf("replication: all stores failed to read bundle, last error: %w", lastErr)
}
