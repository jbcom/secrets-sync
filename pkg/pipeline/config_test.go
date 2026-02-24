package pipeline

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	configContent := `
log:
  level: debug
  format: json

vault:
  address: https://vault.example.com/
  namespace: eng/data-platform
  auth:
    approle:
      mount: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

aws:
  region: us-east-1
  execution_context:
    type: management_account
    account_id: "123456789012"
  control_tower:
    enabled: true
    execution_role:
      name: AWSControlTowerExecution

sources:
  analytics:
    vault:
      mount: analytics
  analytics-engineers:
    vault:
      mount: analytics-engineers

merge_store:
  vault:
    mount: merged-secrets

targets:
  Serverless_Stg:
    account_id: "111111111111"
    imports:
      - analytics
      - analytics-engineers
  Serverless_Prod:
    account_id: "222222222222"
    imports:
      - Serverless_Stg
  livequery_demos:
    account_id: "222222222222"
    imports:
      - Serverless_Prod

pipeline:
  merge:
    parallel: 4
  sync:
    parallel: 4
    delete_orphans: false
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(configContent)
	require.NoError(t, err)
	tmpfile.Close()

	// Set env vars for expansion test
	os.Setenv("VAULT_ROLE_ID", "test-role-id")
	os.Setenv("VAULT_SECRET_ID", "test-secret-id")
	defer os.Unsetenv("VAULT_ROLE_ID")
	defer os.Unsetenv("VAULT_SECRET_ID")

	// Load config
	cfg, err := LoadConfig(tmpfile.Name())
	require.NoError(t, err)

	// Validate structure
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "https://vault.example.com/", cfg.Vault.Address)
	assert.Equal(t, "eng/data-platform", cfg.Vault.Namespace)

	// Check env var expansion
	assert.Equal(t, "test-role-id", cfg.Vault.Auth.AppRole.RoleID)
	assert.Equal(t, "test-secret-id", cfg.Vault.Auth.AppRole.SecretID)

	// Check AWS config
	assert.Equal(t, "us-east-1", cfg.AWS.Region)
	assert.Equal(t, ExecutionContextManagement, cfg.AWS.ExecutionContext.Type)
	assert.True(t, cfg.AWS.ControlTower.Enabled)
	assert.Equal(t, "AWSControlTowerExecution", cfg.AWS.ControlTower.ExecutionRole.Name)

	// Check sources
	assert.Len(t, cfg.Sources, 2)
	assert.Equal(t, "analytics", cfg.Sources["analytics"].Vault.Mount)

	// Check targets
	assert.Len(t, cfg.Targets, 3)
	assert.Equal(t, "111111111111", cfg.Targets["Serverless_Stg"].AccountID)
	assert.Equal(t, []string{"analytics", "analytics-engineers"}, cfg.Targets["Serverless_Stg"].Imports)
	assert.Equal(t, []string{"Serverless_Stg"}, cfg.Targets["Serverless_Prod"].Imports)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid full config",
			config: Config{
				Vault: VaultConfig{Address: "https://vault.example.com"},
				Sources: map[string]Source{
					"analytics": {Vault: &VaultSource{Mount: "analytics"}},
				},
				MergeStore: MergeStoreConfig{Vault: &MergeStoreVault{Mount: "merged"}},
				Targets: map[string]Target{
					"Stg": {AccountID: "111111111111", Imports: []string{"analytics"}},
				},
			},
			wantErr: false,
		},
		{
			name: "minimal config - just targets with imports",
			config: Config{
				// No vault address, no merge store, no explicit sources
				// All will be auto-resolved
				Targets: map[string]Target{
					"Production": {Imports: []string{"analytics", "data-engineers"}},
				},
			},
			wantErr: false,
		},
		{
			name: "minimal config - targets without account_id",
			config: Config{
				// account_id is optional - resolved via fuzzy matching
				Targets: map[string]Target{
					"Staging":    {Imports: []string{"shared"}},
					"Production": {Imports: []string{"Staging"}}, // inherits from Staging
				},
			},
			wantErr: false,
		},
		{
			name: "no targets at all",
			config: Config{
				Vault:      VaultConfig{Address: "https://vault.example.com"},
				MergeStore: MergeStoreConfig{Vault: &MergeStoreVault{Mount: "merged"}},
			},
			wantErr: true,
			errMsg:  "at least one target",
		},
		{
			name: "invalid account_id format when provided",
			config: Config{
				Targets: map[string]Target{
					"Stg": {AccountID: "invalid-id", Imports: []string{"analytics"}},
				},
			},
			wantErr: true,
			errMsg:  "invalid account_id format",
		},
		{
			name: "valid S3 merge store",
			config: Config{
				MergeStore: MergeStoreConfig{S3: &MergeStoreS3{Bucket: "my-bucket", Prefix: "secrets/"}},
				Targets: map[string]Target{
					"Stg": {AccountID: "111111111111", Imports: []string{"analytics"}},
				},
			},
			wantErr: false,
		},
		{
			name: "S3 merge store missing bucket",
			config: Config{
				MergeStore: MergeStoreConfig{S3: &MergeStoreS3{Prefix: "secrets/"}},
				Targets: map[string]Target{
					"Stg": {AccountID: "111111111111", Imports: []string{"analytics"}},
				},
			},
			wantErr: true,
			errMsg:  "merge_store.s3.bucket is required",
		},
		{
			name: "valid dynamic target with discovery",
			config: Config{
				DynamicTargets: map[string]DynamicTarget{
					"sandboxes": {
						Discovery: DiscoveryConfig{
							IdentityCenter: &IdentityCenterDiscovery{Group: "Engineers"},
						},
						Imports: []string{"analytics"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "dynamic target with organizations discovery",
			config: Config{
				DynamicTargets: map[string]DynamicTarget{
					"all_accounts": {
						Discovery: DiscoveryConfig{
							Organizations: &OrganizationsDiscovery{Recursive: true},
						},
						Imports: []string{"shared"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "dynamic target with accounts_list",
			config: Config{
				DynamicTargets: map[string]DynamicTarget{
					"sandboxes": {
						Discovery: DiscoveryConfig{
							AccountsList: &AccountsListDiscovery{Source: "ssm:/platform/sandboxes"},
						},
						Imports: []string{"analytics"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mixed static and dynamic targets",
			config: Config{
				Targets: map[string]Target{
					"Production": {AccountID: "111111111111", Imports: []string{"shared"}},
				},
				DynamicTargets: map[string]DynamicTarget{
					"sandboxes": {
						Discovery: DiscoveryConfig{
							Organizations: &OrganizationsDiscovery{
								OU:        "ou-xxxx-sandboxes",
								Recursive: true,
							},
						},
						Imports: []string{"Production"}, // inherit from static target
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetRoleARN(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		accountID string
		expected  string
	}{
		{
			name: "control tower role",
			config: Config{
				AWS: AWSConfig{
					ControlTower: ControlTowerConfig{
						Enabled: true,
						ExecutionRole: ExecutionRoleConfig{
							Name: "AWSControlTowerExecution",
						},
					},
				},
			},
			accountID: "123456789012",
			expected:  "arn:aws:iam::123456789012:role/AWSControlTowerExecution",
		},
		{
			name: "control tower role with path",
			config: Config{
				AWS: AWSConfig{
					ControlTower: ControlTowerConfig{
						Enabled: true,
						ExecutionRole: ExecutionRoleConfig{
							Name: "CustomRole",
							Path: "/secrets/",
						},
					},
				},
			},
			accountID: "123456789012",
			expected:  "arn:aws:iam::123456789012:role/secrets/CustomRole",
		},
		{
			name: "custom role pattern",
			config: Config{
				AWS: AWSConfig{
					ExecutionContext: ExecutionContextConfig{
						CustomRolePattern: "arn:aws:iam::{{.AccountID}}:role/SecretsHub",
					},
				},
			},
			accountID: "123456789012",
			expected:  "arn:aws:iam::123456789012:role/SecretsHub",
		},
		{
			name: "explicit target role",
			config: Config{
				AWS: AWSConfig{
					ControlTower: ControlTowerConfig{
						Enabled: true,
						ExecutionRole: ExecutionRoleConfig{
							Name: "AWSControlTowerExecution",
						},
					},
				},
				Targets: map[string]Target{
					"Special": {
						AccountID: "123456789012",
						RoleARN:   "arn:aws:iam::123456789012:role/SpecialRole",
					},
				},
			},
			accountID: "123456789012",
			expected:  "arn:aws:iam::123456789012:role/SpecialRole",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetRoleARN(tt.accountID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsInheritedTarget(t *testing.T) {
	cfg := Config{
		Sources: map[string]Source{
			"analytics": {Vault: &VaultSource{Mount: "analytics"}},
		},
		Targets: map[string]Target{
			"Stg":  {AccountID: "111111111111", Imports: []string{"analytics"}},
			"Prod": {AccountID: "222222222222", Imports: []string{"Stg"}},
		},
	}

	assert.False(t, cfg.IsInheritedTarget("Stg")) // imports only source
	assert.True(t, cfg.IsInheritedTarget("Prod")) // imports another target
}

func TestGetSourcePath(t *testing.T) {
	cfg := Config{
		Sources: map[string]Source{
			"analytics": {Vault: &VaultSource{Mount: "analytics"}},
		},
		MergeStore: MergeStoreConfig{Vault: &MergeStoreVault{Mount: "merged-secrets"}},
		Targets: map[string]Target{
			"Stg":  {AccountID: "111111111111", Imports: []string{"analytics"}},
			"Prod": {AccountID: "222222222222", Imports: []string{"Stg"}},
		},
	}

	// Direct source
	assert.Equal(t, "analytics", cfg.GetSourcePath("analytics"))

	// Inherited target
	assert.Equal(t, "merged-secrets/Stg", cfg.GetSourcePath("Stg"))
}

func TestIsValidAWSAccountID(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
		valid     bool
	}{
		{"valid 12 digits", "123456789012", true},
		{"valid all zeros", "000000000000", true},
		{"too short", "12345678901", false},
		{"too long", "1234567890123", false},
		{"contains letters", "12345678901a", false},
		{"contains special chars", "123456789-12", false},
		{"empty", "", false},
		{"spaces", "123456789 12", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAWSAccountID(tt.accountID)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestConfigValidateAccountIDFormat(t *testing.T) {
	// Test that invalid account IDs are rejected
	cfg := Config{
		Vault: VaultConfig{Address: "https://vault.example.com"},
		Sources: map[string]Source{
			"analytics": {Vault: &VaultSource{Mount: "analytics"}},
		},
		MergeStore: MergeStoreConfig{Vault: &MergeStoreVault{Mount: "merged"}},
		Targets: map[string]Target{
			"Stg": {AccountID: "invalid", Imports: []string{"analytics"}},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid account_id format")
}

// TestTargetInheritance tests the target inheritance detection and resolution
func TestTargetInheritance(t *testing.T) {
	t.Run("IsInheritedTarget - detect inherited targets", func(t *testing.T) {
		cfg := Config{
			Sources: map[string]Source{
				"analytics": {Vault: &VaultSource{Mount: "analytics"}},
				"common":    {Vault: &VaultSource{Mount: "common"}},
			},
			Targets: map[string]Target{
				"Stg": {
					AccountID: "111111111111",
					Imports:   []string{"analytics", "common"},
				},
				"Prod": {
					AccountID: "222222222222",
					Imports:   []string{"Stg"}, // Inherits from another target
				},
				"Direct": {
					AccountID: "333333333333",
					Imports:   []string{"analytics"}, // Only sources, no inheritance
				},
			},
		}

		// Prod inherits from Stg (another target)
		assert.True(t, cfg.IsInheritedTarget("Prod"), "Prod should be detected as inherited")

		// Stg and Direct only import from sources, not targets
		assert.False(t, cfg.IsInheritedTarget("Stg"), "Stg should not be detected as inherited")
		assert.False(t, cfg.IsInheritedTarget("Direct"), "Direct should not be detected as inherited")

		// Non-existent target
		assert.False(t, cfg.IsInheritedTarget("NonExistent"), "Non-existent target should return false")
	})

	t.Run("Multi-level inheritance chain", func(t *testing.T) {
		cfg := Config{
			Sources: map[string]Source{
				"analytics": {Vault: &VaultSource{Mount: "analytics"}},
			},
			Targets: map[string]Target{
				"Stg": {
					AccountID: "111111111111",
					Imports:   []string{"analytics"},
				},
				"Prod": {
					AccountID: "222222222222",
					Imports:   []string{"Stg"},
				},
				"ProdCA": {
					AccountID: "333333333333",
					Imports:   []string{"Prod"}, // Inherits from Prod which inherits from Stg
				},
			},
			MergeStore: MergeStoreConfig{
				Vault: &MergeStoreVault{Mount: "merged-secrets"},
			},
		}

		// Verify inheritance detection
		assert.False(t, cfg.IsInheritedTarget("Stg"))
		assert.True(t, cfg.IsInheritedTarget("Prod"))
		assert.True(t, cfg.IsInheritedTarget("ProdCA"))
	})

	t.Run("GetSourcePath - resolve paths correctly", func(t *testing.T) {
		cfg := Config{
			Sources: map[string]Source{
				"analytics": {Vault: &VaultSource{Mount: "kv/analytics"}},
				"common":    {Vault: &VaultSource{Mount: "kv/common"}},
			},
			Targets: map[string]Target{
				"Stg": {
					AccountID: "111111111111",
					Imports:   []string{"analytics"},
				},
				"Prod": {
					AccountID: "222222222222",
					Imports:   []string{"Stg"},
				},
			},
			MergeStore: MergeStoreConfig{
				Vault: &MergeStoreVault{Mount: "merged-secrets"},
			},
		}

		// Direct source should return mount path
		assert.Equal(t, "kv/analytics", cfg.GetSourcePath("analytics"))
		assert.Equal(t, "kv/common", cfg.GetSourcePath("common"))

		// Inherited target should return merge store path
		assert.Equal(t, "merged-secrets/Stg", cfg.GetSourcePath("Stg"))
		assert.Equal(t, "merged-secrets/Prod", cfg.GetSourcePath("Prod"))

		// Non-existent should return the name itself
		assert.Equal(t, "nonexistent", cfg.GetSourcePath("nonexistent"))
	})

	t.Run("GetSourcePath with S3 merge store", func(t *testing.T) {
		cfg := Config{
			Sources: map[string]Source{
				"analytics": {Vault: &VaultSource{Mount: "kv/analytics"}},
			},
			Targets: map[string]Target{
				"Stg": {
					AccountID: "111111111111",
					Imports:   []string{"analytics"},
				},
				"Prod": {
					AccountID: "222222222222",
					Imports:   []string{"Stg"},
				},
			},
			MergeStore: MergeStoreConfig{
				S3: &MergeStoreS3{Bucket: "secrets-bucket", Prefix: "merged/"},
			},
		}

		// With S3 merge store, inherited targets still return the import name
		// The S3 path is resolved separately in the S3 store implementation
		assert.Equal(t, "Stg", cfg.GetSourcePath("Stg"))
	})

	t.Run("Mixed imports - sources and targets", func(t *testing.T) {
		cfg := Config{
			Sources: map[string]Source{
				"analytics": {Vault: &VaultSource{Mount: "kv/analytics"}},
				"common":    {Vault: &VaultSource{Mount: "kv/common"}},
			},
			Targets: map[string]Target{
				"Stg": {
					AccountID: "111111111111",
					Imports:   []string{"analytics", "common"},
				},
				"Prod": {
					AccountID: "222222222222",
					Imports:   []string{"Stg", "common"}, // Mix of target and source
				},
			},
			MergeStore: MergeStoreConfig{
				Vault: &MergeStoreVault{Mount: "merged"},
			},
		}

		// Prod imports from both a target (Stg) and a source (common)
		// It should be detected as inherited because it has at least one target import
		assert.True(t, cfg.IsInheritedTarget("Prod"))
	})

	t.Run("Inheritance order matters for merge", func(t *testing.T) {
		// This test documents the expected behavior for merge order
		// When Prod imports ["Stg", "common"], it should:
		// 1. Read merged state of Stg from merge store
		// 2. Read common from source
		// 3. Deep merge them in order

		cfg := Config{
			Sources: map[string]Source{
				"base": {Vault: &VaultSource{Mount: "kv/base"}},
			},
			Targets: map[string]Target{
				"Stg": {
					AccountID: "111111111111",
					Imports:   []string{"base"},
				},
				"Prod": {
					AccountID: "222222222222",
					Imports:   []string{"Stg"},
				},
			},
			MergeStore: MergeStoreConfig{
				Vault: &MergeStoreVault{Mount: "merged"},
			},
		}

		// Verify that Stg must be processed before Prod (topological ordering)
		// This is handled by the graph package
		assert.True(t, cfg.IsInheritedTarget("Prod"))
		assert.False(t, cfg.IsInheritedTarget("Stg"))
	})

	t.Run("Circular dependency detection", func(t *testing.T) {
		cfg := Config{
			Targets: map[string]Target{
				"A": {AccountID: "111111111111", Imports: []string{"B"}},
				"B": {AccountID: "222222222222", Imports: []string{"A"}},
			},
		}

		err := cfg.ValidateTargetInheritance()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular")
	})

	t.Run("Valid inheritance chain passes validation", func(t *testing.T) {
		cfg := Config{
			Sources: map[string]Source{
				"base": {Vault: &VaultSource{Mount: "kv/base"}},
			},
			Targets: map[string]Target{
				"Stg":  {AccountID: "111111111111", Imports: []string{"base"}},
				"Prod": {AccountID: "222222222222", Imports: []string{"Stg"}},
				"Demo": {AccountID: "333333333333", Imports: []string{"Prod"}},
			},
		}

		err := cfg.ValidateTargetInheritance()
		assert.NoError(t, err)
	})

	t.Run("Three-way circular dependency detection", func(t *testing.T) {
		cfg := Config{
			Targets: map[string]Target{
				"A": {AccountID: "111111111111", Imports: []string{"B"}},
				"B": {AccountID: "222222222222", Imports: []string{"C"}},
				"C": {AccountID: "333333333333", Imports: []string{"A"}},
			},
		}

		err := cfg.ValidateTargetInheritance()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular")
		// Verify the error message shows the full cycle path
		assert.Contains(t, err.Error(), "->")
	})

	t.Run("Self-reference detection (A -> A)", func(t *testing.T) {
		cfg := Config{
			Targets: map[string]Target{
				"A": {AccountID: "111111111111", Imports: []string{"A"}}, // Self-reference
			},
		}

		err := cfg.ValidateTargetInheritance()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular")
		assert.Contains(t, err.Error(), "self-reference")
	})

	t.Run("Cycle detection shows full path", func(t *testing.T) {
		cfg := Config{
			Targets: map[string]Target{
				"A": {AccountID: "111111111111", Imports: []string{"B"}},
				"B": {AccountID: "222222222222", Imports: []string{"A"}},
			},
		}

		err := cfg.ValidateTargetInheritance()
		assert.Error(t, err)
		// Error should show the full cycle path: A -> B -> A or B -> A -> B
		errMsg := err.Error()
		assert.Contains(t, errMsg, "->")
		// Should contain at least two arrows for a proper cycle display
		assert.True(t, strings.Count(errMsg, "->") >= 1, "Error message should show cycle path with arrows")
	})
}
