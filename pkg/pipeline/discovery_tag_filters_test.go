package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterAccountsByTagFilters(t *testing.T) {
	accounts := []AccountInfo{
		{
			ID:   "111111111111",
			Name: "Production Account",
			Tags: map[string]string{
				"Environment": "production",
				"Team":        "platform",
				"CostCenter":  "engineering",
			},
		},
		{
			ID:   "222222222222",
			Name: "Staging Account",
			Tags: map[string]string{
				"Environment": "staging",
				"Team":        "platform",
				"CostCenter":  "engineering",
			},
		},
		{
			ID:   "333333333333",
			Name: "Development Account",
			Tags: map[string]string{
				"Environment": "development",
				"Team":        "backend",
				"CostCenter":  "engineering",
			},
		},
		{
			ID:   "444444444444",
			Name: "Sandbox Account",
			Tags: map[string]string{
				"Environment": "sandbox",
				"Team":        "frontend",
				"CostCenter":  "marketing",
			},
		},
	}

	t.Run("single filter equals", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Environment", Values: []string{"production"}, Operator: "equals"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "AND")
		assert.Len(t, result, 1)
		assert.Equal(t, "111111111111", result[0].ID)
	})

	t.Run("multiple filters AND logic", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Team", Values: []string{"platform"}, Operator: "equals"},
			{Key: "CostCenter", Values: []string{"engineering"}, Operator: "equals"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "AND")
		assert.Len(t, result, 2)
		assert.Equal(t, "111111111111", result[0].ID)
		assert.Equal(t, "222222222222", result[1].ID)
	})

	t.Run("multiple filters OR logic", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Environment", Values: []string{"production"}, Operator: "equals"},
			{Key: "Team", Values: []string{"frontend"}, Operator: "equals"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "OR")
		assert.Len(t, result, 2)
		assert.Equal(t, "111111111111", result[0].ID)
		assert.Equal(t, "444444444444", result[1].ID)
	})

	t.Run("wildcard matching", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Environment", Values: []string{"prod*"}, Operator: "wildcard"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "AND")
		assert.Len(t, result, 1)
		assert.Equal(t, "111111111111", result[0].ID)
	})

	t.Run("contains matching", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Team", Values: []string{"end"}, Operator: "contains"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "AND")
		assert.Len(t, result, 2) // backend and frontend
		assert.Equal(t, "333333333333", result[0].ID)
		assert.Equal(t, "444444444444", result[1].ID)
	})

	t.Run("multiple values in single filter", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Environment", Values: []string{"production", "staging"}, Operator: "equals"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "AND")
		assert.Len(t, result, 2)
		assert.Equal(t, "111111111111", result[0].ID)
		assert.Equal(t, "222222222222", result[1].ID)
	})

	t.Run("no matches", func(t *testing.T) {
		filters := []TagFilter{
			{Key: "Environment", Values: []string{"nonexistent"}, Operator: "equals"},
		}
		result := filterAccountsByTagFilters(accounts, filters, "AND")
		assert.Len(t, result, 0)
	})

	t.Run("empty filters", func(t *testing.T) {
		result := filterAccountsByTagFilters(accounts, []TagFilter{}, "AND")
		assert.Len(t, result, 4) // All accounts returned
	})
}

func TestMatchesWildcard(t *testing.T) {
	tests := []struct {
		text     string
		pattern  string
		expected bool
	}{
		{"production", "prod*", true},
		{"production", "*tion", true},
		{"production", "prod*tion", true},
		{"production", "staging*", false},
		{"production", "p?oduction", true},
		{"production", "p?od*", true},
		{"production", "production", true},
		{"production", "*", true},
		{"", "*", true},
		{"", "", true},
		{"production", "", false},
		{"", "prod", false},
		{"test", "t?st", true},
		{"test", "t??t", true},
		{"test", "t???", true},
		{"test", "t????", false},
	}

	for _, tt := range tests {
		t.Run(tt.text+"_"+tt.pattern, func(t *testing.T) {
			result := matchesWildcard(tt.text, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterAccountsByStatus(t *testing.T) {
	accounts := []AccountInfo{
		{ID: "111111111111", Name: "Active Account", Status: "ACTIVE"},
		{ID: "222222222222", Name: "Suspended Account", Status: "SUSPENDED"},
		{ID: "333333333333", Name: "Closed Account", Status: "CLOSED"},
		{ID: "444444444444", Name: "Another Active", Status: "ACTIVE"},
	}

	t.Run("exclude suspended accounts", func(t *testing.T) {
		result := filterAccountsByStatus(accounts, []string{"SUSPENDED"})
		assert.Len(t, result, 3)
		for _, account := range result {
			assert.NotEqual(t, "SUSPENDED", account.Status)
		}
	})

	t.Run("exclude multiple statuses", func(t *testing.T) {
		result := filterAccountsByStatus(accounts, []string{"SUSPENDED", "CLOSED"})
		assert.Len(t, result, 2)
		for _, account := range result {
			assert.Equal(t, "ACTIVE", account.Status)
		}
	})

	t.Run("no exclusions", func(t *testing.T) {
		result := filterAccountsByStatus(accounts, []string{})
		assert.Len(t, result, 4) // All accounts returned
	})

	t.Run("case insensitive", func(t *testing.T) {
		result := filterAccountsByStatus(accounts, []string{"suspended"})
		assert.Len(t, result, 3)
	})
}

func TestMatchesTagFilter(t *testing.T) {
	account := AccountInfo{
		ID:   "111111111111",
		Name: "Test Account",
		Tags: map[string]string{
			"Environment": "production",
			"Team":        "platform-engineering",
		},
	}

	t.Run("equals operator", func(t *testing.T) {
		filter := TagFilter{Key: "Environment", Values: []string{"production"}, Operator: "equals"}
		assert.True(t, matchesTagFilter(account, filter))

		filter = TagFilter{Key: "Environment", Values: []string{"staging"}, Operator: "equals"}
		assert.False(t, matchesTagFilter(account, filter))
	})

	t.Run("contains operator", func(t *testing.T) {
		filter := TagFilter{Key: "Team", Values: []string{"platform"}, Operator: "contains"}
		assert.True(t, matchesTagFilter(account, filter))

		filter = TagFilter{Key: "Team", Values: []string{"backend"}, Operator: "contains"}
		assert.False(t, matchesTagFilter(account, filter))
	})

	t.Run("wildcard operator", func(t *testing.T) {
		filter := TagFilter{Key: "Team", Values: []string{"platform*"}, Operator: "wildcard"}
		assert.True(t, matchesTagFilter(account, filter))

		filter = TagFilter{Key: "Team", Values: []string{"*engineering"}, Operator: "wildcard"}
		assert.True(t, matchesTagFilter(account, filter))

		filter = TagFilter{Key: "Team", Values: []string{"backend*"}, Operator: "wildcard"}
		assert.False(t, matchesTagFilter(account, filter))
	})

	t.Run("missing tag", func(t *testing.T) {
		filter := TagFilter{Key: "NonExistent", Values: []string{"value"}, Operator: "equals"}
		assert.False(t, matchesTagFilter(account, filter))
	})

	t.Run("account without tags", func(t *testing.T) {
		accountNoTags := AccountInfo{ID: "222222222222", Name: "No Tags"}
		filter := TagFilter{Key: "Environment", Values: []string{"production"}, Operator: "equals"}
		assert.False(t, matchesTagFilter(accountNoTags, filter))
	})
}
