package pipeline

import (
	"context"
	"fmt"

	"github.com/jbcom/secrets-sync/pkg/audit"
	"github.com/jbcom/secrets-sync/pkg/driver"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// rollbackSnapshot captures a target backend's secret values before a sync so
// they can be restored if the sync fails partway. A nil snapshot means rollback
// is disabled or snapshotting was skipped.
type rollbackSnapshot struct {
	// secrets maps secret name -> raw value at snapshot time.
	secrets map[string][]byte
}

// snapshotForRollback reads the current state of every secret in the target
// backend so a failed sync can be reverted. It returns nil (no error) when
// rollback is disabled. The MaxSecrets safety cap aborts snapshotting of an
// unexpectedly large target.
func (p *Pipeline) snapshotForRollback(ctx context.Context, backend driver.TargetBackend) (*rollbackSnapshot, error) {
	cfg := p.config.Pipeline.Rollback
	if !cfg.Enabled {
		return nil, nil
	}

	names, err := backend.ListSecrets(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("snapshot: list secrets: %w", err)
	}
	if cfg.MaxSecrets > 0 && len(names) > cfg.MaxSecrets {
		return nil, fmt.Errorf("snapshot: target holds %d secrets, exceeding rollback max_secrets %d", len(names), cfg.MaxSecrets)
	}

	snap := &rollbackSnapshot{secrets: make(map[string][]byte, len(names))}
	for _, name := range names {
		val, err := backend.GetSecret(ctx, name)
		if err != nil {
			// A partial snapshot is dangerous: an existing secret that we fail to
			// capture would be misclassified as "created during sync" and DELETED
			// during rollback. Fail the snapshot so rollback is skipped entirely
			// rather than acting on incomplete state.
			return nil, fmt.Errorf("snapshot: read secret %q: %w", name, err)
		}
		snap.secrets[name] = val
	}
	return snap, nil
}

// rollback restores a target backend to its pre-sync snapshot: every snapshotted
// secret is rewritten to its captured value. Secrets that did not exist at
// snapshot time (i.e. created by the failed sync) are deleted. A nil snapshot is
// a no-op.
func (p *Pipeline) rollback(ctx context.Context, backend driver.TargetBackend, targetName string, snap *rollbackSnapshot) error {
	if snap == nil {
		return nil
	}
	l := log.WithFields(log.Fields{"action": "rollback", "target": targetName})
	l.WithField("snapshotSize", len(snap.secrets)).Info("Rolling back target to pre-sync snapshot")

	var errs []string

	// Restore captured values.
	for name, val := range snap.secrets {
		meta := metav1.ObjectMeta{Name: name, Namespace: targetName}
		if _, err := backend.WriteSecret(ctx, meta, name, val); err != nil {
			errs = append(errs, fmt.Sprintf("restore %s: %v", name, err))
			continue
		}
		p.audit(ctx, audit.Record{Operation: audit.OpWrite, Driver: string(backend.Driver()), Target: targetName, Secret: name, Success: true, Actor: "rollback"})
	}

	// Delete secrets that were created during the failed sync (present now but
	// not in the snapshot).
	current, err := backend.ListSecrets(ctx, "")
	if err != nil {
		errs = append(errs, fmt.Sprintf("list current state: %v", err))
	} else {
		for _, name := range current {
			if _, existed := snap.secrets[name]; existed {
				continue
			}
			if err := backend.DeleteSecret(ctx, name); err != nil {
				errs = append(errs, fmt.Sprintf("delete created %s: %v", name, err))
				continue
			}
			p.audit(ctx, audit.Record{Operation: audit.OpDelete, Driver: string(backend.Driver()), Target: targetName, Secret: name, Success: true, Actor: "rollback"})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rollback incomplete: %v", errs)
	}
	l.Info("Rollback complete")
	return nil
}
