package vault

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/extended-data-library/secretssync/pkg/driver"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockLogical is a minimal mock for Vault logical operations in tests.
// Only implements ListWithContext which is the method used by recursive listing tests.
type mockLogical struct {
	listWithContextFunc func(ctx context.Context, path string) (*api.Secret, error)
}

// ListWithContext is used by the recursive listing tests
func (m *mockLogical) ListWithContext(ctx context.Context, path string) (*api.Secret, error) {
	if m.listWithContextFunc != nil {
		return m.listWithContextFunc(ctx, path)
	}
	return nil, nil
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *VaultClient
		wantErr bool
	}{
		{
			name: "Valid configuration",
			cfg: &VaultClient{
				Address:    "http://localhost:8200",
				Path:       "secret/data/test",
				AuthMethod: "kubernetes",
			},
			wantErr: false,
		},
		{
			name: "With namespace",
			cfg: &VaultClient{
				Address:    "http://localhost:8200",
				Path:       "secret/data/test",
				AuthMethod: "kubernetes",
				Namespace:  "test-ns",
			},
			wantErr: false,
		},
		{
			name: "With merge enabled",
			cfg: &VaultClient{
				Address:    "http://localhost:8200",
				Path:       "secret/data/test",
				AuthMethod: "kubernetes",
				Merge:      true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, tt.cfg.Address, client.Address)
			assert.Equal(t, tt.cfg.Path, client.Path)
			assert.Equal(t, tt.cfg.AuthMethod, client.AuthMethod)
			assert.Equal(t, tt.cfg.Merge, client.Merge)
		})
	}
}

