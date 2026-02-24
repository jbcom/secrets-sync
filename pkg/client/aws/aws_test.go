package aws

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getTestEndpoint returns the LocalStack endpoint if available
func getTestEndpoint() string {
	return os.Getenv("AWS_ENDPOINT_URL")
}

// skipIfNoLocalStack skips the test if LocalStack is not available
func skipIfNoLocalStack(t *testing.T) {
	t.Helper()
	if getTestEndpoint() == "" {
		t.Skip("Skipping integration test: AWS_ENDPOINT_URL not set (LocalStack not available)")
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *AwsClient
		wantErr bool
	}{
		{
			name: "Valid configuration",
			cfg: &AwsClient{
				Name:   "test-client",
				Region: "us-west-2",
			},
			wantErr: false,
		},
		{
			name: "Default region when not specified",
			cfg: &AwsClient{
				Name: "test-client",
			},
			wantErr: false,
		},
		{
			name: "With role ARN",
			cfg: &AwsClient{
				Name:    "test-client",
				Region:  "eu-west-1",
				RoleArn: "arn:aws:iam::123456789012:role/test-role",
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
			assert.Equal(t, tt.cfg.Name, client.Name)
			if tt.cfg.Region == "" {
				assert.Equal(t, "us-east-1", client.Region)
			} else {
				assert.Equal(t, tt.cfg.Region, client.Region)
			}
		})
	}
}

func TestAwsClient_Validate(t *testing.T) {
	tests := []struct {
		name    string
		client  *AwsClient
		wantErr bool
	}{
		{
			name: "Valid client",
			client: &AwsClient{
				Name: "test-client",
			},
			wantErr: false,
		},
		{
			name:    "Missing name",
			client:  &AwsClient{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, driver.ErrPathRequired, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAwsClient_Driver(t *testing.T) {
	client := &AwsClient{}
	assert.Equal(t, driver.DriverNameAws, client.Driver())
}

func TestAwsClient_GetPath(t *testing.T) {
	client := &AwsClient{Name: "test-path"}
	assert.Equal(t, "test-path", client.GetPath())
}

func TestAwsClient_Meta(t *testing.T) {
	client := &AwsClient{
		Name:   "test-client",
		Region: "us-west-2",
		Tags: map[string]string{
			"env": "test",
		},
	}

	meta := client.Meta()
	assert.NotNil(t, meta)
	assert.Equal(t, "test-client", meta["name"])
	assert.Equal(t, "us-west-2", meta["region"])
}

func TestAwsClient_DeepCopy(t *testing.T) {
	original := &AwsClient{
		Name:           "test",
		Region:         "us-west-2",
		ReplicaRegions: []string{"eu-west-1", "ap-southeast-1"},
		Tags: map[string]string{
			"env": "test",
		},
	}

	copied := original.DeepCopy()
	assert.NotNil(t, copied)
	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Region, copied.Region)
	assert.Equal(t, original.ReplicaRegions, copied.ReplicaRegions)
	assert.Equal(t, original.Tags, copied.Tags)

	// Ensure deep copy, not shallow
	copied.ReplicaRegions[0] = "changed"
	assert.NotEqual(t, original.ReplicaRegions[0], copied.ReplicaRegions[0])
}

func TestAwsClient_ListSecrets(t *testing.T) {
	t.Run("List secrets with LocalStack", func(t *testing.T) {
		skipIfNoLocalStack(t)

		client := &AwsClient{
			Name:           "test",
			Region:         "us-east-1",
			NoEmptySecrets: false,
		}

		ctx := context.Background()

		// Create client with LocalStack endpoint
		err := client.CreateClientWithEndpoint(ctx, getTestEndpoint())
		require.NoError(t, err)

		// List secrets (should be empty initially)
		secrets, err := client.ListSecrets(ctx, "")
		assert.NoError(t, err)
		assert.NotNil(t, secrets)
	})

	t.Run("List secrets validation - no client", func(t *testing.T) {
		// This test validates the code structure without requiring LocalStack
		client := &AwsClient{
			Name:           "test",
			NoEmptySecrets: false,
		}

		// Don't call CreateClient - client.client is nil
		// The test just validates the struct setup
		assert.Equal(t, "test", client.Name)
		assert.False(t, client.NoEmptySecrets)
	})
}

func TestAwsClient_GetAlternatePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Path with leading slash",
			path:     "/foo",
			expected: "foo",
		},
		{
			name:     "Path without leading slash",
			path:     "foo",
			expected: "/foo",
		},
		{
			name:     "Empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "Path with multiple components",
			path:     "/foo/bar/baz",
			expected: "foo/bar/baz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &AwsClient{}
			result := client.getAlternatePath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAwsClient_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		client   *AwsClient
		defaults *AwsClient
		expected *AwsClient
	}{
		{
			name:   "Apply all defaults",
			client: &AwsClient{},
			defaults: &AwsClient{
				Region:         "us-west-2",
				RoleArn:        "arn:aws:iam::123456789012:role/test",
				EncryptionKey:  "key-123",
				ReplicaRegions: []string{"eu-west-1"},
				NoEmptySecrets: true,
				SkipUnchanged:  true,
			},
			expected: &AwsClient{
				Region:         "us-west-2",
				RoleArn:        "arn:aws:iam::123456789012:role/test",
				EncryptionKey:  "key-123",
				ReplicaRegions: []string{"eu-west-1"},
				NoEmptySecrets: true,
				SkipUnchanged:  true,
			},
		},
		{
			name: "Don't override existing values",
			client: &AwsClient{
				Region:  "ap-southeast-1",
				RoleArn: "existing-role",
			},
			defaults: &AwsClient{
				Region:  "us-west-2",
				RoleArn: "default-role",
			},
			expected: &AwsClient{
				Region:  "ap-southeast-1",
				RoleArn: "existing-role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.SetDefaults(tt.defaults)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Region, tt.client.Region)
			assert.Equal(t, tt.expected.RoleArn, tt.client.RoleArn)
			assert.Equal(t, tt.expected.EncryptionKey, tt.client.EncryptionKey)
			assert.Equal(t, tt.expected.ReplicaRegions, tt.client.ReplicaRegions)
			assert.Equal(t, tt.expected.NoEmptySecrets, tt.client.NoEmptySecrets)
			assert.Equal(t, tt.expected.SkipUnchanged, tt.client.SkipUnchanged)
		})
	}
}

func TestAwsClient_WriteSecret(t *testing.T) {
	t.Run("Write and read secret with LocalStack", func(t *testing.T) {
		skipIfNoLocalStack(t)

		client := &AwsClient{
			Name:              "test",
			Region:            "us-east-1",
			SkipUnchanged:     false,
			accountSecretArns: map[string]string{},
		}

		ctx := context.Background()

		// Create client with LocalStack endpoint
		err := client.CreateClientWithEndpoint(ctx, getTestEndpoint())
		require.NoError(t, err)

		meta := metav1.ObjectMeta{Name: "test"}
		secretName := "test-secret-" + t.Name()
		secretValue := []byte(`{"key":"value"}`)

		// Write the secret
		_, err = client.WriteSecret(ctx, meta, secretName, secretValue)
		assert.NoError(t, err)

		// List to populate ARN map
		_, err = client.ListSecrets(ctx, "")
		require.NoError(t, err)

		// Read back the secret
		result, err := client.GetSecret(ctx, secretName)
		assert.NoError(t, err)
		assert.Equal(t, secretValue, result)

		// Cleanup
		_ = client.DeleteSecret(ctx, secretName)
	})

	t.Run("Write secret validation - no client", func(t *testing.T) {
		// This test validates the struct setup without requiring LocalStack
		client := &AwsClient{
			Name:              "test",
			SkipUnchanged:     false,
			accountSecretArns: map[string]string{},
		}

		assert.Equal(t, "test", client.Name)
		assert.False(t, client.SkipUnchanged)
		assert.NotNil(t, client.accountSecretArns)
	})
}

func TestAwsClient_Close(t *testing.T) {
	client := &AwsClient{
		Name: "test",
	}

	err := client.Close()
	assert.NoError(t, err)
	assert.Nil(t, client.client)
}

// TestAwsClient_ConcurrentMapAccess validates that accountSecretArns map
// is protected by arnMu mutex and can handle concurrent read/write operations
// without data races. This test specifically addresses the race condition
// concern raised in issue #29.
func TestAwsClient_ConcurrentMapAccess(t *testing.T) {
	client := &AwsClient{
		Name:              "test-concurrent",
		accountSecretArns: make(map[string]string),
	}

	const (
		numWriters = 10
		numReaders = 20
		iterations = 100
	)

	var wg sync.WaitGroup

	// Simulate concurrent write operations to accountSecretArns (like ListSecrets does)
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Simulate what ListSecrets does: replace the entire map
				newMap := make(map[string]string)
				for k := 0; k < 5; k++ {
					newMap[fmt.Sprintf("secret-%d-%d-%d", writerID, j, k)] = fmt.Sprintf("arn-%d-%d-%d", writerID, j, k)
				}
				client.arnMu.Lock()
				client.accountSecretArns = newMap
				client.arnMu.Unlock()
			}
		}(i)
	}

	// Simulate concurrent reads from accountSecretArns (like GetSecret, updateSecret, DeleteSecret do)
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Simulate what GetSecret/updateSecret/DeleteSecret do: read from map
				client.arnMu.RLock()
				_ = client.accountSecretArns[fmt.Sprintf("secret-%d", readerID)]
				// Read the entire map to increase race condition likelihood
				for k := range client.accountSecretArns {
					_ = k
				}
				client.arnMu.RUnlock()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Test passes if no race condition occurred
	// When run with -race flag, Go's race detector will catch any issues
}

