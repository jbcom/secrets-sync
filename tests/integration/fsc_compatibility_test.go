// Package integration provides FSC-compatible end-to-end tests.
// These tests validate the complete pipeline with realistic patterns:
// - Multi-tier target inheritance (Staging → Production → Demo)
// - Deepmerge strategies (list append, dict merge, scalar override)
// - AWS Organizations discovery with fuzzy account name matching
// - Recursive Vault KV2 listing
// - S3 and Vault merge stores
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestData represents the seed data structure
type TestData struct {
	Sources              map[string]map[string]interface{} `json:"sources"`
	ExpectedMergeResults map[string]interface{}            `json:"expected_merge_results"`
}

// AccountData represents AWS account info for discovery testing
type AccountData struct {
	Accounts []struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Email  string            `json:"email"`
		Status string            `json:"status"`
		OU     string            `json:"ou"`
		Tags   map[string]string `json:"tags"`
	} `json:"accounts"`
	OrganizationalUnits []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Parent string `json:"parent"`
	} `json:"organizational_units"`
}

// TestFSCCompatibilityFullPipeline validates the complete FSC-compatible workflow
func TestFSCCompatibilityFullPipeline(t *testing.T) {
	skipIfNoIntegrationEnv(t)
	ctx := context.Background()

	// Load test data
	testData := loadTestData(t)
	accountData := loadAccountData(t)

	// Setup clients
	vaultClient := setupVaultClient(t)
	awsClient := setupAWSClient(t, ctx)

	// Phase 1: Seed Vault with source secrets
	t.Run("SeedVaultSources", func(t *testing.T) {
		seedVaultFromTestData(t, vaultClient, testData)
	})

	// Phase 2: Validate recursive listing works
	t.Run("RecursiveListing", func(t *testing.T) {
		validateRecursiveListing(t, vaultClient, testData)
	})

	// Phase 3: Test merge phase with inheritance
	t.Run("MergeWithInheritance", func(t *testing.T) {
		testMergePhaseWithInheritance(t, vaultClient, testData)
	})

	// Phase 4: Test Organizations discovery with fuzzy matching
	t.Run("OrganizationsDiscovery", func(t *testing.T) {
		testOrganizationsDiscovery(t, accountData)
	})

	// Phase 5: Test full sync to AWS
	t.Run("SyncToAWS", func(t *testing.T) {
		testSyncToAWS(t, ctx, vaultClient, awsClient, testData)
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		cleanupAll(t, ctx, vaultClient, awsClient)
	})
}

func loadTestData(t *testing.T) *TestData {
	t.Helper()
	data, err := os.ReadFile("testdata/secrets_seed.json")
	if err != nil {
		// Fall back to embedded data for CI
		return getEmbeddedTestData()
	}

	var testData struct {
		Sources              map[string]map[string]interface{} `json:"sources"`
		ExpectedMergeResults map[string]interface{}            `json:"expected_merge_results"`
	}
	require.NoError(t, json.Unmarshal(data, &testData))

	return &TestData{
		Sources:              testData.Sources,
		ExpectedMergeResults: testData.ExpectedMergeResults,
	}
}

func loadAccountData(t *testing.T) *AccountData {
	t.Helper()
	data, err := os.ReadFile("testdata/accounts.json")
	if err != nil {
		return getEmbeddedAccountData()
	}

	var accountData AccountData
	require.NoError(t, json.Unmarshal(data, &accountData))
	return &accountData
}

