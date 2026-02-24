package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameMatcher_NormalizeAccountName(t *testing.T) {
	tests := []struct {
		name     string
		config   *NameMatchingConfig
		input    string
		expected string
	}{
		{
			name: "default config - case insensitive",
			config: &NameMatchingConfig{
				CaseInsensitive: true,
			},
			input:    "AWS-Analytics-PROD",
			expected: "aws-analytics-prod",
		},
		{
			name: "strip prefix",
			config: &NameMatchingConfig{
				CaseInsensitive: true,
				StripPrefixes:   []string{"aws-", "fsc-"},
			},
			input:    "aws-analytics-prod",
			expected: "analytics-prod",
		},
		{
			name: "strip suffix",
			config: &NameMatchingConfig{
				CaseInsensitive: true,
				StripSuffixes:   []string{"-account", "-acct"},
			},
			input:    "analytics-staging-account",
			expected: "analytics-staging",
		},
		{
			name: "strip both prefix and suffix",
			config: &NameMatchingConfig{
				CaseInsensitive: true,
				StripPrefixes:   []string{"fsc-"},
				StripSuffixes:   []string{"-acct"},
			},
			input:    "fsc-data-engineers-acct",
			expected: "data-engineers",
		},
		{
			name: "JSON key normalization",
			config: &NameMatchingConfig{
				CaseInsensitive: true,
				NormalizeKeys:   true,
			},
			input:    "Data_Engineers_Sandbox",
			expected: "data-engineers-sandbox",
		},
		{
			name: "full normalization pipeline",
			config: &NameMatchingConfig{
				CaseInsensitive: true,
				NormalizeKeys:   true,
				StripPrefixes:   []string{"org-", "aws-"},
				// Note: suffix stripping happens before JSON key normalization,
				// so use the original form with underscore
				StripSuffixes: []string{"_account", "-account"},
			},
			input:    "org-Data_Engineers_Sandbox_Account",
			expected: "data-engineers-sandbox",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewNameMatcher(tc.config)
			result := matcher.NormalizeAccountName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNameMatcher_MatchAccountToTarget(t *testing.T) {
	patterns := []AccountNamePattern{
		{Pattern: ".*-staging$", Target: "Staging"},
		{Pattern: ".*-prod(uction)?$", Target: "Production"},
		{Pattern: ".*-demo$", Target: "Demo"},
		{Pattern: "sandbox", Target: "Sandbox"},
	}

	tests := []struct {
		name           string
		strategy       string
		accountName    string
		expectMatch    bool
		expectedTarget string
	}{
		{
			name:           "exact match staging",
			strategy:       "exact",
			accountName:    "analytics-staging",
			expectMatch:    true,
			expectedTarget: "Staging",
		},
		{
			name:           "exact match production",
			strategy:       "exact",
			accountName:    "data-production",
			expectMatch:    true,
			expectedTarget: "Production",
		},
		{
			name:           "exact match prod shorthand",
			strategy:       "exact",
			accountName:    "analytics-prod",
			expectMatch:    true,
			expectedTarget: "Production",
		},
		{
			name:           "fuzzy match sandbox in middle",
			strategy:       "fuzzy",
			accountName:    "my-sandbox-account",
			expectMatch:    true,
			expectedTarget: "Sandbox",
		},
		{
			name:           "no match",
			strategy:       "exact",
			accountName:    "some-other-account",
			expectMatch:    false,
			expectedTarget: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewNameMatcher(&NameMatchingConfig{
				Strategy:        tc.strategy,
				CaseInsensitive: true,
			})
			target, matched := matcher.MatchAccountToTarget(tc.accountName, patterns)
			assert.Equal(t, tc.expectMatch, matched)
			if matched {
				assert.Equal(t, tc.expectedTarget, target)
			}
		})
	}
}

func TestNameMatcher_ResolveAccountImports(t *testing.T) {
	targetConfigs := map[string]Target{
		"Staging": {
			AccountID: "111111111111",
			Imports:   []string{"analytics", "data-engineers"},
		},
		"Production": {
			AccountID: "222222222222",
			Imports:   []string{"Staging"},
		},
	}

	patterns := []AccountNamePattern{
		{Pattern: ".*-staging$", Target: "Staging"},
		{Pattern: ".*-prod(uction)?$", Target: "Production"},
	}

	defaultImports := []string{"shared", "common"}

	tests := []struct {
		name            string
		accountName     string
		expectedImports []string
	}{
		{
			name:            "matches staging target",
			accountName:     "analytics-staging",
			expectedImports: []string{"analytics", "data-engineers"},
		},
		{
			name:            "matches production target",
			accountName:     "analytics-prod",
			expectedImports: []string{"Staging"},
		},
		{
			name:            "no match - uses defaults",
			accountName:     "some-other-account",
			expectedImports: []string{"shared", "common"},
		},
	}

	matcher := NewNameMatcher(&NameMatchingConfig{
		Strategy:        "exact",
		CaseInsensitive: true,
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acct := AccountInfo{
				ID:   "123456789012",
				Name: tc.accountName,
			}
			imports := matcher.ResolveAccountImports(acct, patterns, defaultImports, targetConfigs)
			assert.Equal(t, tc.expectedImports, imports)
		})
	}
}

func TestNormalizeForJSONKey(t *testing.T) {
	// normalizeForJSONKey replaces underscores with hyphens and removes special chars
	// It does NOT lowercase - that's handled by NameMatcher.CaseInsensitive
	tests := []struct {
		input    string
		expected string
	}{
		{"Data_Engineers", "Data-Engineers"},
		{"PRODUCTION_ACCOUNT", "PRODUCTION-ACCOUNT"},
		{"foo--bar__baz", "foo-bar-baz"},
		{"--leading-and-trailing--", "leading-and-trailing"},
		{"Special@#$Characters!", "SpecialCharacters"},
		{"mixed_Case-With_Stuff", "mixed-Case-With-Stuff"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeForJSONKey(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFilterAccountsByFuzzyMatch(t *testing.T) {
	accounts := []AccountInfo{
		{ID: "111111111111", Name: "analytics-staging"},
		{ID: "222222222222", Name: "analytics-production"},
		{ID: "333333333333", Name: "data-engineers-staging"},
		{ID: "444444444444", Name: "shared-services"},
	}

	matcher := NewNameMatcher(&NameMatchingConfig{
		Strategy:        "fuzzy",
		CaseInsensitive: true,
	})

	tests := []struct {
		name        string
		patterns    []string
		expectedIDs []string
	}{
		{
			name:        "match staging accounts",
			patterns:    []string{"staging"},
			expectedIDs: []string{"111111111111", "333333333333"},
		},
		{
			name:        "match analytics accounts",
			patterns:    []string{"analytics"},
			expectedIDs: []string{"111111111111", "222222222222"},
		},
		{
			name:        "no matches",
			patterns:    []string{"nonexistent"},
			expectedIDs: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FilterAccountsByFuzzyMatch(accounts, matcher, tc.patterns)
			var resultIDs []string
			for _, a := range result {
				resultIDs = append(resultIDs, a.ID)
			}
			assert.ElementsMatch(t, tc.expectedIDs, resultIDs)
		})
	}
}
