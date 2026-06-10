package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestOrganizationsDiscovery_MultipleOUsConfig(t *testing.T) {
	t.Run("single OU uses OUs list", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{
			OUs: []string{"ou-prod-123"},
		}

		assert.Equal(t, []string{"ou-prod-123"}, cfg.OUs)
	})

	t.Run("multiple OUs", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{
			OUs: []string{"ou-prod-123", "ou-staging-456", "ou-dev-789"},
		}

		assert.Len(t, cfg.OUs, 3)
		assert.Contains(t, cfg.OUs, "ou-prod-123")
		assert.Contains(t, cfg.OUs, "ou-staging-456")
		assert.Contains(t, cfg.OUs, "ou-dev-789")
	})

	t.Run("removed single OU yaml is rejected", func(t *testing.T) {
		cfg := &OrganizationsDiscovery{}

		err := yaml.Unmarshal([]byte("ou: ou-prod-123\n"), cfg)

		assert.ErrorContains(t, err, "organizations.ou has been removed")
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
