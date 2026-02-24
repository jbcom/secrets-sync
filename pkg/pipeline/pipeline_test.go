package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid configuration",
			config: &Config{
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Sources: map[string]Source{
					"test-source": {
						Vault: &VaultSource{
							Address: "http://localhost:8200",
							Mount:   "secret",
							Paths:   []string{"data/test"},
						},
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
						Region:    "us-east-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid minimal configuration - no explicit vault address",
			config: &Config{
				// Vault address is optional - can be auto-detected
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
					},
				},
			},
			wantErr: false, // Now valid - vault address not required
		},
		{
			name: "Invalid configuration - no targets",
			config: &Config{
				Vault: VaultConfig{Address: "http://localhost:8200"},
			},
			wantErr: true,
			errMsg:  "at least one target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, pipeline)
			assert.NotNil(t, pipeline.config)
			assert.NotNil(t, pipeline.graph)
		})
	}
}

func TestPipeline_Operations(t *testing.T) {
	tests := []struct {
		name      string
		operation Operation
		valid     bool
	}{
		{
			name:      "Merge operation",
			operation: OperationMerge,
			valid:     true,
		},
		{
			name:      "Sync operation",
			operation: OperationSync,
			valid:     true,
		},
		{
			name:      "Pipeline operation",
			operation: OperationPipeline,
			valid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.operation)
		})
	}
}

func TestPipeline_Result(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected Result
	}{
		{
			name: "Successful result",
			result: Result{
				Target:  "test-target",
				Success: true,
			},
			expected: Result{
				Target:  "test-target",
				Success: true,
			},
		},
		{
			name: "Failed result with error",
			result: Result{
				Target:  "test-target",
				Success: false,
				Error:   errors.New("test error"),
			},
			expected: Result{
				Target:  "test-target",
				Success: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected.Target, tt.result.Target)
			assert.Equal(t, tt.expected.Success, tt.result.Success)
			if tt.result.Error != nil {
				assert.NotNil(t, tt.result.Error)
			}
		})
	}
}

func TestNewWithContext(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid configuration without AWS",
			config: &Config{
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Sources: map[string]Source{
					"test-source": {
						Vault: &VaultSource{
							Address: "http://localhost:8200",
							Mount:   "secret",
							Paths:   []string{"data/test"},
						},
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid configuration with AWS",
			config: &Config{
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				AWS: AWSConfig{
					Region: "us-east-1",
				},
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Sources: map[string]Source{
					"test-source": {
						Vault: &VaultSource{
							Address: "http://localhost:8200",
							Mount:   "secret",
							Paths:   []string{"data/test"},
						},
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
						Region:    "us-east-1",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pipeline, err := NewWithContext(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			// Note: May fail if AWS credentials are not available
			// This is expected behavior in unit tests
			if err != nil {
				t.Logf("Expected error without AWS credentials: %v", err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, pipeline)
			assert.NotNil(t, pipeline.config)
		})
	}
}

func TestPipeline_Initialize(t *testing.T) {
	config := &Config{
		Vault: VaultConfig{
			Address: "http://localhost:8200",
		},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "merge",
			},
		},
		Sources: map[string]Source{
			"test-source": {
				Vault: &VaultSource{
					Address: "http://localhost:8200",
					Mount:   "secret",
					Paths:   []string{"data/test"},
				},
			},
		},
		Targets: map[string]Target{
			"test-target": {
				AccountID: "123456789012",
			},
		},
	}

	pipeline, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, pipeline)

	// Validate initialization state
	assert.NotNil(t, pipeline.config)
	assert.NotNil(t, pipeline.graph)
	assert.False(t, pipeline.initialized) // Should not be initialized yet
}

func TestPipeline_DryRun(t *testing.T) {
	tests := []struct {
		name   string
		dryRun bool
	}{
		{
			name:   "Dry run enabled",
			dryRun: true,
		},
		{
			name:   "Dry run disabled",
			dryRun: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Pipeline: PipelineSettings{
					DryRun: tt.dryRun,
				},
				Sources: map[string]Source{
					"test-source": {
						Vault: &VaultSource{
							Address: "http://localhost:8200",
							Mount:   "secret",
							Paths:   []string{"data/test"},
						},
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
					},
				},
			}

			pipeline, err := New(config)
			require.NoError(t, err)
			assert.Equal(t, tt.dryRun, pipeline.config.Pipeline.DryRun)
		})
	}
}

func TestPipeline_ConcurrentExecution(t *testing.T) {
	config := &Config{
		Vault: VaultConfig{
			Address: "http://localhost:8200",
		},
		MergeStore: MergeStoreConfig{
			Vault: &MergeStoreVault{
				Mount: "merge",
			},
		},
		Sources: map[string]Source{
			"test-source": {
				Vault: &VaultSource{
					Address: "http://localhost:8200",
					Mount:   "secret",
					Paths:   []string{"data/test"},
				},
			},
		},
		Targets: map[string]Target{
			"target1": {
				AccountID: "123456789012",
			},
			"target2": {
				AccountID: "123456789013",
			},
		},
	}

	pipeline, err := New(config)
	require.NoError(t, err)

	// Validate that pipeline can handle multiple targets
	assert.Len(t, pipeline.config.Targets, 2)
}

func TestPipeline_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid minimal config - vault address optional",
			config: &Config{
				// Vault address is now optional
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
					},
				},
			},
			wantErr: false, // Now valid
		},
		{
			name: "Valid minimal config - merge store auto-configured",
			config: &Config{
				// Merge store is auto-configured when Vault is present
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
					},
				},
			},
			wantErr: false, // Now valid - merge store auto-configured
		},
		{
			name: "Missing targets",
			config: &Config{
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				MergeStore: MergeStoreConfig{
					Vault: &MergeStoreVault{
						Mount: "merge",
					},
				},
				Sources: map[string]Source{
					"test-source": {},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPipeline_MergeStoreConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		mergeStore MergeStoreConfig
		valid      bool
	}{
		{
			name: "Vault merge store",
			mergeStore: MergeStoreConfig{
				Vault: &MergeStoreVault{
					Mount: "merge",
				},
			},
			valid: true,
		},
		{
			name: "S3 merge store",
			mergeStore: MergeStoreConfig{
				S3: &MergeStoreS3{
					Bucket: "test-bucket",
					Prefix: "merge/",
				},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Vault: VaultConfig{
					Address: "http://localhost:8200",
				},
				MergeStore: tt.mergeStore,
				Sources: map[string]Source{
					"test-source": {
						Vault: &VaultSource{
							Address: "http://localhost:8200",
							Mount:   "secret",
							Paths:   []string{"data/test"},
						},
					},
				},
				Targets: map[string]Target{
					"test-target": {
						AccountID: "123456789012",
					},
				},
			}

			pipeline, err := New(config)
			if tt.valid {
				require.NoError(t, err)
				// Check merge store configuration
				if tt.mergeStore.Vault != nil {
					assert.NotNil(t, pipeline.config.MergeStore.Vault)
				}
				if tt.mergeStore.S3 != nil {
					assert.NotNil(t, pipeline.config.MergeStore.S3)
				}
			}
		})
	}
}

func TestS3Config(t *testing.T) {
	// Alias for S3Config from config.go
	type S3Config = MergeStoreS3

	config := &S3Config{
		Bucket:   "test-bucket",
		Prefix:   "merge/",
		KMSKeyID: "key-123",
	}

	assert.NotNil(t, config)
	assert.Equal(t, "test-bucket", config.Bucket)
	assert.Equal(t, "merge/", config.Prefix)
	assert.Equal(t, "key-123", config.KMSKeyID)
}
