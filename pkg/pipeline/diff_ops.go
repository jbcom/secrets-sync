package pipeline

import (
	"context"
	"fmt"

	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/diff"
	log "github.com/sirupsen/logrus"
)

// initDiff initializes diff tracking for a run
func (p *Pipeline) initDiff(dryRun bool, configPath string) {
	p.diffMu.Lock()
	defer p.diffMu.Unlock()
	p.pipelineDiff = &diff.PipelineDiff{
		DryRun:     dryRun,
		ConfigPath: configPath,
	}
}

// addTargetDiff adds a target diff to the pipeline diff
func (p *Pipeline) addTargetDiff(td diff.TargetDiff) {
	p.diffMu.Lock()
	defer p.diffMu.Unlock()
	if p.pipelineDiff != nil {
		p.pipelineDiff.AddTargetDiff(td)
	}
}

// computeMergeDiff computes the diff for a merge operation
func (p *Pipeline) computeMergeDiff(ctx context.Context, targetName string, sourcePaths []string) (*diff.TargetDiff, error) {
	l := log.WithFields(log.Fields{
		"action": "computeMergeDiff",
		"target": targetName,
	})

	// Fetch current state from merge store
	var currentSecrets map[string]interface{}
	var err error

	if p.config.MergeStore.Vault != nil {
		mergePath := fmt.Sprintf("%s/%s", p.config.MergeStore.Vault.Mount, targetName)
		currentSecrets, err = p.fetchVaultSecrets(ctx, mergePath)
		if err != nil {
			l.WithError(err).Debug("Failed to fetch current merge store state")
			currentSecrets = make(map[string]interface{})
		}
	} else if p.s3Store != nil {
		currentSecrets, err = p.fetchS3MergeSecrets(ctx, targetName)
		if err != nil {
			l.WithError(err).Debug("Failed to fetch current S3 merge store state")
			currentSecrets = make(map[string]interface{})
		}
	}

	// Fetch desired state from source paths
	desiredSecrets := make(map[string]interface{})
	for _, sourcePath := range sourcePaths {
		sourceSecrets, err := p.fetchVaultSecrets(ctx, sourcePath)
		if err != nil {
			l.WithError(err).WithField("sourcePath", sourcePath).Debug("Failed to fetch source secrets")
			continue
		}
		for k, v := range sourceSecrets {
			desiredSecrets[k] = v
		}
	}

	changes := diff.DiffSecrets(currentSecrets, desiredSecrets)
	summary := diff.ComputeSummary(changes)

	targetDiff := &diff.TargetDiff{
		Target:  targetName,
		Changes: changes,
		Summary: summary,
	}

	return targetDiff, nil
}

// computeSyncDiff computes the diff for a sync operation
func (p *Pipeline) computeSyncDiff(ctx context.Context, targetName string, roleARN, region string) (*diff.TargetDiff, error) {
	l := log.WithFields(log.Fields{
		"action": "computeSyncDiff",
		"target": targetName,
	})

	currentSecrets, err := p.fetchAWSSecrets(ctx, roleARN, region)
	if err != nil {
		l.WithError(err).Debug("Failed to fetch current AWS state")
		currentSecrets = make(map[string]interface{})
	}

	var desiredSecrets map[string]interface{}
	if p.config.MergeStore.Vault != nil {
		mergePath := fmt.Sprintf("%s/%s", p.config.MergeStore.Vault.Mount, targetName)
		desiredSecrets, err = p.fetchVaultSecrets(ctx, mergePath)
		if err != nil {
			l.WithError(err).Debug("Failed to fetch desired state from merge store")
			desiredSecrets = make(map[string]interface{})
		}
	} else if p.s3Store != nil {
		desiredSecrets, err = p.fetchS3MergeSecrets(ctx, targetName)
		if err != nil {
			l.WithError(err).Debug("Failed to fetch desired state from S3")
			desiredSecrets = make(map[string]interface{})
		}
	}

	changes := diff.DiffSecrets(currentSecrets, desiredSecrets)
	summary := diff.ComputeSummary(changes)

	targetDiff := &diff.TargetDiff{
		Target:  targetName,
		Changes: changes,
		Summary: summary,
	}

	return targetDiff, nil
}

// FormatDiff returns the formatted diff output
func (p *Pipeline) FormatDiff(format diff.OutputFormat) string {
	p.diffMu.Lock()
	defer p.diffMu.Unlock()
	if p.pipelineDiff == nil {
		return ""
	}
	return diff.FormatDiff(p.pipelineDiff, format)
}

// ExitCode returns the appropriate exit code based on diff results
// 0 = no changes (zero-sum), 1 = changes detected, 2 = errors
func (p *Pipeline) ExitCode() int {
	p.diffMu.Lock()
	defer p.diffMu.Unlock()

	p.resultsMu.Lock()
	hasErrors := false
	for _, r := range p.results {
		if !r.Success {
			hasErrors = true
			break
		}
	}
	p.resultsMu.Unlock()

	if hasErrors {
		return 2
	}

	if p.pipelineDiff != nil {
		return p.pipelineDiff.ExitCode()
	}

	return 0
}
