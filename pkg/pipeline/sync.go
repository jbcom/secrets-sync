package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jbcom/secrets-sync/pkg/audit"
	"github.com/jbcom/secrets-sync/pkg/client/aws"
	reqctx "github.com/jbcom/secrets-sync/pkg/context"
	"github.com/jbcom/secrets-sync/pkg/driver"
	"github.com/jbcom/secrets-sync/pkg/observability"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// syncTarget executes sync operations for a single target.
//
// Sync reads from the merge store bundle (created by merge phase) and writes to AWS.
// The bundle path is deterministic based on the source sequence used during merge,
// so sync always knows where to find the merged secrets.
//
// Flow: MergeStore[bundle_path] → AWS[target_account]
func (p *Pipeline) syncTarget(ctx context.Context, targetName string, dryRun bool) Result {
	start := time.Now()
	ctx, span := observability.StartPhaseSpan(ctx, "sync", targetName)
	defer span.End()
	requestID := reqctx.GetRequestID(ctx)
	l := log.WithFields(log.Fields{
		"action":     "syncTarget",
		"target":     targetName,
		"dryRun":     dryRun,
		"request_id": requestID,
	})

	target, ok := p.config.Targets[targetName]
	if !ok {
		return Result{
			Target:   targetName,
			Phase:    "sync",
			Success:  false,
			Error:    fmt.Errorf("target not found"),
			Duration: time.Since(start),
		}
	}

	// Enforce sync policy before doing any work. A deny for any import→target
	// pair blocks the whole target sync with a clear, actionable error.
	if denied := p.policyDenied(targetName, target); denied != nil {
		l.WithError(denied).Warn("Sync blocked by policy")
		return Result{
			Target:   targetName,
			Phase:    "sync",
			Success:  false,
			Error:    denied,
			Duration: time.Since(start),
		}
	}

	// Get the deterministic bundle path (same calculation as merge phase)
	bundlePath, err := p.GetBundlePath(targetName)
	if err != nil {
		return Result{
			Target:   targetName,
			Phase:    "sync",
			Success:  false,
			Error:    fmt.Errorf("failed to get bundle path: %w", err),
			Duration: time.Since(start),
		}
	}

	l.WithFields(log.Fields{
		"bundlePath": bundlePath,
		"accountId":  target.AccountID,
	}).Info("Starting sync from merge store bundle")

	// Read all secrets from the bundle
	secretsData, err := p.readBundleSecrets(ctx, targetName, bundlePath)
	if err != nil {
		return Result{
			Target:   targetName,
			Phase:    "sync",
			Success:  false,
			Error:    fmt.Errorf("failed to read bundle: %w", err),
			Duration: time.Since(start),
		}
	}

	l.WithField("secretsCount", len(secretsData)).Debug("Retrieved secrets from bundle")

	if dryRun {
		l.WithField("secretsCount", len(secretsData)).Info("[DRY-RUN] Would sync secrets to AWS")
		return Result{
			Target:    targetName,
			Phase:     "sync",
			Operation: string(OperationSync),
			Success:   true,
			Duration:  time.Since(start),
			Details: ResultDetails{
				SecretsProcessed: len(secretsData),
				SourcePaths:      []string{bundlePath},
				DestinationPath:  fmt.Sprintf("aws://%s", target.AccountID),
			},
		}
	}

	// Get role ARN and region for this target
	roleARN := p.getRoleARNForTarget(target)
	region := target.Region
	if region == "" {
		region = p.config.AWS.Region
	}

	// Resolve the target backend (AWS by default, else via the registry).
	targetBackend, err := p.getTargetBackend(ctx, target)
	if err != nil {
		return Result{
			Target:   targetName,
			Phase:    "sync",
			Success:  false,
			Error:    fmt.Errorf("failed to get target backend: %w", err),
			Duration: time.Since(start),
		}
	}

	// Sync each secret to the target backend.
	var syncErrors []string
	successCount := 0

	for secretPath, data := range secretsData {
		// Determine the destination secret name.
		destName := p.getAWSSecretName(targetName, secretPath)

		// Convert data to JSON bytes for the backend.
		secretBytes, err := json.Marshal(data)
		if err != nil {
			l.WithError(err).WithField("secret", secretPath).Error("Failed to marshal secret data")
			syncErrors = append(syncErrors, secretPath)
			continue
		}

		// Create metadata for the write operation.
		meta := metav1.ObjectMeta{
			Name:      destName,
			Namespace: targetName,
		}

		if _, err := targetBackend.WriteSecret(ctx, meta, destName, secretBytes); err != nil {
			l.WithError(err).WithFields(log.Fields{
				"secret":     secretPath,
				"destSecret": destName,
				"driver":     targetBackend.Driver(),
			}).Error("Failed to write secret to target backend")
			p.audit(audit.Record{
				Operation: audit.OpWrite, Driver: string(targetBackend.Driver()),
				Target: targetName, Secret: destName, Success: false, Error: err.Error(),
			})
			syncErrors = append(syncErrors, secretPath)
			continue
		}

		l.WithFields(log.Fields{
			"secret":     secretPath,
			"destSecret": destName,
			"driver":     targetBackend.Driver(),
		}).Debug("Secret synced to target backend")
		p.audit(audit.Record{
			Operation: audit.OpWrite, Driver: string(targetBackend.Driver()),
			Target: targetName, Secret: destName, Success: true,
		})
		successCount++
	}

	success := len(syncErrors) == 0
	var lastErr error
	if !success {
		lastErr = fmt.Errorf("failed to sync %d secrets: %v", len(syncErrors), syncErrors)
	}

	l.WithFields(log.Fields{
		"duration": time.Since(start),
		"success":  success,
		"synced":   successCount,
		"failed":   len(syncErrors),
	}).Info("Sync completed")

	result := Result{
		Target:    targetName,
		Phase:     "sync",
		Operation: string(OperationSync),
		Success:   success,
		Error:     lastErr,
		Duration:  time.Since(start),
		Details: ResultDetails{
			SecretsProcessed: successCount,
			SourcePaths:      []string{bundlePath},
			DestinationPath:  fmt.Sprintf("aws://%s", target.AccountID),
			RoleARN:          roleARN,
		},
	}

	// Compute diff if tracking is enabled
	if p.pipelineDiff != nil {
		targetDiff, err := p.computeSyncDiff(ctx, targetName, roleARN, region)
		if err != nil {
			l.WithError(err).Debug("Failed to compute sync diff")
		} else {
			result.Diff = targetDiff
			p.addTargetDiff(*targetDiff)
		}
	}

	return result
}