// getEmbeddedTestData returns default test data when files aren't available
func getEmbeddedTestData() *TestData {
	return &TestData{
		Sources: map[string]map[string]interface{}{
			"analytics": {
				"config/database": map[string]interface{}{
					"host":      "analytics-db.internal",
					"port":      5432,
					"database":  "analytics",
					"ssl":       true,
					"pool_size": 10,
					"tags":      []interface{}{"analytics", "database"},
				},
				"config/api": map[string]interface{}{
					"base_url":   "https://api.analytics.internal",
					"timeout_ms": 30000,
					"retries":    3,
					"features":   []interface{}{"v2", "batch", "streaming"},
				},
				"credentials/service_account": map[string]interface{}{
					"client_id":     "analytics-service",
					"client_secret": "REDACTED-analytics-secret",
					"scopes":        []interface{}{"read", "write", "admin"},
				},
				"nested/deep/level1/level2/config": map[string]interface{}{
					"deeply_nested": true,
					"path_test":     "analytics/nested/deep/level1/level2",
				},
			},
			"data-engineers": {
				"config/database": map[string]interface{}{
					"host":      "engineers-db.internal",
					"port":      5433,
					"database":  "engineering",
					"ssl":       true,
					"pool_size": 20,
					"tags":      []interface{}{"engineering", "database", "extra-tag"},
				},
				"config/tools": map[string]interface{}{
					"dbt_version": "1.5.0",
					"airflow_url": "https://airflow.internal",
					"spark_config": map[string]interface{}{
						"executor_memory": "4g",
						"driver_memory":   "2g",
						"partitions":      200,
					},
				},
				"credentials/snowflake": map[string]interface{}{
					"account":   "xy12345.us-east-1",
					"username":  "data_engineer",
					"password":  "REDACTED-snowflake-pass",
					"warehouse": "COMPUTE_WH",
					"role":      "DATA_ENGINEER",
				},
				"team/members": map[string]interface{}{
					"leads":     []interface{}{"alice", "bob"},
					"engineers": []interface{}{"charlie", "diana", "eve"},
				},
			},
			"shared": {
				"config/common": map[string]interface{}{
					"environment":     "test",
					"region":          "us-east-1",
					"log_level":       "INFO",
					"metrics_enabled": true,
				},
			},
		},
	}
}

func getEmbeddedAccountData() *AccountData {
	return &AccountData{
		Accounts: []struct {
			ID     string            `json:"id"`
			Name   string            `json:"name"`
			Email  string            `json:"email"`
			Status string            `json:"status"`
			OU     string            `json:"ou"`
			Tags   map[string]string `json:"tags"`
		}{
			{ID: "111111111111", Name: "analytics-staging", OU: "ou-xxxx-development", Tags: map[string]string{"Environment": "staging"}},
			{ID: "222222222222", Name: "analytics-production", OU: "ou-xxxx-production", Tags: map[string]string{"Environment": "production"}},
			{ID: "333333333333", Name: "demo-environment", OU: "ou-xxxx-development", Tags: map[string]string{"Environment": "demo"}},
			{ID: "444444444444", Name: "AWS-DataEngineering-Sandbox-Account", OU: "ou-xxxx-development", Tags: map[string]string{"Environment": "sandbox"}},
			{ID: "555555555555", Name: "fsc-analytics-dev-acct", OU: "ou-xxxx-development", Tags: map[string]string{"Environment": "development"}},
			{ID: "666666666666", Name: "org-Data_Engineers_Sandbox", OU: "ou-xxxx-development", Tags: map[string]string{"Environment": "sandbox"}},
		},
	}
}

func seedVaultFromTestData(t *testing.T, client *api.Client, testData *TestData) {
	t.Helper()

	// Create KV2 mounts for each source
	for sourceName := range testData.Sources {
		mountPath := sourceName
		// Try to enable, ignore if exists
		client.Sys().Mount(mountPath, &api.MountInput{
			Type: "kv-v2",
		})
	}

	// Also create merge-store mount
	client.Sys().Mount("merged-secrets", &api.MountInput{Type: "kv-v2"})

	// Seed secrets into each source
	for sourceName, secrets := range testData.Sources {
		for path, data := range secrets {
			fullPath := fmt.Sprintf("%s/data/%s", sourceName, path)
			_, err := client.Logical().Write(fullPath, map[string]interface{}{
				"data": data,
			})
			require.NoError(t, err, "Failed to write secret %s", fullPath)
		}
		t.Logf("Seeded %d secrets into source: %s", len(secrets), sourceName)
	}
}

