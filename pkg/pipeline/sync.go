package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/extended-data-library/secretssync/pkg/client/aws"
	"github.com/extended-data-library/secretssync/pkg/client/vault"
	reqctx "github.com/extended-data-library/secretssync/pkg/context"
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

	// Initialize AWS client for target account
	awsClient, err := p.getAWSClientForTarget(ctx, target)
	if err != nil {
		return Result{
			Target:   targetName,
			Phase:    "sync",
			Success:  false,
			Error:    fmt.Errorf("failed to get AWS client for target: %w", err),
			Duration: time.Since(start),
		}
	}

	// Sync each secret to AWS
	var syncErrors []string
	successCount := 0

	for secretPath, data := range secretsData {
		// Determine AWS secret name
		awsSecretName := p.getAWSSecretName(targetName, secretPath)

		// Convert data to JSON bytes for AWS
		secretBytes, err := json.Marshal(data)
		if err != nil {
			l.WithError(err).WithField("secret", secretPath).Error("Failed to marshal secret data")
			syncErrors = append(syncErrors, secretPath)
			continue
		}

		// Create metadata for the write operation
		meta := metav1.ObjectMeta{
			Name:      awsSecretName,
			Namespace: targetName,
		}

		if _, err := awsClient.WriteSecret(ctx, meta, awsSecretName, secretBytes); err != nil {
			l.WithError(err).WithFields(log.Fields{
				"secret":    secretPath,
				"awsSecret": awsSecretName,
			}).Error("Failed to write secret to AWS")
			syncErrors = append(syncErrors, secretPath)
			continue
		}

		l.WithFields(log.Fields{
			"secret":    secretPath,
			"awsSecret": awsSecretName,
		}).Debug("Secret synced to AWS")
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
		mergeClient := &vault.VaultClient{
			Address:   p.config.Vault.Address,
			Namespace: p.config.Vault.Namespace,
		}
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
	} else if p.s3Store != nil {
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

		data, err := p.s3Store.ReadMergedBundle(ctx, targetName, bundleID)
		if err != nil {
			return nil, fmt.Errorf("failed to read bundle from S3: %w", err)
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

	client := &aws.AwsClient{
		Region: region,
	}

	// If we have an AWS execution context with role assumption
	roleArn := p.getRoleARNForTarget(target)
	if roleArn != "" {
		client.RoleArn = roleArn
	}

	if err := client.Init(ctx); err != nil {
		return nil, err
	}

	return client, nil
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
