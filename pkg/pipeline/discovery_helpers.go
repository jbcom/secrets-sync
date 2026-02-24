package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// discoverFromAccountsList discovers accounts from an external source (e.g., SSM Parameter Store)
func (d *DiscoveryService) discoverFromAccountsList(cfg *AccountsListDiscovery) ([]AccountInfo, error) {
	l := log.WithFields(log.Fields{
		"action": "discoverFromAccountsList",
		"source": cfg.Source,
	})
	l.Debug("Discovering accounts from external list")

	// Parse the source - currently supports SSM Parameter Store
	if strings.HasPrefix(cfg.Source, "ssm:") {
		paramName := strings.TrimPrefix(cfg.Source, "ssm:")
		return d.getAccountsFromSSM(paramName)
	}

	return nil, fmt.Errorf("unsupported accounts list source: %s (supported: ssm:)", cfg.Source)
}

// getAccountsFromSSM retrieves account IDs from an SSM Parameter Store parameter.
// The parameter value can be:
//   - A comma-separated list of account IDs: "111111111111,222222222222,333333333333"
//   - A JSON array: ["111111111111","222222222222","333333333333"]
//   - A JSON array of objects: [{"id": "111111111111", "name": "Account1"}, ...]
func (d *DiscoveryService) getAccountsFromSSM(paramName string) ([]AccountInfo, error) {
	l := log.WithFields(log.Fields{
		"action": "getAccountsFromSSM",
		"param":  paramName,
	})
	l.Debug("Fetching accounts from SSM Parameter Store")

	// Get parameter value
	value, err := d.awsCtx.GetSSMParameter(d.ctx, paramName)
	if err != nil {
		return nil, err
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("SSM parameter %s is empty", paramName)
	}

	var accounts []AccountInfo

	// Try to parse as JSON array first
	if strings.HasPrefix(value, "[") {
		// Try as array of objects with id/name fields
		var objArray []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(value), &objArray); err == nil && len(objArray) > 0 && objArray[0].ID != "" {
			for _, obj := range objArray {
				accounts = append(accounts, AccountInfo{
					ID:   obj.ID,
					Name: obj.Name,
				})
			}
			l.WithField("count", len(accounts)).Debug("Parsed SSM parameter as JSON object array")
			return accounts, nil
		}

		// Try as simple string array
		var strArray []string
		if err := json.Unmarshal([]byte(value), &strArray); err == nil {
			for _, id := range strArray {
				id = strings.TrimSpace(id)
				if id != "" {
					accounts = append(accounts, AccountInfo{ID: id})
				}
			}
			l.WithField("count", len(accounts)).Debug("Parsed SSM parameter as JSON string array")
			return accounts, nil
		}
	}

	// Fall back to comma-separated list
	parts := strings.Split(value, ",")
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id != "" {
			accounts = append(accounts, AccountInfo{ID: id})
		}
	}

	l.WithField("count", len(accounts)).Debug("Parsed SSM parameter as comma-separated list")
	return accounts, nil
}

// Helper functions

func isExcluded(accountID string, excludeList []string) bool {
	for _, excluded := range excludeList {
		if excluded == accountID {
			return true
		}
	}
	return false
}

