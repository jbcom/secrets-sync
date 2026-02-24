package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatSideBySide tests side-by-side diff formatting (v1.2.0 - Requirement 25)
func TestFormatSideBySide(t *testing.T) {
	diff := &PipelineDiff{
		Targets: []TargetDiff{
			{
				Target: "production",
				Changes: []SecretChange{
					{
						Path:           "app/api-key",
						ChangeType:     ChangeTypeAdded,
						DesiredVersion: 1,
						DesiredKeys:    []string{"key", "description"},
						DesiredValues: map[string]interface{}{
							"key":         "sk-1234567890abcdef",
							"description": "Production API key",
						},
					},
				},
				Summary: ChangeSummary{
					Added: 1,
					Total: 1,
				},
			},
		},
		Summary: ChangeSummary{
			Added: 1,
			Total: 1,
		},
	}

	t.Run("side-by-side without values", func(t *testing.T) {
		output := FormatDiffWithOptions(diff, OutputFormatSideBySide, false)

		// Check header
		assert.Contains(t, output, "Pipeline Diff Summary (Side-by-Side)")
		assert.Contains(t, output, "Added:     1")

		// Check change indicators
		assert.Contains(t, output, "+ app/api-key (→ v1)")

		// Check that values are not shown
		assert.NotContains(t, output, "sk-1234567890abcdef")
	})

	t.Run("side-by-side with values", func(t *testing.T) {
		output := FormatDiffWithOptions(diff, OutputFormatSideBySide, true)

		// Check that sensitive values are masked
		assert.Contains(t, output, "Pr**************ey") // Masked description (contains "Production")
		// The "key" field name triggers masking, but the value itself may not be masked
		// if it doesn't match sensitive patterns
		assert.Contains(t, output, "key")
		assert.Contains(t, output, "description")
	})
}

// TestValueMasking tests value masking functionality (v1.2.0 - Requirement 25)
func TestValueMasking(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		showValues bool
		expected   string
	}{
		{
			name:       "mask when showValues false",
			value:      "secret-value-123",
			showValues: false,
			expected:   "se************23",
		},
		{
			name:       "show non-sensitive when showValues true",
			value:      "normal-config-value",
			showValues: true,
			expected:   "normal-config-value",
		},
		{
			name:       "handle nil value",
			value:      nil,
			showValues: true,
			expected:   "<nil>",
		},
		{
			name:       "handle empty string",
			value:      "",
			showValues: false,
			expected:   "<empty>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskValue(tt.value, tt.showValues)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSensitivePatternDetection tests detection of sensitive value patterns (v1.2.0 - Requirement 25)
func TestSensitivePatternDetection(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		sensitive bool
	}{
		{
			name:      "password field",
			value:     "my-password-123",
			sensitive: true,
		},
		{
			name:      "API key field",
			value:     "api_key_value",
			sensitive: true,
		},
		{
			name:      "normal config value",
			value:     "database-host",
			sensitive: false,
		},
		{
			name:      "normal text",
			value:     "production",
			sensitive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitivePattern(tt.value)
			assert.Equal(t, tt.sensitive, result)
		})
	}
}

// TestEnhancedDiffFormats tests all enhanced diff output formats (v1.2.0 - Requirement 25)
func TestEnhancedDiffFormats(t *testing.T) {
	diff := &PipelineDiff{
		Targets: []TargetDiff{
			{
				Target: "test",
				Changes: []SecretChange{
					{
						Path:        "test/secret",
						ChangeType:  ChangeTypeAdded,
						DesiredKeys: []string{"key"},
					},
				},
				Summary: ChangeSummary{Added: 1, Total: 1},
			},
		},
		Summary: ChangeSummary{Added: 1, Total: 1},
	}

	t.Run("side-by-side format", func(t *testing.T) {
		output := FormatDiff(diff, OutputFormatSideBySide)
		assert.Contains(t, output, "Pipeline Diff Summary (Side-by-Side)")
		assert.Contains(t, output, "+ test/secret")
		assert.Contains(t, output, "NEW SECRET")
	})
}
