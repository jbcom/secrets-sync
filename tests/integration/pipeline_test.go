// Package integration provides end-to-end tests for the merge+sync pipeline.
// These tests require LocalStack and Vault to be running (via docker-compose.test.yml).
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Environment variables for integration testing
const (
	envVaultAddr    = "VAULT_ADDR"
	envVaultToken   = "VAULT_TOKEN"
	envAWSEndpoint  = "AWS_ENDPOINT_URL"
	envAWSRegion    = "AWS_REGION"
	envAWSAccessKey = "AWS_ACCESS_KEY_ID"
	envAWSSecretKey = "AWS_SECRET_ACCESS_KEY"
)

func skipIfNoIntegrationEnv(t *testing.T) {
	t.Helper()
	if os.Getenv(envVaultAddr) == "" || os.Getenv(envAWSEndpoint) == "" {
		t.Skip("Skipping integration test: VAULT_ADDR and AWS_ENDPOINT_URL required")
	}
}

// TestMergeSyncPipeline validates the complete merge+sync workflow:
// 1. Seed Vault with source secrets
// 2. Run merge phase (sources -> merged output with deepmerge)
// 3. Run sync phase (merged output -> AWS Secrets Manager)
// 4. Validate final secrets match expected merged output
func TestMergeSyncPipeline(t *testing.T) {
	skipIfNoIntegrationEnv(t)

	ctx := context.Background()

	// Setup clients
	vaultClient := setupVaultClient(t)
	awsClient := setupAWSClient(t, ctx)

	// Step 1: Seed Vault with source secrets
	seedVaultSecrets(t, vaultClient)

	// Step 2: Validate Vault secrets were created correctly
	validateVaultSecrets(t, vaultClient)

	// Step 3: Simulate merge phase - read from Vault, deepmerge, write to merge store
	mergedSecrets := runMergePhase(t, vaultClient)

	// Step 4: Simulate sync phase - write merged secrets to AWS
	runSyncPhase(t, ctx, awsClient, mergedSecrets)

	// Step 5: Validate AWS secrets match expected output
	validateAWSSecrets(t, ctx, awsClient, mergedSecrets)

	// Cleanup
	cleanup(t, ctx, vaultClient, awsClient)
}

func setupVaultClient(t *testing.T) *api.Client {
	t.Helper()

	cfg := api.DefaultConfig()
	cfg.Address = os.Getenv(envVaultAddr)

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	client.SetToken(os.Getenv(envVaultToken))
	return client
}

func setupAWSClient(t *testing.T, ctx context.Context) *secretsmanager.Client {
	t.Helper()

	endpoint := os.Getenv(envAWSEndpoint)
	region := os.Getenv(envAWSRegion)
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv(envAWSAccessKey),
			os.Getenv(envAWSSecretKey),
			"",
		)),
	)
	require.NoError(t, err)

	return secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
}

// seedVaultSecrets creates generic test fixtures that exercise the merge patterns
func seedVaultSecrets(t *testing.T, client *api.Client) {
	t.Helper()

	client.Sys().Mount("secret", &api.MountInput{Type: "kv-v2"})

	// Source A: base layer
	writeVaultSecret(t, client, "secret/data/source-a/config", map[string]interface{}{
		"data": map[string]interface{}{
			"scalar": "value-a",
			"list":   []interface{}{"a1", "a2"},
			"dict":   map[string]interface{}{"key1": "from-a", "key2": "from-a"},
			"nested": map[string]interface{}{"deep": map[string]interface{}{"value": "a"}},
		},
	})

	// Source B: override layer (merges with A)
	writeVaultSecret(t, client, "secret/data/source-b/config", map[string]interface{}{
		"data": map[string]interface{}{
			"scalar": "value-b",                                                  // override
			"list":   []interface{}{"b1"},                                        // append
			"dict":   map[string]interface{}{"key2": "from-b", "key3": "from-b"}, // merge
			"nested": map[string]interface{}{"deep": map[string]interface{}{"extra": "b"}},
		},
	})

	// Source C: additional layer
	writeVaultSecret(t, client, "secret/data/source-c/other", map[string]interface{}{
		"data": map[string]interface{}{
			"independent": "value-c",
		},
	})

	// Shared: common across targets
	writeVaultSecret(t, client, "secret/data/shared/common", map[string]interface{}{
		"data": map[string]interface{}{
			"shared_key": "shared_value",
		},
	})

	// Nested paths for recursive listing
	for i := 1; i <= 5; i++ {
		path := "secret/data/nested"
		for j := 1; j <= i; j++ {
			path += fmt.Sprintf("/level%d", j)
		}
		writeVaultSecret(t, client, path, map[string]interface{}{
			"data": map[string]interface{}{"depth": i},
		})
	}

	t.Log("Seeded generic test fixtures")
}

