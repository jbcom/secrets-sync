package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiffSecretsWithVersions tests version-aware diff functionality (v1.2.0 - Requirement 24)
func TestDiffSecretsWithVersions(t *testing.T) {
	tests := []struct {
		name             string
		current          map[string]interface{}
		desired          map[string]interface{}
		currentVersions  map[string]int
		desiredVersions  map[string]int
		expectedChanges  int
		expectedVersions map[string][2]int // [current, desired]
	}{
		{
			name:    "version tracking for new secret",
			current: map[string]interface{}{},
			desired: map[string]interface{}{
				"app/api-key": map[string]interface{}{"key": "value"},
			},
			currentVersions: map[string]int{},
			desiredVersions: map[string]int{
				"app/api-key": 1,
			},
			expectedChanges: 1,
			expectedVersions: map[string][2]int{
				"app/api-key": {0, 1},
			},
		},
		{
			name: "version tracking for modified secret",
			current: map[string]interface{}{
				"app/api-key": map[string]interface{}{"key": "old-value"},
			},
			desired: map[string]interface{}{
				"app/api-key": map[string]interface{}{"key": "new-value"},
			},
			currentVersions: map[string]int{
				"app/api-key": 1,
			},
			desiredVersions: map[string]int{
				"app/api-key": 2,
			},
			expectedChanges: 1,
			expectedVersions: map[string][2]int{
				"app/api-key": {1, 2},
			},
		},
		{
			name: "version tracking for removed secret",
			current: map[string]interface{}{
				"app/api-key": map[string]interface{}{"key": "value"},
			},
			desired: map[string]interface{}{},
			currentVersions: map[string]int{
				"app/api-key": 3,
			},
			desiredVersions: map[string]int{},
			expectedChanges: 1,
			expectedVersions: map[string][2]int{
				"app/api-key": {3, 0},
			},
		},
		{
			name: "version tracking for unchanged secret",
			current: map[string]interface{}{
				"app/api-key": map[string]interface{}{"key": "value"},
			},
			desired: map[string]interface{}{
				"app/api-key": map[string]interface{}{"key": "value"},
			},
			currentVersions: map[string]int{
				"app/api-key": 2,
			},
			desiredVersions: map[string]int{
				"app/api-key": 2,
			},
			expectedChanges: 1,
			expectedVersions: map[string][2]int{
				"app/api-key": {2, 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := DiffSecretsWithVersions(tt.current, tt.desired, tt.currentVersions, tt.desiredVersions)

			assert.Len(t, changes, tt.expectedChanges)

			for _, change := range changes {
				expectedVersion, exists := tt.expectedVersions[change.Path]
				assert.True(t, exists, "Expected version info for path %s", change.Path)
				assert.Equal(t, expectedVersion[0], change.CurrentVersion, "Current version mismatch for %s", change.Path)
				assert.Equal(t, expectedVersion[1], change.DesiredVersion, "Desired version mismatch for %s", change.Path)
			}
		})
	}
}

// TestFormatDiffWithVersions tests version display in diff output (v1.2.0 - Requirement 24)
func TestFormatDiffWithVersions(t *testing.T) {
	diff := &PipelineDiff{
		Targets: []TargetDiff{
			{
				Target: "production",
				Changes: []SecretChange{
					{
						Path:           "app/api-key",
						ChangeType:     ChangeTypeAdded,
						DesiredVersion: 1,
						DesiredKeys:    []string{"key"},
					},
					{
						Path:           "app/db-password",
						ChangeType:     ChangeTypeModified,
						CurrentVersion: 2,
						DesiredVersion: 3,
						KeysModified:   []string{"password"},
					},
					{
						Path:           "app/old-secret",
						ChangeType:     ChangeTypeRemoved,
						CurrentVersion: 5,
					},
				},
				Summary: ChangeSummary{
					Added:    1,
					Modified: 1,
					Removed:  1,
					Total:    3,
				},
			},
		},
		Summary: ChangeSummary{
			Added:    1,
			Modified: 1,
			Removed:  1,
			Total:    3,
		},
	}

	output := FormatDiff(diff, OutputFormatHuman)

	// Check that version information is included
	assert.Contains(t, output, "(v1)", "Should show version for new secret")
	assert.Contains(t, output, "(v2 → v3)", "Should show version transition for modified secret")
	assert.Contains(t, output, "(was v5)", "Should show version for removed secret")
}

// TestBackwardCompatibility tests that diff works without version information (v1.2.0 - Requirement 24)
func TestBackwardCompatibility(t *testing.T) {
	current := map[string]interface{}{
		"app/api-key": map[string]interface{}{"key": "old-value"},
	}
	desired := map[string]interface{}{
		"app/api-key": map[string]interface{}{"key": "new-value"},
	}

	// Test without version information (backward compatibility)
	changes := DiffSecrets(current, desired)
	assert.Len(t, changes, 1)
	assert.Equal(t, ChangeTypeModified, changes[0].ChangeType)
	assert.Equal(t, 0, changes[0].CurrentVersion)
	assert.Equal(t, 0, changes[0].DesiredVersion)

	// Test with nil version maps
	changes = DiffSecretsWithVersions(current, desired, nil, nil)
	assert.Len(t, changes, 1)
	assert.Equal(t, ChangeTypeModified, changes[0].ChangeType)
	assert.Equal(t, 0, changes[0].CurrentVersion)
	assert.Equal(t, 0, changes[0].DesiredVersion)
}