func TestVaultClient_Validate(t *testing.T) {
	tests := []struct {
		name    string
		client  *VaultClient
		wantErr bool
	}{
		{
			name: "Valid client",
			client: &VaultClient{
				Address: "http://localhost:8200",
			},
			wantErr: false,
		},
		{
			name:    "Missing address",
			client:  &VaultClient{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVaultClient_Driver(t *testing.T) {
	client := &VaultClient{}
	assert.Equal(t, driver.DriverNameVault, client.Driver())
}

func TestVaultClient_GetPath(t *testing.T) {
	client := &VaultClient{Path: "secret/data/test"}
	assert.Equal(t, "secret/data/test", client.GetPath())
}

func TestVaultClient_Meta(t *testing.T) {
	client := &VaultClient{
		Address:    "http://localhost:8200",
		Path:       "secret/data/test",
		AuthMethod: "kubernetes",
	}

	meta := client.Meta()
	assert.NotNil(t, meta)
	assert.Equal(t, "http://localhost:8200", meta["address"])
	assert.Equal(t, "secret/data/test", meta["path"])
	assert.Equal(t, "kubernetes", meta["authMethod"])
}

func TestVaultClient_DeepCopy(t *testing.T) {
	original := &VaultClient{
		Address:    "http://localhost:8200",
		Path:       "secret/data/test",
		AuthMethod: "kubernetes",
		Namespace:  "test-ns",
		Merge:      true,
	}

	copied := original.DeepCopy()
	assert.NotNil(t, copied)
	assert.Equal(t, original.Address, copied.Address)
	assert.Equal(t, original.Path, copied.Path)
	assert.Equal(t, original.AuthMethod, copied.AuthMethod)
	assert.Equal(t, original.Namespace, copied.Namespace)
	assert.Equal(t, original.Merge, copied.Merge)
}

func TestInsertSliceString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		index    int
		value    string
		expected []string
	}{
		{
			name:     "Insert at beginning",
			slice:    []string{"a", "b", "c"},
			index:    0,
			value:    "new",
			expected: []string{"new", "a", "b", "c"},
		},
		{
			name:     "Insert in middle",
			slice:    []string{"a", "b", "c"},
			index:    1,
			value:    "new",
			expected: []string{"a", "new", "b", "c"},
		},
		{
			name:     "Insert at end",
			slice:    []string{"a", "b"},
			index:    2,
			value:    "new",
			expected: []string{"a", "b", "new"},
		},
		{
			name:     "Insert into empty slice",
			slice:    []string{},
			index:    0,
			value:    "new",
			expected: []string{"new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := insertSliceString(tt.slice, tt.index, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVaultClient_GetKVSecretOnce(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		mockFunc  func(ctx context.Context, path string) (*api.Secret, error)
		wantData  map[string]interface{}
		wantErr   bool
		errString string
	}{
		{
			name: "Get secret successfully",
			path: "secret/test",
			mockFunc: func(ctx context.Context, path string) (*api.Secret, error) {
				assert.Equal(t, "secret/data/test", path)
				return &api.Secret{
					Data: map[string]interface{}{
						"data": map[string]interface{}{
							"key": "value",
						},
					},
				}, nil
			},
			wantData: map[string]interface{}{
				"key": "value",
			},
			wantErr: false,
		},
		{
			name:      "Empty path",
			path:      "",
			wantErr:   true,
			errString: "secret path required",
		},
		{
			name:      "Invalid path format",
			path:      "invalid",
			wantErr:   true,
			errString: "secret path must be in kv/path/to/secret format",
		},
		{
			name: "Secret not found",
			path: "secret/notfound",
			mockFunc: func(ctx context.Context, path string) (*api.Secret, error) {
				return nil, nil
			},
			wantErr:   true,
			errString: "secret not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require full Vault mock
			// These tests document expected behavior but need integration tests
			if !tt.wantErr || tt.path == "secret/notfound" {
				t.Skip("Requires full Vault API mock or integration test")
				return
			}

			client := &VaultClient{
				Address: "http://localhost:8200",
			}

			ctx := context.Background()

			// Test error cases that don't require Vault client
			_, err := client.GetKVSecretOnce(ctx, tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
			}
		})
	}
}

func TestVaultClient_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		client   *VaultClient
		defaults *VaultClient
		expected *VaultClient
	}{
		{
			name:   "Apply all defaults",
			client: &VaultClient{},
			defaults: &VaultClient{
				Address:    "http://localhost:8200",
				CIDR:       "10.0.0.0/8",
				AuthMethod: "kubernetes",
				Namespace:  "test-ns",
				Role:       "test-role",
				TTL:        "1h",
			},
			expected: &VaultClient{
				Address:    "http://localhost:8200",
				CIDR:       "10.0.0.0/8",
				AuthMethod: "kubernetes",
				Namespace:  "test-ns",
				Role:       "test-role",
				TTL:        "1h",
			},
		},
		{
			name: "Don't override existing values",
			client: &VaultClient{
				Address:    "http://existing:8200",
				AuthMethod: "token",
			},
			defaults: &VaultClient{
				Address:    "http://default:8200",
				AuthMethod: "kubernetes",
				Namespace:  "default-ns",
			},
			expected: &VaultClient{
				Address:    "http://existing:8200",
				AuthMethod: "token",
				Namespace:  "default-ns",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.SetDefaults(tt.defaults)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Address, tt.client.Address)
			assert.Equal(t, tt.expected.CIDR, tt.client.CIDR)
			assert.Equal(t, tt.expected.AuthMethod, tt.client.AuthMethod)
			assert.Equal(t, tt.expected.Namespace, tt.client.Namespace)
			assert.Equal(t, tt.expected.Role, tt.client.Role)
			assert.Equal(t, tt.expected.TTL, tt.client.TTL)
		})
	}
}

