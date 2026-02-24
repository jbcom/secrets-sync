// Package pipeline provides fuzzy account name matching for dynamic target discovery.
// Supports multiple matching strategies for resolving AWS account names to target configurations.
package pipeline

import (
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// NameMatcher handles fuzzy matching of AWS account names to targets
type NameMatcher struct {
	config *NameMatchingConfig
}

// NewNameMatcher creates a new name matcher with the given config
func NewNameMatcher(cfg *NameMatchingConfig) *NameMatcher {
	if cfg == nil {
		cfg = &NameMatchingConfig{
			Strategy:        "exact",
			CaseInsensitive: true,
		}
	}
	return &NameMatcher{config: cfg}
}

// NormalizeAccountName normalizes an account name for fuzzy matching
// Applies configured normalizations: case folding, prefix/suffix stripping, etc.
func (m *NameMatcher) NormalizeAccountName(name string) string {
	normalized := name

	// Apply case insensitivity
	if m.config.CaseInsensitive {
		normalized = strings.ToLower(normalized)
	}

	// Strip configured prefixes
	for _, prefix := range m.config.StripPrefixes {
		prefixLower := prefix
		if m.config.CaseInsensitive {
			prefixLower = strings.ToLower(prefix)
		}
		if strings.HasPrefix(strings.ToLower(normalized), prefixLower) {
			normalized = normalized[len(prefix):]
		}
	}

	// Strip configured suffixes
	for _, suffix := range m.config.StripSuffixes {
		suffixLower := suffix
		if m.config.CaseInsensitive {
			suffixLower = strings.ToLower(suffix)
		}
		if strings.HasSuffix(strings.ToLower(normalized), suffixLower) {
			normalized = normalized[:len(normalized)-len(suffix)]
		}
	}

	// Apply JSON key normalization if enabled
	if m.config.NormalizeKeys {
		normalized = normalizeForJSONKey(normalized)
	}

	return normalized
}

// normalizeForJSONKey applies JSON key-style normalization:
// - Converts underscores to hyphens for consistency
// - Removes special characters except hyphens
// - Collapses multiple hyphens
func normalizeForJSONKey(name string) string {
	// Replace underscores with hyphens
	normalized := strings.ReplaceAll(name, "_", "-")

	// Keep only alphanumeric and hyphens
	var result strings.Builder
	prevHyphen := false
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
			prevHyphen = false
		} else if r == '-' && !prevHyphen {
			result.WriteRune(r)
			prevHyphen = true
		}
	}

	// Trim leading/trailing hyphens
	return strings.Trim(result.String(), "-")
}

// MatchAccountToTarget finds the best matching target for an account name
// using configured patterns and fuzzy matching strategy
func (m *NameMatcher) MatchAccountToTarget(accountName string, patterns []AccountNamePattern) (string, bool) {
	l := log.WithFields(log.Fields{
		"action":   "MatchAccountToTarget",
		"account":  accountName,
		"strategy": m.config.Strategy,
	})

	normalizedName := m.NormalizeAccountName(accountName)
	l.WithField("normalized", normalizedName).Debug("Normalized account name")

	for _, pattern := range patterns {
		var matched bool
		var err error

		switch m.config.Strategy {
		case "loose", "fuzzy":
			// For fuzzy/loose, normalize the pattern too and do substring match
			matched, err = m.fuzzyMatch(normalizedName, pattern.Pattern)
		default: // "exact"
			matched, err = m.exactMatch(normalizedName, pattern.Pattern)
		}

		if err != nil {
			l.WithError(err).WithField("pattern", pattern.Pattern).Warn("Invalid pattern")
			continue
		}

		if matched {
			l.WithFields(log.Fields{
				"pattern": pattern.Pattern,
				"target":  pattern.Target,
			}).Debug("Account matched pattern")
			return pattern.Target, true
		}
	}

	l.Debug("No pattern matched")
	return "", false
}

// exactMatch performs exact regex matching (case-insensitive if configured)
func (m *NameMatcher) exactMatch(name, pattern string) (bool, error) {
	flags := ""
	if m.config.CaseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(name), nil
}

// fuzzyMatch performs fuzzy matching with normalization and substring matching
func (m *NameMatcher) fuzzyMatch(name, pattern string) (bool, error) {
	flags := ""
	if m.config.CaseInsensitive {
		flags = "(?i)"
	}

	// For fuzzy matching, we wrap the pattern to match substrings
	// unless it already has anchors
	if !strings.HasPrefix(pattern, "^") && !strings.HasPrefix(pattern, ".*") {
		pattern = ".*" + pattern
	}
	if !strings.HasSuffix(pattern, "$") && !strings.HasSuffix(pattern, ".*") {
		pattern = pattern + ".*"
	}

	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(name), nil
}

// FilterAccountsByFuzzyMatch filters accounts using fuzzy name matching
// against a list of target account IDs or names
func FilterAccountsByFuzzyMatch(accounts []AccountInfo, matcher *NameMatcher, patterns []string) []AccountInfo {
	var result []AccountInfo

	for _, acct := range accounts {
		normalized := matcher.NormalizeAccountName(acct.Name)

		for _, pattern := range patterns {
			normalizedPattern := matcher.NormalizeAccountName(pattern)

			// Check if normalized names match (substring for fuzzy)
			if strings.Contains(normalized, normalizedPattern) ||
				strings.Contains(normalizedPattern, normalized) {
				result = append(result, acct)
				break
			}
		}
	}

	return result
}

// ResolveAccountImports resolves which imports an account should inherit
// based on fuzzy matching its name to configured patterns
func (m *NameMatcher) ResolveAccountImports(
	acct AccountInfo,
	patterns []AccountNamePattern,
	defaultImports []string,
	targetConfigs map[string]Target,
) []string {
	l := log.WithFields(log.Fields{
		"action":  "ResolveAccountImports",
		"account": acct.ID,
		"name":    acct.Name,
	})

	// Check if account name matches a target pattern
	if targetName, matched := m.MatchAccountToTarget(acct.Name, patterns); matched {
		// If matched target exists, use its imports
		if target, ok := targetConfigs[targetName]; ok {
			l.WithField("matchedTarget", targetName).Debug("Using imports from matched target")
			return target.Imports
		}
		l.WithField("matchedTarget", targetName).Warn("Matched target not found in config")
	}

	// Fall back to default imports
	return defaultImports
}