// readBundleSecrets reads all secrets from the merge store bundle
func (p *Pipeline) readBundleSecrets(ctx context.Context, targetName, bundlePath string) (map[string]map[string]interface{}, error) {
	secretsData := make(map[string]map[string]interface{})

	if p.config.MergeStore.Vault != nil {
		mergeClient := p.vaultClient("")
		if err := mergeClient.Init(ctx); err != nil {
			return nil, fmt.Errorf("failed to init merge vault client: %w", err)
		}

		secrets, err := mergeClient.ListSecrets(ctx, bundlePath)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets from bundle: %w", err)
		}

		for _, secretPath := range secrets {
			data, err := mergeClient.GetKVSecretOnce(ctx, secretPath)
			if err != nil {
				log.WithError(err).WithField("secret", secretPath).Warn("Failed to read secret from bundle")
				continue
			}
			// Use relative path within bundle
			relPath := secretPath
			if len(secretPath) > len(bundlePath) {
				relPath = secretPath[len(bundlePath):]
				if len(relPath) > 0 && relPath[0] == '/' {
					relPath = relPath[1:]
				}
			}
			secretsData[relPath] = data
		}
	} else if bs := p.bundleStore(); bs != nil {
		target, ok := p.config.Targets[targetName]
		if !ok {
			return nil, fmt.Errorf("target not found: %s", targetName)
		}

		var sourcePaths []string
		for _, importName := range target.Imports {
			sourcePath := p.config.GetSourcePath(importName)
			sourcePaths = append(sourcePaths, sourcePath)
		}
		bundleID := BundleID(sourcePaths)

		data, err := bs.ReadMergedBundle(ctx, targetName, bundleID)
		if err != nil {
			return nil, fmt.Errorf("failed to read bundle from bundle store: %w", err)
		}
		secretsData = data
	}

	return secretsData, nil
}

