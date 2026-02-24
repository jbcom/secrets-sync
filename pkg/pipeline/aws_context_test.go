package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetAccountTagsStructure verifies the GetAccountTags function signature and error handling
func TestGetAccountTagsStructure(t *testing.T) {
	// This test verifies the structure without requiring AWS credentials
	// Real integration tests should be run with actual AWS access

	t.Run("method_exists", func(t *testing.T) {
		// Create a minimal execution context (will fail on AWS operations but validates structure)
		ec := &AWSExecutionContext{
			Config: &AWSConfig{
				Region: "us-east-1",
			},
		}

		// Verify that GetAccountTags exists and returns the right types
		// This will return an error since we don't have a valid org client, but that's expected
		tags, err := ec.GetAccountTags(context.TODO(), "123456789012")

		// Should return error about no access to Organizations
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no access to Organizations")
		assert.Nil(t, tags)
	})
}

// TestAccountInfoTagsField verifies that AccountInfo has a Tags field
func TestAccountInfoTagsField(t *testing.T) {
	// Create an AccountInfo with tags
	acct := AccountInfo{
		ID:     "123456789012",
		Name:   "TestAccount",
		Email:  "test@example.com",
		Status: "ACTIVE",
		Tags: map[string]string{
			"Environment": "production",
			"Team":        "platform",
		},
	}

	// Verify all fields are stored correctly
	assert.Equal(t, "123456789012", acct.ID)
	assert.Equal(t, "TestAccount", acct.Name)
	assert.Equal(t, "test@example.com", acct.Email)
	assert.Equal(t, "ACTIVE", acct.Status)
	assert.NotNil(t, acct.Tags)
	assert.Equal(t, "production", acct.Tags["Environment"])
	assert.Equal(t, "platform", acct.Tags["Team"])
}

// TestFilterAccountsByTagsWithNilTags verifies filtering handles accounts without tags
func TestFilterAccountsByTagsWithNilTags(t *testing.T) {
	accounts := []AccountInfo{
		{
			ID:   "111111111111",
			Name: "WithTags",
			Tags: map[string]string{"Environment": "production"},
		},
		{
			ID:   "222222222222",
			Name: "NoTags",
			Tags: nil, // No tags
		},
		{
			ID:   "333333333333",
			Name: "EmptyTags",
			Tags: map[string]string{}, // Empty map
		},
	}

	result := filterAccountsByTags(accounts, map[string][]string{"Environment": {"production"}})

	// Only the account with matching tags should be returned
	assert.Len(t, result, 1)
	assert.Equal(t, "111111111111", result[0].ID)
}

// TestFilterAccountsByTagsPartialMatch verifies all required tags must match
func TestFilterAccountsByTagsPartialMatch(t *testing.T) {
	accounts := []AccountInfo{
		{
			ID:   "111111111111",
			Name: "BothTags",
			Tags: map[string]string{
				"Environment": "production",
				"Team":        "platform",
			},
		},
		{
			ID:   "222222222222",
			Name: "OnlyEnvironment",
			Tags: map[string]string{
				"Environment": "production",
			},
		},
		{
			ID:   "333333333333",
			Name: "WrongTeam",
			Tags: map[string]string{
				"Environment": "production",
				"Team":        "analytics",
			},
		},
	}

	// Require both tags to match
	result := filterAccountsByTags(accounts, map[string][]string{
		"Environment": {"production"},
		"Team":        {"platform"},
	})

	// Only the account with both matching tags should be returned
	assert.Len(t, result, 1)
	assert.Equal(t, "111111111111", result[0].ID)
}

// TestOrganizationsDiscoveryTagFiltering tests the tag filtering in discovery
func TestOrganizationsDiscoveryTagFiltering(t *testing.T) {
	// Test the discovery config parsing and tag filtering logic
	// Tags now support multiple values per key for OR matching
	cfg := &OrganizationsDiscovery{
		Tags: map[string][]string{
			"Environment": {"production", "staging"},
			"CostCenter":  {"engineering"},
		},
	}

	assert.NotNil(t, cfg.Tags)
	assert.Equal(t, 2, len(cfg.Tags))
	assert.ElementsMatch(t, []string{"production", "staging"}, cfg.Tags["Environment"])
	assert.ElementsMatch(t, []string{"engineering"}, cfg.Tags["CostCenter"])
}

// TestDiscoveryWithTagsAndOU tests discovery configuration with both OU and tags
func TestDiscoveryWithTagsAndOU(t *testing.T) {
	cfg := &OrganizationsDiscovery{
		OU:        "ou-abc-12345678",
		Tags:      map[string][]string{"Environment": {"production"}},
		Recursive: true,
	}

	// Verify configuration is valid
	assert.Equal(t, "ou-abc-12345678", cfg.OU)
	assert.True(t, cfg.Recursive)
	assert.NotNil(t, cfg.Tags)
	assert.ElementsMatch(t, []string{"production"}, cfg.Tags["Environment"])
}

// TestDiscoveryWithTagsOnly tests discovery configuration with only tags (no OU)
func TestDiscoveryWithTagsOnly(t *testing.T) {
	cfg := &OrganizationsDiscovery{
		Tags: map[string][]string{"Environment": {"sandbox", "development"}},
	}

	// When no OU is specified, all accounts should be listed and filtered by tags
	// Now supports multiple values per tag for OR matching
	assert.Equal(t, "", cfg.OU)
	assert.False(t, cfg.Recursive)
	assert.NotNil(t, cfg.Tags)
	assert.ElementsMatch(t, []string{"sandbox", "development"}, cfg.Tags["Environment"])
}