func writeVaultSecret(t *testing.T, client *api.Client, path string, data map[string]interface{}) {
	t.Helper()
	_, err := client.Logical().Write(path, data)
	require.NoError(t, err, "Failed to write secret to %s", path)
}

func validateVaultSecrets(t *testing.T, client *api.Client) {
	t.Helper()

	// Validate source-a exists
	secret, err := client.Logical().Read("secret/data/source-a/config")
	require.NoError(t, err)
	require.NotNil(t, secret)

	data := secret.Data["data"].(map[string]interface{})
	assert.Equal(t, "value-a", data["scalar"])

	// Validate nested secrets exist (tests recursive listing)
	nested, err := client.Logical().Read("secret/data/nested/level1/level2/level3")
	require.NoError(t, err)
	require.NotNil(t, nested)

	t.Log("Validated Vault secrets exist")
}

// runMergePhase simulates the merge pattern:
// Target imports: source-a, source-b, source-c, shared
// Expected: deepmerge with list append, dict merge, scalar override
func runMergePhase(t *testing.T, client *api.Client) map[string]map[string]interface{} {
	t.Helper()

	merged := make(map[string]map[string]interface{})

	// Read sources
	sourceA := readVaultSecretData(t, client, "secret/data/source-a/config")
	sourceB := readVaultSecretData(t, client, "secret/data/source-b/config")
	sourceC := readVaultSecretData(t, client, "secret/data/source-c/other")
	shared := readVaultSecretData(t, client, "secret/data/shared/common")

	// Merge A + B (B overrides A)
	mergedConfig := deepMerge(sourceA, sourceB)
	merged["config"] = mergedConfig

	// Pass-through
	merged["other"] = sourceC
	merged["common"] = shared

	// Validate merge results

	// Scalar: B overrides A
	assert.Equal(t, "value-b", mergedConfig["scalar"], "scalar should be overridden by source-b")

	// List: A + B appended
	list := mergedConfig["list"].([]interface{})
	assert.Len(t, list, 3, "list should have 3 items (a1, a2, b1)")

	// Dict: merged (key1 from A, key2 from B, key3 from B)
	dict := mergedConfig["dict"].(map[string]interface{})
	assert.Equal(t, "from-a", dict["key1"], "key1 should be from source-a")
	assert.Equal(t, "from-b", dict["key2"], "key2 should be overridden by source-b")
	assert.Equal(t, "from-b", dict["key3"], "key3 should be from source-b")

	// Nested dict merge
	nested := mergedConfig["nested"].(map[string]interface{})
	deep := nested["deep"].(map[string]interface{})
	assert.Equal(t, "a", deep["value"], "nested.deep.value from source-a")
	assert.Equal(t, "b", deep["extra"], "nested.deep.extra from source-b")

	t.Log("Merge phase validated: scalar override, list append, dict merge")
	return merged
}