// getAWSClientForTarget returns an AWS client configured for the target account.
// It handles cross-account role assumption via Control Tower or custom patterns.
func (p *Pipeline) getAWSClientForTarget(ctx context.Context, target Target) (*aws.AwsClient, error) {
	region := target.Region
	if region == "" {
		region = p.config.AWS.Region
	}

	// If we have an AWS execution context with role assumption
	roleArn := p.getRoleARNForTarget(target)
	client := p.awsClient(roleArn, region, "sync-target")

	if err := client.Init(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

// policyDenied evaluates the sync policy for every import→target pair and
// returns a descriptive error if any is denied, or nil when all are allowed.
// When no policy is configured the engine defaults to allow, so this is a
// no-op for users who haven't opted in.
func (p *Pipeline) policyDenied(targetName string, target Target) error {
	if p.policy == nil {
		return nil
	}
	imports := target.Imports
	if len(imports) == 0 {
		// A target with no explicit imports is still subject to a wildcard rule
		// matched against an empty source name.
		if d := p.policy.Evaluate("", targetName); !d.Allowed {
			return fmt.Errorf("sync to target %q denied by policy rule %q", targetName, d.Rule)
		}
		return nil
	}
	for _, source := range imports {
		if d := p.policy.Evaluate(source, targetName); !d.Allowed {
			return fmt.Errorf("sync of source %q to target %q denied by policy rule %q", source, targetName, d.Rule)
		}
	}
	return nil
}

// getTargetBackend resolves the sync destination for a target. When no explicit
// backend is configured (or it names "aws"), it returns the AWS client built by
// getAWSClientForTarget, preserving cross-account role assumption. Otherwise it
// constructs the backend through the registry from the target's backend config.
func (p *Pipeline) getTargetBackend(ctx context.Context, target Target) (driver.TargetBackend, error) {
	if target.Backend == nil || target.Backend.Driver == "" || target.Backend.Driver == string(driver.DriverNameAws) {
		return p.getAWSClientForTarget(ctx, target)
	}

	spec := driver.BackendSpec{
		Driver:  driver.DriverName(target.Backend.Driver),
		Path:    target.Backend.Path,
		Options: target.Backend.Options,
	}
	backend, err := p.backends.NewTarget(spec)
	if err != nil {
		return nil, err
	}
	if err := backend.Init(ctx); err != nil {
		return nil, fmt.Errorf("init %s target backend: %w", spec.Driver, err)
	}
	return backend, nil
}

// getRoleARNForTarget returns the role ARN for assuming into the target account
func (p *Pipeline) getRoleARNForTarget(target Target) string {
	if target.AccountID == "" {
		return ""
	}

	// Use custom role pattern if provided
	if p.awsCtx != nil && p.config.AWS.ExecutionContext.CustomRolePattern != "" {
		return fmt.Sprintf(p.config.AWS.ExecutionContext.CustomRolePattern, target.AccountID)
	}

	// Use Control Tower execution role if enabled
	if p.config.AWS.ControlTower.Enabled {
		roleName := p.config.AWS.ControlTower.ExecutionRole.Name
		if roleName == "" {
			roleName = "AWSControlTowerExecution"
		}
		return fmt.Sprintf("arn:aws:iam::%s:role/%s", target.AccountID, roleName)
	}

	return ""
}

// getAWSSecretName determines the AWS Secrets Manager secret name for a given path.
func (p *Pipeline) getAWSSecretName(targetName, secretPath string) string {
	// Default: use the secret path as-is
	// Could be customized via target config or naming patterns
	return secretPath
}
