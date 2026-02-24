package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationsDiscovery_MultipleOUsConfig(t *testing.T) {
	t.Run("legacy single OU", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{
			OU: "ou-prod-123",
		}

		// Test that the configuration is properly structured
		assert.Equal(t, "ou-prod-123", cfg.OU)
		assert.Empty(t, cfg.OUs)
	})

	t.Run("multiple OUs", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{
			OUs: []string{"ou-prod-123", "ou-staging-456", "ou-dev-789"},
		}

		assert.Empty(t, cfg.OU)
		assert.Len(t, cfg.OUs, 3)
		assert.Contains(t, cfg.OUs, "ou-prod-123")
		assert.Contains(t, cfg.OUs, "ou-staging-456")
		assert.Contains(t, cfg.OUs, "ou-dev-789")
	})

	t.Run("legacy OU + multiple OUs", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{
			OU:  "ou-prod-123",
			OUs: []string{"ou-staging-456", "ou-dev-789"},
		}

		assert.Equal(t, "ou-prod-123", cfg.OU)
		assert.Len(t, cfg.OUs, 2)
	})

	t.Run("OU caching enabled", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{
			CacheOUStructure: true,
		}

		assert.True(t, cfg.CacheOUStructure)
	})
}

func TestDiscoveryService_CacheInitialization(t *testing.T) {
	discovery := &DiscoveryService{
		ouCache:      make(map[string][]AccountInfo),
		ouChildCache: make(map[string][]string),
	}

	// Test that caches are properly initialized
	assert.NotNil(t, discovery.ouCache)
	assert.NotNil(t, discovery.ouChildCache)
	assert.Len(t, discovery.ouCache, 0)
	assert.Len(t, discovery.ouChildCache, 0)

	// Test cache operations
	testAccounts := []AccountInfo{
		{ID: "111111111111", Name: "Test Account"},
	}

	discovery.ouCache["ou-test-123"] = testAccounts

	cached, exists := discovery.ouCache["ou-test-123"]
	assert.True(t, exists)
	assert.Len(t, cached, 1)
	assert.Equal(t, "111111111111", cached[0].ID)
}