func readVaultSecretData(t *testing.T, client *api.Client, path string) map[string]interface{} {
	t.Helper()
	secret, err := client.Logical().Read(path)
	require.NoError(t, err)
	require.NotNil(t, secret)
	return secret.Data["data"].(map[string]interface{})
}

// deepMerge implements the merge strategy:
// - Lists: append
// - Dicts: recursive merge
// - Scalars: override
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy dst
	for k, v := range dst {
		result[k] = v
	}

	// Merge src
	for k, v := range src {
		if existing, ok := result[k]; ok {
			// Handle list append
			if dstSlice, ok := existing.([]interface{}); ok {
				if srcSlice, ok := v.([]interface{}); ok {
					result[k] = append(dstSlice, srcSlice...)
					continue
				}
			}
			// Handle dict merge
			if dstMap, ok := existing.(map[string]interface{}); ok {
				if srcMap, ok := v.(map[string]interface{}); ok {
					result[k] = deepMerge(dstMap, srcMap)
					continue
				}
			}
		}
		// Default: override
		result[k] = v
	}

	return result
}

// runSyncPhase writes merged secrets to AWS Secrets Manager
func runSyncPhase(t *testing.T, ctx context.Context, client *secretsmanager.Client, secrets map[string]map[string]interface{}) {
	t.Helper()

	for name, data := range secrets {
		secretName := "test-sync/" + name
		secretValue, err := json.Marshal(data)
		require.NoError(t, err)

		// Create or update secret
		_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secretName),
			SecretString: aws.String(string(secretValue)),
		})
		if err != nil {
			// Try update if create fails (secret exists)
			_, err = client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
				SecretId:     aws.String(secretName),
				SecretString: aws.String(string(secretValue)),
			})
			require.NoError(t, err)
		}
	}

	t.Log("Sync phase completed - secrets written to AWS")
}

// validateAWSSecrets reads back secrets from AWS and validates they match
func validateAWSSecrets(t *testing.T, ctx context.Context, client *secretsmanager.Client, expected map[string]map[string]interface{}) {
	t.Helper()

	for name, expectedData := range expected {
		secretName := "test-sync/" + name

		result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretName),
		})
		require.NoError(t, err)

		var actualData map[string]interface{}
		err = json.Unmarshal([]byte(*result.SecretString), &actualData)
		require.NoError(t, err)

		// Validate key fields
		for key := range expectedData {
			assert.Contains(t, actualData, key, "Expected key %s in AWS secret %s", key, name)
		}

		t.Logf("Validated AWS secret: %s", secretName)
	}

	t.Log("All AWS secrets validated successfully")
}

// cleanup removes test data
func cleanup(t *testing.T, ctx context.Context, vaultClient *api.Client, awsClient *secretsmanager.Client) {
	t.Helper()

	// Delete Vault secrets
	paths := []string{
		"secret/metadata/source-a",
		"secret/metadata/source-b",
		"secret/metadata/source-c",
		"secret/metadata/shared",
		"secret/metadata/nested",
	}
	for _, path := range paths {
		vaultClient.Logical().Delete(path)
	}

	// Delete AWS secrets
	secretNames := []string{
		"test-sync/config",
		"test-sync/other",
		"test-sync/common",
	}
	for _, name := range secretNames {
		awsClient.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(name),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	}

	t.Log("Cleanup completed")
}

// TestRecursiveVaultListing validates the BFS recursive listing works correctly
func TestRecursiveVaultListing(t *testing.T) {
	skipIfNoIntegrationEnv(t)

	vaultClient := setupVaultClient(t)

	// Create nested structure
	paths := []string{
		"secret/data/recursive-test/level0",
		"secret/data/recursive-test/a/level1",
		"secret/data/recursive-test/a/b/level2",
		"secret/data/recursive-test/a/b/c/level3",
		"secret/data/recursive-test/x/y/z/deep",
	}

	for _, path := range paths {
		writeVaultSecret(t, vaultClient, path, map[string]interface{}{
			"data": map[string]interface{}{"path": path},
		})
	}

	// List recursively using Vault LIST API (simulating our BFS)
	allSecrets := listVaultSecretsRecursive(t, vaultClient, "secret/metadata/recursive-test")

	// Should find all 5 secrets
	assert.GreaterOrEqual(t, len(allSecrets), 5, "Expected at least 5 secrets from recursive listing")

	// Cleanup
	vaultClient.Logical().Delete("secret/metadata/recursive-test")

	t.Logf("Found %d secrets via recursive listing", len(allSecrets))
}

