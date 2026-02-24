package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceResolver_Resolve(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		AWS: AWSConfig{Region: "us-east-1"},
		Sources: map[string]Source{
			"analytics": {Vault: &VaultSource{Mount: "analytics"}},
		},
	}

	resolver := NewResourceResolver(ctx, cfg)

	// Manually populate AWS accounts for testing
	resolver.awsAccounts = []AccountInfo{
		{ID: "111111111111", Name: "analytics-staging"},
		{ID: "222222222222", Name: "analytics-production"},
		{ID: "333333333333", Name: "AWS-DataEngineering-Sandbox-Account"},
		{ID: "444444444444", Name: "fsc-data-engineers-acct"},
		{ID: "555555555555", Name: "org_Platform_Services"},
	}

	// Build indexes
	for _, acct := range resolver.awsAccounts {
		resolver.awsAccountsById[acct.ID] = acct
		normalized := resolver.nameMatcher.NormalizeAccountName(acct.Name)
		resolver.awsAccountsByNormalizedName[normalized] = acct
		resolver.awsAccountsByNormalizedName[acct.Name] = acct
	}

	tests := []struct {
		name         string
		input        string
		expectedType ResourceType
		expectedID   string
		expectedConf MatchConfidence
	}{
		{
			name:         "exact AWS account ID",
			input:        "111111111111",
			expectedType: ResourceTypeAWSAccount,
			expectedID:   "111111111111",
			expectedConf: MatchAccountID,
		},
		{
			name:         "exact AWS account name",
			input:        "analytics-staging",
			expectedType: ResourceTypeAWSAccount,
			expectedID:   "111111111111",
			expectedConf: MatchExact,
		},
		{
			name:         "case-insensitive exact match",
			input:        "Analytics-Staging",
			expectedType: ResourceTypeAWSAccount,
			expectedID:   "111111111111",
			expectedConf: MatchExact,
		},
		{
			name:         "normalized match - underscores to hyphens",
			input:        "org-platform-services",
			expectedType: ResourceTypeAWSAccount,
			expectedID:   "555555555555",
			// Matches via normalized name in the map (which includes exact matches too)
			expectedConf: MatchExact,
		},
		{
			name:         "fuzzy match - partial name",
			input:        "data-engineers",
			expectedType: ResourceTypeAWSAccount,
			expectedID:   "444444444444",
			expectedConf: MatchFuzzy,
		},
		{
			name:         "normalized match - prefix/suffix stripped",
			input:        "dataengineering-sandbox",
			expectedType: ResourceTypeAWSAccount,
			expectedID:   "333333333333",
			// "AWS-DataEngineering-Sandbox-Account" normalizes to "dataengineering-sandbox"
			// so this is an exact match on the normalized form
			expectedConf: MatchExact,
		},
		{
			name:         "vault mount fallback",
			input:        "shared-secrets",
			expectedType: ResourceTypeVaultMount,
			expectedConf: MatchExact,
		},
		{
			name:         "vault mount - known source",
			input:        "my-vault-mount",
			expectedType: ResourceTypeVaultMount,
			expectedConf: MatchExact,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolver.Resolve(tc.input)
			assert.Equal(t, tc.expectedType, result.Type, "Type mismatch")
			if tc.expectedType == ResourceTypeAWSAccount {
				assert.Equal(t, tc.expectedID, result.AccountID, "Account ID mismatch")
			}
			assert.Equal(t, tc.expectedConf, result.MatchConfidence, "Confidence mismatch")
		})
	}
}

