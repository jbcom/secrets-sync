package pipeline

import (
	"context"
	"testing"

	"github.com/extended-data-library/secretssync/pkg/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiffIntegration_InitAndTracking tests that diff tracking is initialized and working
func TestDiffIntegration_InitAndTracking(t *testing.T) {
	// Create a minimal pipeline config
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://vault.example.com",
			Namespace: "test",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sources: map[string]Source{
			"source1": {
				Vault: &VaultSource{
					Mount: "kv/source1",
				},
			},
		},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "kv/merged",
			},
		},
		Targets: map[string]Target{
			"target1": {
				AccountID: "123456789012",
				Imports:   []string{"source1"},
			},
		},
		Pipeline: PipelineSettings{
			Merge: MergeSettings{Parallel: 1},
			Sync:  SyncSettings{Parallel: 1},
		},
	}

	// Create pipeline
	p, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Initialize diff tracking
	p.initDiff(true, "/tmp/test-config.yaml")

	// Verify diff tracking is initialized
	assert.NotNil(t, p.pipelineDiff)
	assert.True(t, p.pipelineDiff.DryRun)
	assert.Equal(t, "/tmp/test-config.yaml", p.pipelineDiff.ConfigPath)
}

// TestDiffIntegration_AddTargetDiff tests that addTargetDiff works correctly
func TestDiffIntegration_AddTargetDiff(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://vault.example.com",
			Namespace: "test",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sources: map[string]Source{},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "kv/merged",
			},
		},
		Targets: map[string]Target{
			"dummy": {AccountID: "123456789012", Imports: []string{}},
		},
		Pipeline: PipelineSettings{
			Merge: MergeSettings{Parallel: 1},
			Sync:  SyncSettings{Parallel: 1},
		},
	}

	p, err := New(cfg)
	require.NoError(t, err)

	// Initialize diff tracking
	p.initDiff(true, "")

	// Create a test target diff
	targetDiff := diff.TargetDiff{
		Target: "test-target",
		Changes: []diff.SecretChange{
			{
				Path:       "secret1",
				ChangeType: diff.ChangeTypeAdded,
			},
			{
				Path:       "secret2",
				ChangeType: diff.ChangeTypeModified,
			},
		},
		Summary: diff.ChangeSummary{
			Added:    1,
			Modified: 1,
			Total:    2,
		},
	}

	// Add the target diff
	p.addTargetDiff(targetDiff)

	// Verify the diff was added
	pipelineDiff := p.Diff()
	require.NotNil(t, pipelineDiff)
	assert.Len(t, pipelineDiff.Targets, 1)
	assert.Equal(t, "test-target", pipelineDiff.Targets[0].Target)
	assert.Equal(t, 1, pipelineDiff.Summary.Added)
	assert.Equal(t, 1, pipelineDiff.Summary.Modified)
	assert.Equal(t, 2, pipelineDiff.Summary.Total)
}

// TestDiffIntegration_MultipleTargets tests adding diffs from multiple targets
func TestDiffIntegration_MultipleTargets(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://vault.example.com",
			Namespace: "test",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sources: map[string]Source{},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "kv/merged",
			},
		},
		Targets: map[string]Target{
			"dummy": {AccountID: "123456789012", Imports: []string{}},
		},
		Pipeline: PipelineSettings{
			Merge: MergeSettings{Parallel: 1},
			Sync:  SyncSettings{Parallel: 1},
		},
	}

	p, err := New(cfg)
	require.NoError(t, err)

	// Initialize diff tracking
	p.initDiff(false, "")

	// Add diffs for multiple targets
	p.addTargetDiff(diff.TargetDiff{
		Target: "target1",
		Changes: []diff.SecretChange{
			{Path: "s1", ChangeType: diff.ChangeTypeAdded},
		},
		Summary: diff.ChangeSummary{Added: 1, Total: 1},
	})

	p.addTargetDiff(diff.TargetDiff{
		Target: "target2",
		Changes: []diff.SecretChange{
			{Path: "s2", ChangeType: diff.ChangeTypeRemoved},
			{Path: "s3", ChangeType: diff.ChangeTypeModified},
		},
		Summary: diff.ChangeSummary{Removed: 1, Modified: 1, Total: 2},
	})

	// Verify aggregated summary
	pipelineDiff := p.Diff()
	require.NotNil(t, pipelineDiff)
	assert.Len(t, pipelineDiff.Targets, 2)
	assert.Equal(t, 1, pipelineDiff.Summary.Added)
	assert.Equal(t, 1, pipelineDiff.Summary.Removed)
	assert.Equal(t, 1, pipelineDiff.Summary.Modified)
	assert.Equal(t, 3, pipelineDiff.Summary.Total)
}