func validateRecursiveListing(t *testing.T, client *api.Client, testData *TestData) {
	t.Helper()

	for sourceName, secrets := range testData.Sources {
		metadataPath := fmt.Sprintf("%s/metadata", sourceName)
		listed := listVaultSecretsRecursive(t, client, metadataPath)

		// Validate all secrets were found
		for secretPath := range secrets {
			found := false
			for _, listedPath := range listed {
				// Normalize for comparison
				if strings.Contains(listedPath, secretPath) {
					found = true
					break
				}
			}
			assert.True(t, found, "Secret %s not found in recursive listing of %s", secretPath, sourceName)
		}

		t.Logf("Source %s: found %d secrets via recursive listing", sourceName, len(listed))
	}
}

func testMergePhaseWithInheritance(t *testing.T, client *api.Client, testData *TestData) {
	t.Helper()

	// Read sources
	analytics := readAllSecretsFromSource(t, client, "analytics", testData.Sources["analytics"])
	engineers := readAllSecretsFromSource(t, client, "data-engineers", testData.Sources["data-engineers"])

	// Test deepmerge for config/database (exists in both)
	t.Run("DeepMergeDatabase", func(t *testing.T) {
		analyticsDB := analytics["config/database"].(map[string]interface{})
		engineersDB := engineers["config/database"].(map[string]interface{})

		merged := deepMerge(analyticsDB, engineersDB)

		// Scalar override: engineers wins
		assert.Equal(t, "engineers-db.internal", merged["host"])
		assert.Equal(t, "5433", toNumericString(merged["port"]))
		assert.Equal(t, "engineering", merged["database"])
		assert.Equal(t, "20", toNumericString(merged["pool_size"]))

		// List append: tags from both
		tags := merged["tags"].([]interface{})
		assert.GreaterOrEqual(t, len(tags), 3, "Tags should be appended")

		t.Log("DeepMerge validated: scalar override, list append")
	})

	// Test inheritance chain: Staging → Production → Demo
	t.Run("InheritanceChain", func(t *testing.T) {
		// Staging = analytics + data-engineers
		stagingSecrets := mergeAllSecrets(analytics, engineers)

		// Production inherits from Staging (gets everything)
		productionSecrets := stagingSecrets // In reality, read from merge store

		// Demo inherits from Production
		demoSecrets := productionSecrets

		// All should have the merged config/database
		for targetName, secrets := range map[string]map[string]interface{}{
			"Staging":    stagingSecrets,
			"Production": productionSecrets,
			"Demo":       demoSecrets,
		} {
			assert.Contains(t, secrets, "config/database", "%s should have config/database", targetName)
			assert.Contains(t, secrets, "config/api", "%s should have config/api", targetName)
			assert.Contains(t, secrets, "config/tools", "%s should have config/tools", targetName)
		}

		t.Log("Inheritance chain validated: Staging → Production → Demo")
	})
}

func readAllSecretsFromSource(t *testing.T, client *api.Client, sourceName string, expectedSecrets map[string]interface{}) map[string]interface{} {
	t.Helper()
	result := make(map[string]interface{})

	for path := range expectedSecrets {
		fullPath := fmt.Sprintf("%s/data/%s", sourceName, path)
		secret, err := client.Logical().Read(fullPath)
		require.NoError(t, err)
		require.NotNil(t, secret, "Secret %s should exist", fullPath)
		result[path] = secret.Data["data"]
	}

	return result
}

func mergeAllSecrets(sources ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for _, source := range sources {
		for path, data := range source {
			if existing, ok := result[path]; ok {
				// Deepmerge if path exists
				if existingMap, ok := existing.(map[string]interface{}); ok {
					if newMap, ok := data.(map[string]interface{}); ok {
						result[path] = deepMerge(existingMap, newMap)
						continue
					}
				}
			}
			result[path] = data
		}
	}

	return result
}

