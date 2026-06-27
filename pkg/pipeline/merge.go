package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jbcom/secrets-sync/pkg/client/vault"
	reqctx "github.com/jbcom/secrets-sync/pkg/context"
	"github.com/jbcom/secrets-sync/pkg/observability"
	"github.com/jbcom/secrets-sync/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// mergeTarget executes merge operations for a single target.
//
// Merge is a two-phase operation:
// 1. Read secrets from N sources in sequence (order determines deepmerge priority)
// 2. Write merged result as JSON blob to deterministic path in merge store
//
// The merge store path is deterministic based on source sequence checksum,
// so the same sources in the same order always produce the same path.
// Existing data at that path is wiped before writing.
func (p *Pipeline) mergeTarget(ctx context.Context, targetName string, dryRun bool) Result {
	start := time.Now()
	ctx, span := observability.StartPhaseSpan(ctx, "merge", targetName)
	defer span.End()
	requestID := reqctx.SafeRequestID(ctx)
	l := log.WithFields(log.Fields{
		"action":     "mergeTarget",
		"target":     targetName,
		"dryRun":     dryRun,
		"request_id": requestID,
	})

	target, ok := p.config.Targets[targetName]
	if !ok {
		return Result{
			Target:   targetName,
			Phase:    "merge",
			Success:  false,
			Error:    fmt.Errorf("target not found"),
			Duration: time.Since(start),
		}
	}

	// Build source paths in order (order determines merge priority)
	var sourcePaths []string
	for _, importName := range target.Imports {
		sourcePath := p.config.GetSourcePath(importName)
		sourcePaths = append(sourcePaths, sourcePath)
	}

	// Calculate deterministic bundle path based on source sequence
	var bundlePath string
	var bundleID string
	if p.config.MergeStore.Vault != nil {
		bundleID = BundleID(sourcePaths)
		bundlePath = TargetBundlePath(p.config.MergeStore.Vault.Mount, targetName, sourcePaths)
	} else if p.s3Store != nil {
		bundleID = BundleID(sourcePaths)
		bundlePath = p.s3Store.GetBundlePath(targetName, bundleID)
	} else {
		return Result{
			Target:   targetName,
			Phase:    "merge",
			Success:  false,
			Error:    fmt.Errorf("no merge store configured"),
			Duration: time.Since(start),
		}
	}

	l.WithFields(log.Fields{
		"bundlePath": bundlePath,
		"bundleID":   bundleID,
		"sources":    sourcePaths,
	}).Info("Starting merge")

	// Initialize Vault client for reading sources
	sourceClient := p.vaultClient("")
	if err := sourceClient.Init(ctx); err != nil {
		return Result{
			Target:   targetName,
			Phase:    "merge",
			Success:  false,
			Error:    fmt.Errorf("failed to init source vault client: %w", err),
			Duration: time.Since(start),
		}
	}

	// Read every source concurrently (bounded by pipeline.merge.parallel), then
	// merge the per-source results in priority order. Reading is the slow,
	// I/O-bound step and parallelizes cleanly; the deep-merge must stay
	// sequential because later sources override earlier ones.
	sourceResults, failedSources := p.readSourcesConcurrently(ctx, sourceClient, sourcePaths)

	mergedSecrets := make(map[string]interface{})
	for _, sr := range sourceResults {
		for _, relPath := range sr.order {
			secretData := sr.secrets[relPath]
			if existing, ok := mergedSecrets[relPath]; ok {
				if existingMap, ok := existing.(map[string]interface{}); ok {
					mergedSecrets[relPath] = utils.DeepMerge(existingMap, secretData)
				} else {
					// Not a map, just override
					mergedSecrets[relPath] = secretData
				}
			} else {
				mergedSecrets[relPath] = secretData
			}
		}
	}

	l.WithField("secretsCount", len(mergedSecrets)).Debug("Merge complete, writing to store")

	if dryRun {
		l.WithFields(log.Fields{
			"secretsCount": len(mergedSecrets),
			"bundlePath":   bundlePath,
		}).Info("[DRY-RUN] Would write merged bundle")
		return Result{
			Target:    targetName,
			Phase:     "merge",
			Operation: string(OperationMerge),
			Success:   true,
			Duration:  time.Since(start),
			Details: ResultDetails{
				SecretsProcessed: len(mergedSecrets),
				SourcePaths:      sourcePaths,
				DestinationPath:  bundlePath,
			},
		}
	}

	// Refuse to persist a partial bundle: if any source failed to list/read, the
	// merged result is incomplete and writing it would silently overwrite a
	// previously-good bundle with missing secrets. Fail the merge instead.
	if len(failedSources) > 0 {
		return Result{
			Target:   targetName,
			Phase:    "merge",
			Success:  false,
			Error:    fmt.Errorf("refusing to write partial bundle: failed to read %d sources: %v", len(failedSources), failedSources),
			Duration: time.Since(start),
			Details:  ResultDetails{FailedImports: failedSources},
		}
	}

	// Write to merge store. The Vault path is path-based and legacy; the bundle
	// store path goes through the driver.BundleStore interface.
	var writeErr error
	if p.config.MergeStore.Vault != nil {
		writeErr = p.writeMergedBundleToVault(ctx, bundlePath, mergedSecrets)
	} else if bs := p.bundleStore(); bs != nil {
		writeErr = bs.WriteMergedBundle(ctx, targetName, bundleID, mergedSecrets)
	}

	if writeErr != nil {
		return Result{
			Target:   targetName,
			Phase:    "merge",
			Success:  false,
			Error:    fmt.Errorf("failed to write merged bundle: %w", writeErr),
			Duration: time.Since(start),
		}
	}

	success := len(failedSources) == 0
	var lastErr error
	if !success {
		lastErr = fmt.Errorf("failed to read from %d sources: %v", len(failedSources), failedSources)
	}

	l.WithFields(log.Fields{
		"duration":      time.Since(start),
		"success":       success,
		"bundlePath":    bundlePath,
		"secretsCount":  len(mergedSecrets),
		"failedSources": failedSources,
	}).Info("Merge completed")

	result := Result{
		Target:    targetName,
		Phase:     "merge",
		Operation: string(OperationMerge),
		Success:   success,
		Error:     lastErr,
		Duration:  time.Since(start),
		Details: ResultDetails{
			SecretsProcessed: len(mergedSecrets),
			SourcePaths:      sourcePaths,
			DestinationPath:  bundlePath,
			FailedImports:    failedSources,
		},
	}

	// Compute diff if tracking is enabled
	if p.pipelineDiff != nil {
		targetDiff, err := p.computeMergeDiff(ctx, targetName, sourcePaths)
		if err != nil {
			l.WithError(err).Debug("Failed to compute merge diff")
		} else {
			result.Diff = targetDiff
			p.addTargetDiff(*targetDiff)
		}
	}

	return result
}