// TestDiffIntegration_ZeroSum tests zero-sum validation
func TestDiffIntegration_ZeroSum(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://vault.example.com",
			Namespace: "test",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sources: map[string]Source{},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "kv/merged",
			},
		},
		Targets: map[string]Target{
			"dummy": {AccountID: "123456789012", Imports: []string{}},
		},
		Pipeline: PipelineSettings{
			Merge: MergeSettings{Parallel: 1},
			Sync:  SyncSettings{Parallel: 1},
		},
	}

	p, err := New(cfg)
	require.NoError(t, err)

	// Initialize diff tracking
	p.initDiff(true, "")

	// Add a zero-sum diff (only unchanged secrets)
	p.addTargetDiff(diff.TargetDiff{
		Target: "target1",
		Changes: []diff.SecretChange{
			{Path: "s1", ChangeType: diff.ChangeTypeUnchanged},
			{Path: "s2", ChangeType: diff.ChangeTypeUnchanged},
		},
		Summary: diff.ChangeSummary{Unchanged: 2, Total: 2},
	})

	// Verify zero-sum
	pipelineDiff := p.Diff()
	require.NotNil(t, pipelineDiff)
	assert.True(t, pipelineDiff.IsZeroSum())
	assert.Equal(t, 0, pipelineDiff.ExitCode())
}

// TestDiffIntegration_ExitCodes tests exit code logic
func TestDiffIntegration_ExitCodes(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://vault.example.com",
			Namespace: "test",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sources: map[string]Source{},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "kv/merged",
			},
		},
		Targets: map[string]Target{
			"dummy": {AccountID: "123456789012", Imports: []string{}},
		},
		Pipeline: PipelineSettings{
			Merge: MergeSettings{Parallel: 1},
			Sync:  SyncSettings{Parallel: 1},
		},
	}

	t.Run("ExitCode 0 for zero-sum", func(t *testing.T) {
		p, err := New(cfg)
		require.NoError(t, err)
		p.initDiff(true, "")

		p.addTargetDiff(diff.TargetDiff{
			Target:  "target1",
			Changes: []diff.SecretChange{},
			Summary: diff.ChangeSummary{},
		})

		assert.Equal(t, 0, p.ExitCode())
	})

	t.Run("ExitCode 1 for changes detected", func(t *testing.T) {
		p, err := New(cfg)
		require.NoError(t, err)
		p.initDiff(true, "")

		p.addTargetDiff(diff.TargetDiff{
			Target: "target1",
			Changes: []diff.SecretChange{
				{Path: "s1", ChangeType: diff.ChangeTypeAdded},
			},
			Summary: diff.ChangeSummary{Added: 1, Total: 1},
		})

		assert.Equal(t, 1, p.ExitCode())
	})

	t.Run("ExitCode 2 for errors", func(t *testing.T) {
		p, err := New(cfg)
		require.NoError(t, err)
		p.initDiff(true, "")

		// Add a failed result
		p.resultsMu.Lock()
		p.results = []Result{
			{
				Target:  "target1",
				Success: false,
				Error:   assert.AnError,
			},
		}
		p.resultsMu.Unlock()

		assert.Equal(t, 2, p.ExitCode())
	})
}

// TestFetchSecretsHelpers_Errors tests error handling in fetch methods
func TestFetchSecretsHelpers_Errors(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://nonexistent.vault.example.com",
			Namespace: "test",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Sources: map[string]Source{},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "kv/merged",
			},
		},
		Targets: map[string]Target{
			"dummy": {AccountID: "123456789012", Imports: []string{}},
		},
		Pipeline: PipelineSettings{
			Merge: MergeSettings{Parallel: 1},
			Sync:  SyncSettings{Parallel: 1},
		},
	}

	p, err := New(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Test fetchVaultSecrets with invalid vault
	t.Run("fetchVaultSecrets handles errors gracefully", func(t *testing.T) {
		secrets, _ := p.fetchVaultSecrets(ctx, "kv/nonexistent")
		// Should return empty map on error, not fail
		assert.NotNil(t, secrets)
	})

	// Test fetchAWSSecrets with invalid credentials
	t.Run("fetchAWSSecrets handles errors gracefully", func(t *testing.T) {
		_, err := p.fetchAWSSecrets(ctx, "arn:aws:iam::000000000000:role/NonExistent", "us-east-1")
		// Should handle errors gracefully
		assert.NotNil(t, err)
	})
}