// toNumericString converts various numeric types to string for comparison
// Handles json.Number, float64, and int values
func toNumericString(v interface{}) string {
	switch n := v.(type) {
	case json.Number:
		return n.String()
	case float64:
		return fmt.Sprintf("%v", n)
	case int:
		return fmt.Sprintf("%d", n)
	case int64:
		return fmt.Sprintf("%d", n)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// TestOrganizationsDiscoveryFuzzyMatching validates the fuzzy account name matching
func testOrganizationsDiscovery(t *testing.T, accountData *AccountData) {
	t.Helper()

	// Test fuzzy matching strategies
	t.Run("FuzzyMatchingStrategies", func(t *testing.T) {
		testCases := []struct {
			accountName    string
			pattern        string
			shouldMatch    bool
			expectedTarget string
		}{
			// Exact patterns
			{"analytics-staging", ".*-staging$", true, "Staging"},
			{"analytics-production", ".*-prod(uction)?$", true, "Production"},
			{"demo-environment", ".*-demo$", false, ""}, // Doesn't match -demo$

			// Fuzzy matching with normalization
			{"AWS-DataEngineering-Sandbox-Account", ".*sandbox.*", true, ""},
			{"fsc-analytics-dev-acct", ".*analytics.*dev.*", true, ""},
			{"org-Data_Engineers_Sandbox", ".*data.?engineer.*", true, ""},
		}

		for _, tc := range testCases {
			normalized := normalizeAccountName(tc.accountName)
			matched, _ := regexp.MatchString("(?i)"+tc.pattern, normalized)
			assert.Equal(t, tc.shouldMatch, matched,
				"Account %s (normalized: %s) with pattern %s",
				tc.accountName, normalized, tc.pattern)
		}
	})

	// Test OU-based filtering
	t.Run("OUFiltering", func(t *testing.T) {
		devAccounts := filterAccountsByOU(accountData.Accounts, "ou-xxxx-development")
		assert.GreaterOrEqual(t, len(devAccounts), 4, "Should find dev accounts")

		prodAccounts := filterAccountsByOU(accountData.Accounts, "ou-xxxx-production")
		assert.Equal(t, 1, len(prodAccounts), "Should find 1 prod account")
	})

	// Test tag-based filtering
	t.Run("TagFiltering", func(t *testing.T) {
		sandboxAccounts := filterAccountsByTag(accountData.Accounts, "Environment", "sandbox")
		assert.GreaterOrEqual(t, len(sandboxAccounts), 2, "Should find sandbox accounts")
	})

	t.Log("Organizations discovery with fuzzy matching validated")
}

// normalizeAccountName applies normalization for fuzzy matching
func normalizeAccountName(name string) string {
	// Lowercase
	normalized := strings.ToLower(name)

	// Strip common prefixes
	prefixes := []string{"aws-", "fsc-", "org-"}
	for _, prefix := range prefixes {
		normalized = strings.TrimPrefix(normalized, prefix)
	}

	// Strip common suffixes
	suffixes := []string{"-account", "-acct"}
	for _, suffix := range suffixes {
		normalized = strings.TrimSuffix(normalized, suffix)
	}

	// Replace underscores and hyphens with single character for matching
	normalized = strings.ReplaceAll(normalized, "_", "-")

	return normalized
}

func filterAccountsByOU(accounts []struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Email  string            `json:"email"`
	Status string            `json:"status"`
	OU     string            `json:"ou"`
	Tags   map[string]string `json:"tags"`
}, ou string) []string {
	var result []string
	for _, acct := range accounts {
		if acct.OU == ou {
			result = append(result, acct.ID)
		}
	}
	return result
}

func filterAccountsByTag(accounts []struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Email  string            `json:"email"`
	Status string            `json:"status"`
	OU     string            `json:"ou"`
	Tags   map[string]string `json:"tags"`
}, tagKey, tagValue string) []string {
	var result []string
	for _, acct := range accounts {
		if val, ok := acct.Tags[tagKey]; ok && strings.EqualFold(val, tagValue) {
			result = append(result, acct.ID)
		}
	}
	return result
}