// sourceReadResult holds one source's secrets plus the order they were listed
// in, so the sequential merge can preserve deterministic per-source ordering.
type sourceReadResult struct {
	secrets map[string]map[string]interface{}
	order   []string
}

// mergeParallelism returns the configured concurrent-source-read limit,
// defaulting to 4 and never exceeding the number of sources.
func (p *Pipeline) mergeParallelism(n int) int {
	limit := p.config.Pipeline.Merge.Parallel
	if limit <= 0 {
		limit = 4
	}
	if limit > n {
		limit = n
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

// readSourcesConcurrently reads every source path concurrently (bounded by the
// merge parallelism) and returns per-source results in the SAME order as
// sourcePaths, so the caller can merge in priority order. Source-level failures
// are collected and reported rather than aborting the whole merge.
func (p *Pipeline) readSourcesConcurrently(ctx context.Context, sourceClient *vault.VaultClient, sourcePaths []string) ([]sourceReadResult, []string) {
	results := make([]sourceReadResult, len(sourcePaths))
	failures := make([]string, len(sourcePaths))
	sem := make(chan struct{}, p.mergeParallelism(len(sourcePaths)))

	var wg sync.WaitGroup
	for i, sourcePath := range sourcePaths {
		wg.Add(1)
		go func(i int, sourcePath string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			l := log.WithFields(log.Fields{"source": sourcePath, "priority": i})
			// Use the no-reauth ListSecretsOnce: the shared sourceClient was
			// already authenticated by Init() before these goroutines launched,
			// so calling ListSecrets here would trigger N concurrent re-logins
			// against Vault for no benefit.
			secrets, err := sourceClient.ListSecretsOnce(ctx, sourcePath)
			if err != nil {
				l.WithError(err).Warn("Failed to list secrets from source")
				failures[i] = sourcePath
				return
			}

			res := sourceReadResult{secrets: map[string]map[string]interface{}{}}
			for _, secretPath := range secrets {
				secretData, err := sourceClient.GetKVSecretOnce(ctx, secretPath)
				if err != nil {
					// A secret that was listed but cannot be read means this
					// source's contribution is incomplete; mark the whole source
					// failed so the caller refuses to write a partial bundle
					// rather than silently dropping the secret.
					l.WithError(err).WithField("secret", secretPath).Warn("Failed to read secret; marking source failed")
					failures[i] = sourcePath
					return
				}
				relPath := secretPath
				if len(secretPath) > len(sourcePath) {
					relPath = secretPath[len(sourcePath):]
					if len(relPath) > 0 && relPath[0] == '/' {
						relPath = relPath[1:]
					}
				}
				if _, seen := res.secrets[relPath]; !seen {
					res.order = append(res.order, relPath)
				}
				res.secrets[relPath] = secretData
			}
			results[i] = res
		}(i, sourcePath)
	}
	wg.Wait()

	var failedSources []string
	for _, f := range failures {
		if f != "" {
			failedSources = append(failedSources, f)
		}
	}
	return results, failedSources
}

// writeMergedBundleToVault writes the merged secrets to Vault, wiping existing data first
func (p *Pipeline) writeMergedBundleToVault(ctx context.Context, bundlePath string, secrets map[string]interface{}) error {
	l := log.WithFields(log.Fields{
		"action":     "writeMergedBundleToVault",
		"bundlePath": bundlePath,
	})

	mergeClient := p.vaultClient("")
	if err := mergeClient.Init(ctx); err != nil {
		return fmt.Errorf("failed to init merge vault client: %w", err)
	}

	// Wipe existing bundle at this path
	existingSecrets, err := mergeClient.ListSecrets(ctx, bundlePath)
	if err == nil && len(existingSecrets) > 0 {
		l.WithField("existingCount", len(existingSecrets)).Debug("Wiping existing bundle")
		for _, secretPath := range existingSecrets {
			if err := mergeClient.DeleteSecret(ctx, secretPath); err != nil {
				l.WithError(err).WithField("secret", secretPath).Warn("Failed to delete existing secret")
			}
		}
	}

	// Write each merged secret
	for relPath, data := range secrets {
		fullPath := fmt.Sprintf("%s/%s", bundlePath, relPath)

		secretData, ok := data.(map[string]interface{})
		if !ok {
			l.WithField("path", relPath).Warn("Secret data is not a map, skipping")
			continue
		}

		if _, err := mergeClient.WriteSecretOnce(ctx, fullPath, secretData, nil); err != nil {
			return fmt.Errorf("failed to write secret %s: %w", fullPath, err)
		}
	}

	l.WithField("secretsWritten", len(secrets)).Debug("Bundle written to Vault")
	return nil
}

// GetBundlePath returns the current bundle path for a target (for sync phase to use)
func (p *Pipeline) GetBundlePath(targetName string) (string, error) {
	target, ok := p.config.Targets[targetName]
	if !ok {
		return "", fmt.Errorf("target not found: %s", targetName)
	}

	var sourcePaths []string
	for _, importName := range target.Imports {
		sourcePath := p.config.GetSourcePath(importName)
		sourcePaths = append(sourcePaths, sourcePath)
	}

	if p.config.MergeStore.Vault != nil {
		return TargetBundlePath(p.config.MergeStore.Vault.Mount, targetName, sourcePaths), nil
	} else if p.s3Store != nil {
		bundleID := BundleID(sourcePaths)
		return p.s3Store.GetBundlePath(targetName, bundleID), nil
	}

	return "", fmt.Errorf("no merge store configured")
}