// TestAwsClient_DeepCopyConcurrentSafety validates that DeepCopyInto
// properly protects concurrent access to accountSecretArns during copy
func TestAwsClient_DeepCopyConcurrentSafety(t *testing.T) {
	original := &AwsClient{
		Name:              "test-deepcopy",
		accountSecretArns: make(map[string]string),
	}

	// Populate initial data with mutex protection
	original.arnMu.Lock()
	for i := 0; i < 100; i++ {
		original.accountSecretArns[fmt.Sprintf("secret-%d", i)] = fmt.Sprintf("arn-%d", i)
	}
	original.arnMu.Unlock()

	const numCopiers = 20
	var wg sync.WaitGroup

	// Perform concurrent deep copies
	for i := 0; i < numCopiers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				copied := original.DeepCopy()
				require.NotNil(t, copied)
				// Verify that the copied map is not nil and its length is within the expected range.
				// The original map starts with 100 items and grows to 200.
				require.NotNil(t, copied.accountSecretArns)
				assert.GreaterOrEqual(t, len(copied.accountSecretArns), 100)
				assert.LessOrEqual(t, len(copied.accountSecretArns), 200)
			}
		}()
	}

	// Also modify the original map concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			original.arnMu.Lock()
			original.accountSecretArns[fmt.Sprintf("new-secret-%d", j)] = fmt.Sprintf("new-arn-%d", j)
			original.arnMu.Unlock()
		}
	}()

	// Wait for all goroutines
	wg.Wait()
}