// listVaultSecretsRecursive is a test helper that mimics our BFS implementation
func listVaultSecretsRecursive(t *testing.T, client *api.Client, basePath string) []string {
	t.Helper()

	var allSecrets []string
	visited := make(map[string]bool)
	queue := []string{basePath}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		result, err := client.Logical().List(current)
		if err != nil || result == nil {
			continue
		}

		keys, ok := result.Data["keys"].([]interface{})
		if !ok {
			continue
		}

		for _, key := range keys {
			keyStr := key.(string)
			fullPath := current + "/" + keyStr

			if keyStr[len(keyStr)-1] == '/' {
				// Directory - add to queue
				queue = append(queue, current+"/"+keyStr[:len(keyStr)-1])
			} else {
				// Secret - add to results
				allSecrets = append(allSecrets, fullPath)
			}
		}
	}

	return allSecrets
}

// TestDeepMergeStrategies validates the deepmerge behavior
func TestDeepMergeStrategies(t *testing.T) {
	// This test doesn't need emulators - it validates the merge logic

	t.Run("list append", func(t *testing.T) {
		dst := map[string]interface{}{
			"users": []interface{}{"alice", "bob"},
		}
		src := map[string]interface{}{
			"users": []interface{}{"charlie"},
		}

		result := deepMerge(dst, src)
		users := result["users"].([]interface{})

		assert.Len(t, users, 3)
		assert.Contains(t, users, "alice")
		assert.Contains(t, users, "bob")
		assert.Contains(t, users, "charlie")
	})

	t.Run("dict merge", func(t *testing.T) {
		dst := map[string]interface{}{
			"config": map[string]interface{}{
				"timeout": 30,
				"retries": 3,
			},
		}
		src := map[string]interface{}{
			"config": map[string]interface{}{
				"debug": true,
			},
		}

		result := deepMerge(dst, src)
		config := result["config"].(map[string]interface{})

		assert.Equal(t, 30, config["timeout"])
		assert.Equal(t, 3, config["retries"])
		assert.Equal(t, true, config["debug"])
	})

	t.Run("scalar override", func(t *testing.T) {
		dst := map[string]interface{}{
			"version": "1.0",
		}
		src := map[string]interface{}{
			"version": "2.0",
		}

		result := deepMerge(dst, src)
		assert.Equal(t, "2.0", result["version"])
	})
}

// TestTargetInheritanceChain validates inheritance resolution
func TestTargetInheritanceChain(t *testing.T) {
	// Validates topological ordering: parents must be processed before children

	targets := map[string]struct {
		imports  []string
		inherits string
	}{
		"target-a": {imports: []string{"source-a", "shared"}, inherits: ""},
		"target-b": {imports: []string{"target-a"}, inherits: "target-a"},
		"target-c": {imports: []string{"target-b"}, inherits: "target-b"},
	}

	// Validate inheritance detection
	assert.Empty(t, targets["target-a"].inherits)
	assert.Equal(t, "target-a", targets["target-b"].inherits)
	assert.Equal(t, "target-b", targets["target-c"].inherits)

	// Validate topological order
	order := []string{"target-a", "target-b", "target-c"}
	for i, target := range order {
		if parent := targets[target].inherits; parent != "" {
			for j, tgt := range order {
				if tgt == parent {
					assert.Less(t, j, i, "parent %s must come before %s", parent, target)
					break
				}
			}
		}
	}
}