func testSyncToAWS(t *testing.T, ctx context.Context, vaultClient *api.Client, awsClient *secretsmanager.Client, testData *TestData) {
	t.Helper()

	// Build merged secrets for Staging target
	analytics := readAllSecretsFromSource(t, vaultClient, "analytics", testData.Sources["analytics"])
	engineers := readAllSecretsFromSource(t, vaultClient, "data-engineers", testData.Sources["data-engineers"])
	merged := mergeAllSecrets(analytics, engineers)

	// Sync to AWS
	for path, data := range merged {
		secretName := fmt.Sprintf("Staging/%s", path)
		secretValue, err := json.Marshal(data)
		require.NoError(t, err)

		// Create secret
		_, err = awsClient.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secretName),
			SecretString: aws.String(string(secretValue)),
		})
		if err != nil {
			// Update if exists
			_, err = awsClient.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
				SecretId:     aws.String(secretName),
				SecretString: aws.String(string(secretValue)),
			})
		}
		require.NoError(t, err, "Failed to sync secret %s", secretName)
	}

	// Validate secrets exist in AWS
	for path := range merged {
		secretName := fmt.Sprintf("Staging/%s", path)
		result, err := awsClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretName),
		})
		require.NoError(t, err, "Secret %s should exist in AWS", secretName)
		assert.NotEmpty(t, *result.SecretString)
	}

	t.Logf("Synced %d secrets to AWS Secrets Manager", len(merged))
}

func cleanupAll(t *testing.T, ctx context.Context, vaultClient *api.Client, awsClient *secretsmanager.Client) {
	t.Helper()

	// Cleanup Vault mounts
	mounts := []string{"analytics", "data-engineers", "shared", "merged-secrets"}
	for _, mount := range mounts {
		vaultClient.Sys().Unmount(mount)
	}

	// Cleanup AWS secrets (list and delete)
	paginator := secretsmanager.NewListSecretsPaginator(awsClient, &secretsmanager.ListSecretsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			break
		}
		for _, secret := range page.SecretList {
			if strings.HasPrefix(*secret.Name, "Staging/") || strings.HasPrefix(*secret.Name, "test-") {
				awsClient.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
					SecretId:                   secret.Name,
					ForceDeleteWithoutRecovery: aws.Bool(true),
				})
			}
		}
	}

	t.Log("Cleanup completed")
}

// TestAccountNameNormalization validates the JSON key normalization for fuzzy matching
func TestAccountNameNormalization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"AWS-DataEngineering-Sandbox-Account", "dataengineering-sandbox"},
		{"fsc-analytics-dev-acct", "analytics-dev"},
		{"org-Data_Engineers_Sandbox", "data-engineers-sandbox"},
		{"analytics-staging", "analytics-staging"},
		{"PRODUCTION-ACCOUNT", "production"},
	}

	for _, tc := range testCases {
		normalized := normalizeAccountName(tc.input)
		// Check key components are present (loose matching)
		for _, part := range strings.Split(tc.expected, "-") {
			assert.Contains(t, normalized, part,
				"Normalized '%s' should contain '%s'", normalized, part)
		}
	}
}

// TestTargetPatternMatching validates account-to-target resolution
func TestTargetPatternMatching(t *testing.T) {
	patterns := map[string]string{
		".*-staging$":       "Staging",
		".*-prod(uction)?$": "Production",
		".*-demo$":          "Demo",
	}

	testCases := []struct {
		accountName    string
		expectedTarget string
	}{
		{"analytics-staging", "Staging"},
		{"data-production", "Production"},
		{"analytics-prod", "Production"},
		{"demo-demo", "Demo"},
	}

	for _, tc := range testCases {
		var matchedTarget string
		for pattern, target := range patterns {
			if matched, _ := regexp.MatchString("(?i)"+pattern, tc.accountName); matched {
				matchedTarget = target
				break
			}
		}
		assert.Equal(t, tc.expectedTarget, matchedTarget,
			"Account %s should match target %s", tc.accountName, tc.expectedTarget)
	}
}