func TestResourceResolver_FuzzyMatch(t *testing.T) {
	resolver := &ResourceResolver{
		nameMatcher: NewNameMatcher(&NameMatchingConfig{
			Strategy:        "fuzzy",
			CaseInsensitive: true,
			NormalizeKeys:   true,
		}),
	}

	tests := []struct {
		name     string
		input    string
		target   string
		expected bool
	}{
		// Exact matches
		{"exact match", "analytics", "analytics", true},
		{"case insensitive", "Analytics", "analytics", true},

		// Substring matches
		{"input substring of target", "staging", "analytics-staging", true},
		{"target substring of input", "analytics-staging", "staging", true},

		// Token-based matches
		{"token overlap - 2/3", "analytics-prod", "analytics-production-east", true},
		{"token overlap - hyphen vs underscore", "data-engineers", "data_engineers", true},

		// Non-matches
		{"no overlap", "billing", "analytics", false},
		// Note: "ana" matches "analytics" via substring - that's correct fuzzy behavior
		{"completely different", "xyz123", "analytics", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputNorm := resolver.nameMatcher.NormalizeAccountName(tc.input)
			targetNorm := resolver.nameMatcher.NormalizeAccountName(tc.target)
			result := resolver.fuzzyMatch(inputNorm, targetNorm)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"analytics-staging", []string{"analytics", "staging"}},
		{"AWS_DataEngineering_Account", []string{"aws", "dataengineering", "account"}},
		{"org.platform.services", []string{"org", "platform", "services"}},
		{"foo-bar_baz.qux", []string{"foo", "bar", "baz", "qux"}},
		{"a-b", nil}, // Single char tokens filtered out, returns nil
		{"analytics", []string{"analytics"}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := tokenize(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDetectAuth(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *Config
		expectedVault bool
		expectedAWS   bool
		vaultMethod   string
	}{
		{
			name: "vault token auth",
			cfg: &Config{
				Vault: VaultConfig{
					Address: "http://vault:8200",
					Auth:    VaultAuthConfig{Token: &TokenAuth{Token: "test-token"}},
				},
			},
			expectedVault: true,
			vaultMethod:   "token",
		},
		{
			name: "vault approle auth",
			cfg: &Config{
				Vault: VaultConfig{
					Address: "http://vault:8200",
					Auth:    VaultAuthConfig{AppRole: &AppRoleAuth{RoleID: "role", SecretID: "secret"}},
				},
			},
			expectedVault: true,
			vaultMethod:   "approle",
		},
		{
			name: "vault kubernetes auth",
			cfg: &Config{
				Vault: VaultConfig{
					Address: "http://vault:8200",
					Auth:    VaultAuthConfig{Kubernetes: &KubernetesAuth{Role: "my-role"}},
				},
			},
			expectedVault: true,
			vaultMethod:   "kubernetes",
		},
		{
			name: "aws with region",
			cfg: &Config{
				AWS: AWSConfig{Region: "us-east-1"},
			},
			expectedAWS: true,
		},
		{
			name: "both vault and aws",
			cfg: &Config{
				Vault: VaultConfig{
					Address: "http://vault:8200",
					Auth:    VaultAuthConfig{Token: &TokenAuth{Token: "test"}},
				},
				AWS: AWSConfig{Region: "us-east-1"},
			},
			expectedVault: true,
			expectedAWS:   true,
			vaultMethod:   "token",
		},
		{
			name:          "empty config",
			cfg:           &Config{},
			expectedVault: false,
			expectedAWS:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectAuth(tc.cfg)
			assert.Equal(t, tc.expectedVault, result.VaultAvailable)
			assert.Equal(t, tc.expectedAWS, result.AWSAvailable)
			if tc.vaultMethod != "" {
				assert.Equal(t, tc.vaultMethod, result.VaultMethod)
			}
		})
	}
}

func TestAutoResolveConfig(t *testing.T) {
	ctx := context.Background()

	cfg := &Config{
		Vault: VaultConfig{Address: "http://vault:8200"},
		AWS:   AWSConfig{Region: "us-east-1"},
		Sources: map[string]Source{
			// Empty sources - should be resolved
			"analytics":      {},
			"data-engineers": {},
		},
		Targets: map[string]Target{
			"Staging": {
				AccountID: "111111111111",
				Imports:   []string{"analytics", "data-engineers"},
			},
		},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{Mount: "merged"},
		},
	}

	// Without AWS context, everything should resolve to Vault
	err := AutoResolveConfig(ctx, cfg, nil)
	require.NoError(t, err)

	// Both sources should now be Vault mounts
	assert.NotNil(t, cfg.Sources["analytics"].Vault)
	assert.NotNil(t, cfg.Sources["data-engineers"].Vault)
	assert.Equal(t, "analytics", cfg.Sources["analytics"].Vault.Mount)
	assert.Equal(t, "data-engineers", cfg.Sources["data-engineers"].Vault.Mount)
}

func TestResolvedResource_String(t *testing.T) {
	// Test that resolved resources have meaningful data
	r := ResolvedResource{
		OriginalName:    "analytics-staging",
		ResolvedName:    "analytics-staging",
		Type:            ResourceTypeAWSAccount,
		AccountID:       "111111111111",
		MatchConfidence: MatchExact,
	}

	// Verify all fields are correct
	assert.Equal(t, "analytics-staging", r.OriginalName)
	assert.Equal(t, "analytics-staging", r.ResolvedName)
	assert.Equal(t, "111111111111", r.AccountID)
	assert.Equal(t, ResourceTypeAWSAccount, r.Type)
	assert.Equal(t, MatchExact, r.MatchConfidence)
}

func TestResourceResolver_ResolveAll(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{}
	resolver := NewResourceResolver(ctx, cfg)

	// Without any AWS accounts, everything should resolve to Vault
	names := []string{"source-a", "source-b", "source-c"}
	results := resolver.ResolveAll(names)

	require.Len(t, results, 3)
	for i, r := range results {
		assert.Equal(t, names[i], r.OriginalName)
		assert.Equal(t, ResourceTypeVaultMount, r.Type)
	}
}

// TestJSONKeyNormalizationMatching verifies that names with different
// delimiter styles (underscores, hyphens) can match
func TestJSONKeyNormalizationMatching(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{AWS: AWSConfig{Region: "us-east-1"}}
	resolver := NewResourceResolver(ctx, cfg)

	// AWS account with underscores
	resolver.awsAccounts = []AccountInfo{
		{ID: "111111111111", Name: "Data_Engineers_Sandbox"},
		{ID: "222222222222", Name: "Platform-Services-Prod"},
	}

	// Build indexes
	for _, acct := range resolver.awsAccounts {
		resolver.awsAccountsById[acct.ID] = acct
		normalized := resolver.nameMatcher.NormalizeAccountName(acct.Name)
		resolver.awsAccountsByNormalizedName[normalized] = acct
		resolver.awsAccountsByNormalizedName[acct.Name] = acct
	}

	// Test hyphen input matches underscore account
	t.Run("hyphens match underscores", func(t *testing.T) {
		result := resolver.Resolve("data-engineers-sandbox")
		assert.Equal(t, ResourceTypeAWSAccount, result.Type)
		assert.Equal(t, "111111111111", result.AccountID)
	})

	// Test underscore input matches hyphen account
	t.Run("underscores match hyphens", func(t *testing.T) {
		result := resolver.Resolve("platform_services_prod")
		assert.Equal(t, ResourceTypeAWSAccount, result.Type)
		assert.Equal(t, "222222222222", result.AccountID)
	})
}
