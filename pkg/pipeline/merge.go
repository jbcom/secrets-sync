package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/client/vault"
	reqctx "github.com/jbcom/extended-data-library/packages/secretssync/pkg/context"
	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/utils"
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
	requestID := reqctx.GetRequestID(ctx)
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
	sourceClient := &vault.VaultClient{
		Address:   p.config.Vault.Address,
		Namespace: p.config.Vault.Namespace,
	}
	if err := sourceClient.Init(ctx); err != nil {
		return Result{
			Target:   targetName,
			Phase:    "merge",
			Success:  false,
			Error:    fmt.Errorf("failed to init source vault client: %w", err),
			Duration: time.Since(start),
		}
	}

	// Merge all sources in sequence (later sources override earlier)
	mergedSecrets := make(map[string]interface{})
	var failedSources []string

	for i, sourcePath := range sourcePaths {
		l.WithFields(log.Fields{
			"source":   sourcePath,
			"priority": i,
		}).Debug("Processing source")

		// List all secrets in this source
		secrets, err := sourceClient.ListSecrets(ctx, sourcePath)
		if err != nil {
			l.WithError(err).WithField("source", sourcePath).Warn("Failed to list secrets from source")
			failedSources = append(failedSources, sourcePath)
			continue
		}

		// Read and merge each secret
		for _, secretPath := range secrets {
			secretData, err := sourceClient.GetKVSecretOnce(ctx, secretPath)
			if err != nil {
				l.WithError(err).WithField("secret", secretPath).Warn("Failed to read secret")
				continue
			}

			// Relative path within this source
			relPath := secretPath
			if len(secretPath) > len(sourcePath) {
				relPath = secretPath[len(sourcePath):]
				if len(relPath) > 0 && relPath[0] == '/' {
					relPath = relPath[1:]
				}
			}

			// Deep merge into accumulated result (later sources win on conflict)
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

	// Write to merge store
	var writeErr error
	if p.config.MergeStore.Vault != nil {
		writeErr = p.writeMergedBundleToVault(ctx, bundlePath, mergedSecrets)
	} else if p.s3Store != nil {
		writeErr = p.s3Store.WriteMergedBundle(ctx, targetName, bundleID, mergedSecrets)
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

// writeMergedBundleToVault writes the merged secrets to Vault, wiping existing data first
func (p *Pipeline) writeMergedBundleToVault(ctx context.Context, bundlePath string, secrets map[string]interface{}) error {
	l := log.WithFields(log.Fields{
		"action":     "writeMergedBundleToVault",
		"bundlePath": bundlePath,
	})

	mergeClient := &vault.VaultClient{
		Address:   p.config.Vault.Address,
		Namespace: p.config.Vault.Namespace,
	}
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