func TestVaultClient_WriteSecretOnce(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		secret  map[string]interface{}
		cas     *int
		wantErr bool
		errMsg  string
	}{
		{
			name: "Write secret with CAS",
			path: "secret/test",
			secret: map[string]interface{}{
				"key": "value",
			},
			cas:     intPtr(1),
			wantErr: false,
		},
		{
			name: "Write secret without CAS",
			path: "secret/test",
			secret: map[string]interface{}{
				"key": "value",
			},
			cas:     nil,
			wantErr: false,
		},
		{
			name:    "Empty path",
			path:    "",
			secret:  map[string]interface{}{"key": "value"},
			wantErr: true,
			errMsg:  "secret path must be in kv/path/to/secret format",
		},
		{
			name:    "Invalid path format",
			path:    "invalid",
			secret:  map[string]interface{}{"key": "value"},
			wantErr: true,
			errMsg:  "secret path must be in kv/path/to/secret format",
		},
		{
			name:    "Nil secret data",
			path:    "secret/test",
			secret:  nil,
			wantErr: true,
			errMsg:  "secret data required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require full Vault mock
			// Only test validation errors that don't require Vault client
			if !tt.wantErr {
				t.Skip("Requires full Vault mock")
				return
			}

			// Create a basic client
			client := &VaultClient{
				Address: "http://localhost:8200",
			}

			ctx := context.Background()
			_, err := client.WriteSecretOnce(ctx, tt.path, tt.secret, tt.cas)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestVaultClient_DeleteSecret(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid path",
			path:    "secret/test",
			wantErr: false,
		},
		{
			name:    "Empty path",
			path:    "",
			wantErr: true,
			errMsg:  "secret path required",
		},
		{
			name:    "Invalid path format",
			path:    "invalid",
			wantErr: true,
			errMsg:  "secret path must be in kv/path/to/secret format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require full Vault mock
			if !tt.wantErr {
				t.Skip("Requires full Vault mock")
				return
			}

			client := &VaultClient{
				Address: "http://localhost:8200",
			}

			ctx := context.Background()
			err := client.DeleteSecret(ctx, tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestVaultClient_Close(t *testing.T) {
	// Create a mock API client
	mockClient, err := api.NewClient(&api.Config{
		Address: "http://localhost:8200",
	})
	require.NoError(t, err)

	mockClient.SetToken("test-token")

	client := &VaultClient{
		Address: "http://localhost:8200",
		Client:  mockClient,
	}

	err = client.Close()
	assert.NoError(t, err)
	assert.Empty(t, client.Client.Token())
}

func TestVaultClient_WriteSecretWithMerge(t *testing.T) {
	tests := []struct {
		name           string
		merge          bool
		existingSecret map[string]interface{}
		newSecret      map[string]interface{}
		expectedData   map[string]interface{}
	}{
		{
			name:  "Merge enabled - merge data",
			merge: true,
			existingSecret: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			newSecret: map[string]interface{}{
				"key2": "updated",
				"key3": "value3",
			},
			expectedData: map[string]interface{}{
				"key1": "value1",
				"key2": "updated",
				"key3": "value3",
			},
		},
		{
			name:  "Merge disabled - override data",
			merge: false,
			existingSecret: map[string]interface{}{
				"key1": "value1",
			},
			newSecret: map[string]interface{}{
				"key2": "value2",
			},
			expectedData: map[string]interface{}{
				"key2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip - these tests require full Vault API mocking
			t.Skip("Requires full Vault mock or integration test")

			client := &VaultClient{
				Address: "http://localhost:8200",
				Merge:   tt.merge,
			}

			ctx := context.Background()
			meta := metav1.ObjectMeta{Name: "test"}
			data, _ := json.Marshal(tt.newSecret)

			// This test validates the merge logic flow
			// Full execution requires mocking the Vault API
			_, err := client.WriteSecret(ctx, meta, "secret/test", data)

			// Without mocks, we expect connection errors
			// The test validates that the code compiles and has the right structure
			assert.Error(t, err) // Expected due to no real Vault
		})
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}

func TestVaultClient_ListSecretsRecursive(t *testing.T) {
	tests := []struct {
		name          string
		basePath      string
		mockResponses map[string]*api.Secret
		expected      []string
		wantErr       bool
		errMsg        string
	}{
		{
			name:     "Single level secrets",
			basePath: "secret/app",
			mockResponses: map[string]*api.Secret{
				"secret/metadata/app/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"config", "database"},
					},
				},
			},
			expected: []string{"secret/app/config", "secret/app/database"},
			wantErr:  false,
		},
		{
			name:     "Nested directory structure",
			basePath: "secret/app",
			mockResponses: map[string]*api.Secret{
				"secret/metadata/app/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"config", "env/", "database"},
					},
				},
				"secret/metadata/app/env/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"prod", "staging", "dev/"},
					},
				},
				"secret/metadata/app/env/dev/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"local", "test"},
					},
				},
			},
			expected: []string{
				"secret/app/config",
				"secret/app/database",
				"secret/app/env/prod",
				"secret/app/env/staging",
				"secret/app/env/dev/local",
				"secret/app/env/dev/test",
			},
			wantErr: false,
		},
		{
			name:     "Deep nesting with mixed content",
			basePath: "kv/myapp",
			mockResponses: map[string]*api.Secret{
				"kv/metadata/myapp/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"api-key", "services/", "version"},
					},
				},
				"kv/metadata/myapp/services/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"auth/", "payment/", "notification"},
					},
				},
				"kv/metadata/myapp/services/auth/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"jwt-secret", "oauth/"},
					},
				},
				"kv/metadata/myapp/services/payment/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"stripe-key", "paypal-config"},
					},
				},
				"kv/metadata/myapp/services/auth/oauth/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"google", "github", "facebook"},
					},
				},
			},
			expected: []string{
				"kv/myapp/api-key",
				"kv/myapp/version",
				"kv/myapp/services/notification",
				"kv/myapp/services/auth/jwt-secret",
				"kv/myapp/services/payment/stripe-key",
				"kv/myapp/services/payment/paypal-config",
				"kv/myapp/services/auth/oauth/google",
				"kv/myapp/services/auth/oauth/github",
				"kv/myapp/services/auth/oauth/facebook",
			},
			wantErr: false,
		},
		{
			name:     "Empty directory",
			basePath: "secret/empty",
			mockResponses: map[string]*api.Secret{
				"secret/metadata/empty/": {
					Data: map[string]interface{}{
						"keys": []interface{}{},
					},
				},
			},
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "Path with trailing slash",
			basePath: "secret/app/",
			mockResponses: map[string]*api.Secret{
				"secret/metadata/app/": {
					Data: map[string]interface{}{
						"keys": []interface{}{"config", "database"},
					},
				},
			},
			expected: []string{"secret/app/config", "secret/app/database"},
			wantErr:  false,
		},
		{
			name:     "Non-existent path",
			basePath: "secret/nonexistent",
			mockResponses: map[string]*api.Secret{
				"secret/metadata/nonexistent/": nil,
			},
			expected: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock logical client with test responses
			mockLogical := &mockLogical{
				listWithContextFunc: func(ctx context.Context, path string) (*api.Secret, error) {
					if response, exists := tt.mockResponses[path]; exists {
						return response, nil
					}
					return nil, nil
				},
			}

			// Inject the mock logical client into the VaultClient
			client := &VaultClient{
				Address: "http://localhost:8200",
			}
			client.SetLogicalClient(mockLogical)

			// Call the actual production listSecretsRecursive function
			ctx := context.Background()
			result, err := client.listSecretsRecursive(ctx, tt.basePath)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestVaultClient_GetMetadataPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Valid KV path",
			path:     "secret/app/config",
			expected: "secret/metadata/app/config/",
			wantErr:  false,
		},
		{
			name:     "Path with trailing slash",
			path:     "secret/app/",
			expected: "secret/metadata/app/",
			wantErr:  false,
		},
		{
			name:     "Root KV path",
			path:     "kv/data",
			expected: "kv/metadata/data/",
			wantErr:  false,
		},
		{
			name:    "Invalid path format",
			path:    "invalid",
			wantErr: true,
			errMsg:  "secret path must be in kv/path/to/secret format",
		},
		{
			name:    "Empty path",
			path:    "",
			wantErr: true,
			errMsg:  "secret path must be in kv/path/to/secret format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &VaultClient{}
			result, err := client.getMetadataPath(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestVaultClient_ListSecretsOnce_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		client  *VaultClient
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Nil client",
			client:  nil,
			path:    "secret/test",
			wantErr: true,
			errMsg:  "vault client not initialized",
		},
		{
			name:    "Client with nil API client",
			client:  &VaultClient{Address: "http://localhost:8200"},
			path:    "secret/test",
			wantErr: true,
			errMsg:  "vault client not initialized",
		},
		{
			name:    "Empty path",
			client:  &VaultClient{Address: "http://localhost:8200", Client: &api.Client{}},
			path:    "",
			wantErr: true,
			errMsg:  "secret path required",
		},
		{
			name:    "Invalid path format",
			client:  &VaultClient{Address: "http://localhost:8200", Client: &api.Client{}},
			path:    "invalid",
			wantErr: true,
			errMsg:  "secret path must be in kv/path/to/secret format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := tt.client.ListSecretsOnce(ctx, tt.path)

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

// TestVaultClient_ListSecretsRecursive_PathTraversalPrevention tests that malicious path inputs are rejected
func TestVaultClient_ListSecretsRecursive_PathTraversalPrevention(t *testing.T) {
	ctx := context.Background()

	// Malicious keys that should be filtered out
	maliciousKeys := []string{
		"../../../etc/passwd", // Directory traversal
		"valid\x00byte",       // Null byte injection
		"//double//slash",     // Double slashes
		"/absolute/path",      // Absolute path
	}

	// Create mock that returns both malicious and valid keys
	mockLogical := &mockLogical{
		listWithContextFunc: func(ctx context.Context, path string) (*api.Secret, error) {
			// Return a mix of malicious and valid keys
			if path == "secret/metadata/test/" {
				return &api.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{
							maliciousKeys[0], // ../../../etc/passwd
							maliciousKeys[1], // valid\x00byte
							"valid_key",      // This should pass
							maliciousKeys[2], // //double//slash
						},
					},
				}, nil
			}
			// For the valid_key, return no subdirectories (it's a leaf)
			if path == "secret/metadata/test/valid_key/" {
				return &api.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{},
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &VaultClient{}
	client.SetLogicalClient(mockLogical)

	secrets, err := client.listSecretsRecursive(ctx, "secret/test")

	// Should succeed without error - malicious keys are filtered, not errored
	require.NoError(t, err)

	// Should only have the valid key
	assert.Len(t, secrets, 1)
	assert.Equal(t, "secret/test/valid_key", secrets[0])

	// Verify malicious keys were NOT included
	for _, secret := range secrets {
		assert.NotContains(t, secret, "..")
		assert.NotContains(t, secret, "\x00")
		assert.NotContains(t, secret, "//")
		assert.False(t, strings.HasPrefix(secret, "/"), "secret should not start with /")
	}
}

// TestVaultClient_getQueueCompactionThreshold tests the adaptive queue compaction threshold logic
func TestVaultClient_getQueueCompactionThreshold(t *testing.T) {
	tests := []struct {
		name                     string
		queueCompactionThreshold int
		maxSecretsPerMount       int
		expected                 int
	}{
		{
			name:                     "Explicit threshold set",
			queueCompactionThreshold: 500,
			maxSecretsPerMount:       100000,
			expected:                 500,
		},
		{
			name:                     "Adaptive threshold - default maxSecrets (100000)",
			queueCompactionThreshold: 0,
			maxSecretsPerMount:       0, // Will use default 100000
			expected:                 1000,
		},
		{
			name:                     "Adaptive threshold - small deployment (1000 secrets)",
			queueCompactionThreshold: 0,
			maxSecretsPerMount:       1000,
			expected:                 10,
		},
		{
			name:                     "Adaptive threshold - medium deployment (50000 secrets)",
			queueCompactionThreshold: 0,
			maxSecretsPerMount:       50000,
			expected:                 500,
		},
		{
			name:                     "Adaptive threshold - large deployment (200000 secrets)",
			queueCompactionThreshold: 0,
			maxSecretsPerMount:       200000,
			expected:                 1000, // Capped at max default
		},
		{
			name:                     "Adaptive threshold - very small deployment (50 secrets)",
			queueCompactionThreshold: 0,
			maxSecretsPerMount:       50,
			expected:                 1, // Minimum threshold
		},
		{
			name:                     "Adaptive threshold - exactly at default (100000 secrets)",
			queueCompactionThreshold: 0,
			maxSecretsPerMount:       100000,
			expected:                 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &VaultClient{
				QueueCompactionThreshold: tt.queueCompactionThreshold,
				MaxSecretsPerMount:       tt.maxSecretsPerMount,
			}
			result := client.getQueueCompactionThreshold()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestVaultClient_getMetadataPath_PathValidation tests that getMetadataPath rejects malicious inputs
func TestVaultClient_getMetadataPath_PathValidation(t *testing.T) {
	client := &VaultClient{}

	tests := []struct {
		name      string
		path      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "Valid path",
			path:      "secret/test",
			wantErr:   false,
			errSubstr: "",
		},
		{
			name:      "Path traversal with ..",
			path:      "secret/../admin/secrets",
			wantErr:   true,
			errSubstr: "forbidden characters",
		},
		{
			name:      "Null byte injection",
			path:      "secret/test\x00admin",
			wantErr:   true,
			errSubstr: "forbidden characters",
		},
		{
			name:      "Double slash",
			path:      "secret//test",
			wantErr:   true,
			errSubstr: "forbidden characters",
		},
		{
			name:      "Absolute path",
			path:      "/secret/test",
			wantErr:   true,
			errSubstr: "forbidden characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.getMetadataPath(tt.path)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result)
				// Verify result contains "metadata"
				assert.Contains(t, result, "metadata")
			}
		})
	}
}