func sanitizeTargetName(name string) string {
	// Replace spaces and special characters with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	// Remove any characters that aren't alphanumeric or underscore
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func deduplicateAccounts(accounts []AccountInfo) []AccountInfo {
	seen := make(map[string]bool)
	var result []AccountInfo
	for _, a := range accounts {
		if !seen[a.ID] {
			seen[a.ID] = true
			result = append(result, a)
		}
	}
	return result
}

// filterAccountsByTags filters accounts that match ALL required tag conditions.
// Tags support multiple values per key - an account matches if it has ANY of the values.
// Example: Tags{"Environment": ["staging", "sandbox"]} matches accounts with
// Environment=staging OR Environment=sandbox
func filterAccountsByTags(accounts []AccountInfo, requiredTags map[string][]string) []AccountInfo {
	var result []AccountInfo
	for _, a := range accounts {
		if a.Tags == nil {
			continue
		}
		matches := true
		for tagKey, allowedValues := range requiredTags {
			accountTagValue, hasTag := a.Tags[tagKey]
			if !hasTag {
				matches = false
				break
			}
			// Check if account's tag value is in the allowed values list
			valueMatches := false
			for _, allowed := range allowedValues {
				if strings.EqualFold(accountTagValue, allowed) {
					valueMatches = true
					break
				}
			}
			if !valueMatches {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, a)
		}
	}
	return result
}

// filterAccountsByTagFilters filters accounts using enhanced tag filtering with wildcards and AND/OR logic
func filterAccountsByTagFilters(accounts []AccountInfo, tagFilters []TagFilter, combination string) []AccountInfo {
	if len(tagFilters) == 0 {
		return accounts
	}

	var result []AccountInfo
	for _, account := range accounts {
		if matchesTagFilters(account, tagFilters, combination) {
			result = append(result, account)
		}
	}
	return result
}

// matchesTagFilters checks if an account matches the tag filter conditions
func matchesTagFilters(account AccountInfo, tagFilters []TagFilter, combination string) bool {
	if len(tagFilters) == 0 {
		return true
	}

	// Default to AND logic if not specified
	useAndLogic := combination != "OR"

	for _, filter := range tagFilters {
		matches := matchesTagFilter(account, filter)

		if useAndLogic {
			// AND logic: all filters must match
			if !matches {
				return false
			}
		} else {
			// OR logic: any filter can match
			if matches {
				return true
			}
		}
	}

	// For AND logic, we reach here only if all matched
	// For OR logic, we reach here only if none matched
	return useAndLogic
}

// matchesTagFilter checks if an account matches a single tag filter
func matchesTagFilter(account AccountInfo, filter TagFilter) bool {
	if account.Tags == nil {
		return false
	}

	accountValue, hasTag := account.Tags[filter.Key]
	if !hasTag {
		return false
	}

	// Check if account value matches any of the filter values
	for _, filterValue := range filter.Values {
		if matchesTagValue(accountValue, filterValue, filter.Operator) {
			return true
		}
	}

	return false
}

// matchesTagValue checks if an account tag value matches a filter value using the specified operator
func matchesTagValue(accountValue, filterValue, operator string) bool {
	switch operator {
	case "contains":
		return strings.Contains(strings.ToLower(accountValue), strings.ToLower(filterValue))
	case "wildcard":
		return matchesWildcard(accountValue, filterValue)
	default: // "equals" or empty
		return strings.EqualFold(accountValue, filterValue)
	}
}

// matchesWildcard performs wildcard matching (* and ? patterns)
func matchesWildcard(text, pattern string) bool {
	// Convert to lowercase for case-insensitive matching
	text = strings.ToLower(text)
	pattern = strings.ToLower(pattern)

	// Simple wildcard implementation
	// * matches any sequence of characters
	// ? matches any single character
	return wildcardMatch(text, pattern)
}

// wildcardMatch implements wildcard pattern matching
func wildcardMatch(text, pattern string) bool {
	// Handle empty pattern
	if pattern == "" {
		return text == ""
	}

	// Handle * at the beginning
	if pattern[0] == '*' {
		if len(pattern) == 1 {
			return true // * matches everything
		}
		// Try matching the rest of the pattern at each position
		for i := 0; i <= len(text); i++ {
			if wildcardMatch(text[i:], pattern[1:]) {
				return true
			}
		}
		return false
	}

	// Handle empty text
	if text == "" {
		return pattern == ""
	}

	// Handle ? or exact character match
	if pattern[0] == '?' || pattern[0] == text[0] {
		return wildcardMatch(text[1:], pattern[1:])
	}

	return false
}

// filterAccountsByStatus filters out accounts with excluded statuses
func filterAccountsByStatus(accounts []AccountInfo, excludeStatuses []string) []AccountInfo {
	if len(excludeStatuses) == 0 {
		return accounts
	}

	var result []AccountInfo
	for _, account := range accounts {
		excluded := false
		for _, status := range excludeStatuses {
			if strings.EqualFold(account.Status, status) {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, account)
		}
	}
	return result
}
