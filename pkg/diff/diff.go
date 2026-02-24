// Package diff provides change detection and reporting for secrets synchronization.
// It enables dry-run validation, zero-sum differential verification, and
// CI/CD-friendly output formats.
package diff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/utils"
)

// ChangeType represents the type of change detected
type ChangeType string

const (
	ChangeTypeAdded     ChangeType = "added"
	ChangeTypeRemoved   ChangeType = "removed"
	ChangeTypeModified  ChangeType = "modified"
	ChangeTypeUnchanged ChangeType = "unchanged"
)

// SecretChange represents a change to a single secret
type SecretChange struct {
	Path       string     `json:"path"`
	ChangeType ChangeType `json:"change_type"`
	Target     string     `json:"target,omitempty"`

	// Version tracking (v1.2.0 - Requirement 24)
	CurrentVersion int `json:"current_version,omitempty"`
	DesiredVersion int `json:"desired_version,omitempty"`

	// For modified secrets, track key-level changes
	KeysAdded    []string `json:"keys_added,omitempty"`
	KeysRemoved  []string `json:"keys_removed,omitempty"`
	KeysModified []string `json:"keys_modified,omitempty"`

	// Current and desired states (values redacted by default)
	CurrentKeys []string `json:"current_keys,omitempty"`
	DesiredKeys []string `json:"desired_keys,omitempty"`

	// Hash comparison for change detection without exposing values
	CurrentHash string `json:"current_hash,omitempty"`
	DesiredHash string `json:"desired_hash,omitempty"`

	// Enhanced diff output (v1.2.0 - Requirement 25)
	CurrentValues map[string]interface{} `json:"current_values,omitempty"` // For side-by-side comparison
	DesiredValues map[string]interface{} `json:"desired_values,omitempty"` // For side-by-side comparison
	ShowValues    bool                   `json:"show_values,omitempty"`    // Whether to show actual values
}

// TargetDiff represents all changes for a single target
type TargetDiff struct {
	Target  string         `json:"target"`
	Changes []SecretChange `json:"changes"`
	Summary ChangeSummary  `json:"summary"`
}

// ChangeSummary provides statistics about changes
type ChangeSummary struct {
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
	Total     int `json:"total"`
}

// IsZeroSum returns true if there are no changes
func (s ChangeSummary) IsZeroSum() bool {
	return s.Added == 0 && s.Removed == 0 && s.Modified == 0
}

// HasChanges returns true if there are any changes
func (s ChangeSummary) HasChanges() bool {
	return !s.IsZeroSum()
}

// PipelineDiff represents the complete diff for a pipeline run
type PipelineDiff struct {
	Targets    []TargetDiff  `json:"targets"`
	Summary    ChangeSummary `json:"summary"`
	DryRun     bool          `json:"dry_run"`
	ConfigPath string        `json:"config_path,omitempty"`
}

// IsZeroSum returns true if the entire pipeline has no changes
func (p *PipelineDiff) IsZeroSum() bool {
	return p.Summary.IsZeroSum()
}

// ExitCode returns an appropriate exit code for CI/CD:
//   - 0: No changes (zero-sum)
//   - 1: Changes detected
//   - 2: Errors occurred (not handled here)
func (p *PipelineDiff) ExitCode() int {
	if p.IsZeroSum() {
		return 0
	}
	return 1
}

// AddTargetDiff adds a target diff and updates the summary
func (p *PipelineDiff) AddTargetDiff(td TargetDiff) {
	p.Targets = append(p.Targets, td)
	p.Summary.Added += td.Summary.Added
	p.Summary.Removed += td.Summary.Removed
	p.Summary.Modified += td.Summary.Modified
	p.Summary.Unchanged += td.Summary.Unchanged
	p.Summary.Total += td.Summary.Total
}

// DiffSecrets compares two secret maps and returns the changes
func DiffSecrets(current, desired map[string]interface{}) []SecretChange {
	return DiffSecretsWithVersions(current, desired, nil, nil)
}

// DiffSecretsWithVersions compares two secret maps with version information and returns the changes
func DiffSecretsWithVersions(current, desired map[string]interface{}, currentVersions, desiredVersions map[string]int) []SecretChange {
	var changes []SecretChange
	seen := make(map[string]bool)

	// Check desired secrets
	for path, desiredVal := range desired {
		seen[path] = true
		currentVal, exists := current[path]

		// Get version information
		var currentVersion, desiredVersion int
		if currentVersions != nil {
			currentVersion = currentVersions[path]
		}
		if desiredVersions != nil {
			desiredVersion = desiredVersions[path]
		}

		if !exists {
			// New secret
			changes = append(changes, SecretChange{
				Path:           path,
				ChangeType:     ChangeTypeAdded,
				DesiredKeys:    getMapKeys(desiredVal),
				DesiredVersion: desiredVersion,
			})
			continue
		}

		// Compare values
		if utils.DeepEqual(currentVal, desiredVal) {
			changes = append(changes, SecretChange{
				Path:           path,
				ChangeType:     ChangeTypeUnchanged,
				CurrentKeys:    getMapKeys(currentVal),
				DesiredKeys:    getMapKeys(desiredVal),
				CurrentVersion: currentVersion,
				DesiredVersion: desiredVersion,
			})
		} else {
			// Modified - compute key-level diff
			change := SecretChange{
				Path:           path,
				ChangeType:     ChangeTypeModified,
				CurrentKeys:    getMapKeys(currentVal),
				DesiredKeys:    getMapKeys(desiredVal),
				CurrentVersion: currentVersion,
				DesiredVersion: desiredVersion,
			}
			change.KeysAdded, change.KeysRemoved, change.KeysModified = diffMapKeys(currentVal, desiredVal)
			changes = append(changes, change)
		}
	}

	// Check for removed secrets
	for path, currentVal := range current {
		if !seen[path] {
			var currentVersion int
			if currentVersions != nil {
				currentVersion = currentVersions[path]
			}
			changes = append(changes, SecretChange{
				Path:           path,
				ChangeType:     ChangeTypeRemoved,
				CurrentKeys:    getMapKeys(currentVal),
				CurrentVersion: currentVersion,
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})

	return changes
}

// ComputeSummary calculates summary statistics from changes
func ComputeSummary(changes []SecretChange) ChangeSummary {
	var summary ChangeSummary
	for _, c := range changes {
		switch c.ChangeType {
		case ChangeTypeAdded:
			summary.Added++
		case ChangeTypeRemoved:
			summary.Removed++
		case ChangeTypeModified:
			summary.Modified++
		case ChangeTypeUnchanged:
			summary.Unchanged++
		}
		summary.Total++
	}
	return summary
}

// getMapKeys returns the keys of a value if it's a map
func getMapKeys(v interface{}) []string {
	if m, ok := v.(map[string]interface{}); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
	return nil
}

// diffMapKeys computes key-level differences between two maps
func diffMapKeys(current, desired interface{}) (added, removed, modified []string) {
	currentMap, okCurrent := current.(map[string]interface{})
	desiredMap, okDesired := desired.(map[string]interface{})

	if !okCurrent || !okDesired {
		// One or both aren't maps, treat as complete modification
		return nil, nil, []string{"<value>"}
	}

	seen := make(map[string]bool)

	for k, dv := range desiredMap {
		seen[k] = true
		cv, exists := currentMap[k]
		if !exists {
			added = append(added, k)
		} else if !utils.DeepEqual(cv, dv) {
			modified = append(modified, k)
		}
	}

	for k := range currentMap {
		if !seen[k] {
			removed = append(removed, k)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(modified)

	return added, removed, modified
}

// OutputFormat specifies the output format for diff reporting
type OutputFormat string

const (
	OutputFormatHuman      OutputFormat = "human"
	OutputFormatJSON       OutputFormat = "json"
	OutputFormatGitHub     OutputFormat = "github"     // GitHub Actions annotations
	OutputFormatCompact    OutputFormat = "compact"    // One-line summary
	OutputFormatSideBySide OutputFormat = "sidebyside" // Side-by-side comparison (v1.2.0 - Requirement 25)
)

// FormatDiff formats the pipeline diff according to the specified format
func FormatDiff(diff *PipelineDiff, format OutputFormat) string {
	return FormatDiffWithOptions(diff, format, false)
}

// FormatDiffWithOptions formats the pipeline diff with additional options (v1.2.0 - Requirement 25)
func FormatDiffWithOptions(diff *PipelineDiff, format OutputFormat, showValues bool) string {
	switch format {
	case OutputFormatJSON:
		return formatJSON(diff)
	case OutputFormatGitHub:
		return formatGitHub(diff)
	case OutputFormatCompact:
		return formatCompact(diff)
	case OutputFormatSideBySide:
		return formatSideBySide(diff, showValues)
	default:
		return formatHuman(diff)
	}
}

func formatJSON(diff *PipelineDiff) string {
	data, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}

func formatHuman(diff *PipelineDiff) string {
	var sb strings.Builder

	// Header
	if diff.DryRun {
		sb.WriteString("=== DRY RUN - No changes will be applied ===\n\n")
	}

	// Overall summary
	sb.WriteString("Pipeline Diff Summary\n")
	sb.WriteString("=====================\n")
	sb.WriteString(fmt.Sprintf("  Added:     %d\n", diff.Summary.Added))
	sb.WriteString(fmt.Sprintf("  Removed:   %d\n", diff.Summary.Removed))
	sb.WriteString(fmt.Sprintf("  Modified:  %d\n", diff.Summary.Modified))
	sb.WriteString(fmt.Sprintf("  Unchanged: %d\n", diff.Summary.Unchanged))
	sb.WriteString(fmt.Sprintf("  Total:     %d\n", diff.Summary.Total))
	sb.WriteString("\n")

	if diff.IsZeroSum() {
		sb.WriteString("✅ ZERO-SUM: No changes detected\n")
		return sb.String()
	}

	sb.WriteString("⚠️  CHANGES DETECTED\n\n")

	// Per-target details
	for _, td := range diff.Targets {
		if !td.Summary.HasChanges() {
			continue
		}

		sb.WriteString(fmt.Sprintf("Target: %s\n", td.Target))
		sb.WriteString(strings.Repeat("-", 40) + "\n")

		for _, c := range td.Changes {
			if c.ChangeType == ChangeTypeUnchanged {
				continue
			}

			switch c.ChangeType {
			case ChangeTypeAdded:
				versionInfo := ""
				if c.DesiredVersion > 0 {
					versionInfo = fmt.Sprintf(" (v%d)", c.DesiredVersion)
				}
				sb.WriteString(fmt.Sprintf("  + %s (new secret)%s\n", c.Path, versionInfo))
				if len(c.DesiredKeys) > 0 {
					sb.WriteString(fmt.Sprintf("    keys: %v\n", c.DesiredKeys))
				}
			case ChangeTypeRemoved:
				versionInfo := ""
				if c.CurrentVersion > 0 {
					versionInfo = fmt.Sprintf(" (was v%d)", c.CurrentVersion)
				}
				sb.WriteString(fmt.Sprintf("  - %s (removed)%s\n", c.Path, versionInfo))
			case ChangeTypeModified:
				versionInfo := ""
				if c.CurrentVersion > 0 && c.DesiredVersion > 0 {
					versionInfo = fmt.Sprintf(" (v%d → v%d)", c.CurrentVersion, c.DesiredVersion)
				} else if c.CurrentVersion > 0 {
					versionInfo = fmt.Sprintf(" (v%d)", c.CurrentVersion)
				} else if c.DesiredVersion > 0 {
					versionInfo = fmt.Sprintf(" (→ v%d)", c.DesiredVersion)
				}
				sb.WriteString(fmt.Sprintf("  ~ %s (modified)%s\n", c.Path, versionInfo))
				if len(c.KeysAdded) > 0 {
					sb.WriteString(fmt.Sprintf("    + keys: %v\n", c.KeysAdded))
				}
				if len(c.KeysRemoved) > 0 {
					sb.WriteString(fmt.Sprintf("    - keys: %v\n", c.KeysRemoved))
				}
				if len(c.KeysModified) > 0 {
					sb.WriteString(fmt.Sprintf("    ~ keys: %v\n", c.KeysModified))
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatGitHub(diff *PipelineDiff) string {
	var sb strings.Builder

	// Summary as workflow output
	sb.WriteString(fmt.Sprintf("::set-output name=changes::%d\n", diff.Summary.Added+diff.Summary.Removed+diff.Summary.Modified))
	sb.WriteString(fmt.Sprintf("::set-output name=added::%d\n", diff.Summary.Added))
	sb.WriteString(fmt.Sprintf("::set-output name=removed::%d\n", diff.Summary.Removed))
	sb.WriteString(fmt.Sprintf("::set-output name=modified::%d\n", diff.Summary.Modified))
	sb.WriteString(fmt.Sprintf("::set-output name=unchanged::%d\n", diff.Summary.Unchanged))
	sb.WriteString(fmt.Sprintf("::set-output name=zero_sum::%t\n", diff.IsZeroSum()))

	if diff.IsZeroSum() {
		sb.WriteString("::notice::✅ Zero-sum: No changes detected\n")
	} else {
		sb.WriteString(fmt.Sprintf("::warning::⚠️ %d changes detected (%d added, %d removed, %d modified)\n",
			diff.Summary.Added+diff.Summary.Removed+diff.Summary.Modified,
			diff.Summary.Added, diff.Summary.Removed, diff.Summary.Modified))
	}

	// Group annotations by target
	for _, td := range diff.Targets {
		if !td.Summary.HasChanges() {
			continue
		}

		sb.WriteString(fmt.Sprintf("::group::Target: %s (%d changes)\n", td.Target,
			td.Summary.Added+td.Summary.Removed+td.Summary.Modified))

		for _, c := range td.Changes {
			switch c.ChangeType {
			case ChangeTypeAdded:
				sb.WriteString(fmt.Sprintf("::notice::+ %s (new secret)\n", c.Path))
			case ChangeTypeRemoved:
				sb.WriteString(fmt.Sprintf("::warning::- %s (removed)\n", c.Path))
			case ChangeTypeModified:
				sb.WriteString(fmt.Sprintf("::notice::~ %s (modified)\n", c.Path))
			}
		}

		sb.WriteString("::endgroup::\n")
	}

	return sb.String()
}

func formatCompact(diff *PipelineDiff) string {
	if diff.IsZeroSum() {
		return fmt.Sprintf("ZERO-SUM: %d secrets unchanged", diff.Summary.Unchanged)
	}
	return fmt.Sprintf("CHANGES: +%d -%d ~%d =%d (total: %d)",
		diff.Summary.Added, diff.Summary.Removed, diff.Summary.Modified,
		diff.Summary.Unchanged, diff.Summary.Total)
}

// DiffResult wraps PipelineDiff with additional metadata for CLI output
type DiffResult struct {
	Diff     *PipelineDiff `json:"diff"`
	ExitCode int           `json:"exit_code"`
	Message  string        `json:"message"`
}

// NewDiffResult creates a DiffResult from a PipelineDiff
func NewDiffResult(diff *PipelineDiff) *DiffResult {
	result := &DiffResult{
		Diff:     diff,
		ExitCode: diff.ExitCode(),
	}

	if diff.IsZeroSum() {
		result.Message = "No changes detected - pipeline is in sync"
	} else {
		result.Message = fmt.Sprintf("%d changes detected",
			diff.Summary.Added+diff.Summary.Removed+diff.Summary.Modified)
	}

	return result
}

// Enhanced diff formatting functions (v1.2.0 - Requirement 25)

// formatSideBySide formats the diff in side-by-side comparison format
func formatSideBySide(diff *PipelineDiff, showValues bool) string {
	var sb strings.Builder

	// Header
	if diff.DryRun {
		sb.WriteString("=== DRY RUN - No changes will be applied ===\n\n")
	}

	// Overall summary
	sb.WriteString("Pipeline Diff Summary (Side-by-Side)\n")
	sb.WriteString("====================================\n")
	sb.WriteString(fmt.Sprintf("  Added:     %d\n", diff.Summary.Added))
	sb.WriteString(fmt.Sprintf("  Removed:   %d\n", diff.Summary.Removed))
	sb.WriteString(fmt.Sprintf("  Modified:  %d\n", diff.Summary.Modified))
	sb.WriteString(fmt.Sprintf("  Unchanged: %d\n", diff.Summary.Unchanged))
	sb.WriteString(fmt.Sprintf("  Total:     %d\n", diff.Summary.Total))
	sb.WriteString("\n")

	if diff.IsZeroSum() {
		sb.WriteString("✅ ZERO-SUM: No changes detected\n")
		return sb.String()
	}

	sb.WriteString("⚠️  CHANGES DETECTED\n\n")

	// Per-target details with side-by-side comparison
	for _, td := range diff.Targets {
		if !td.Summary.HasChanges() {
			continue
		}

		sb.WriteString(fmt.Sprintf("Target: %s\n", td.Target))
		sb.WriteString(strings.Repeat("=", 80) + "\n")

		for _, c := range td.Changes {
			if c.ChangeType == ChangeTypeUnchanged {
				continue
			}

			sb.WriteString(formatSecretChangeSideBySide(c, showValues))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatSecretChangeSideBySide formats a single secret change in side-by-side format
func formatSecretChangeSideBySide(change SecretChange, showValues bool) string {
	var sb strings.Builder

	// Header with change type and version info
	versionInfo := ""
	if change.CurrentVersion > 0 && change.DesiredVersion > 0 {
		versionInfo = fmt.Sprintf(" (v%d → v%d)", change.CurrentVersion, change.DesiredVersion)
	} else if change.CurrentVersion > 0 {
		versionInfo = fmt.Sprintf(" (v%d)", change.CurrentVersion)
	} else if change.DesiredVersion > 0 {
		versionInfo = fmt.Sprintf(" (→ v%d)", change.DesiredVersion)
	}

	switch change.ChangeType {
	case ChangeTypeAdded:
		sb.WriteString(fmt.Sprintf("+ %s%s\n", change.Path, versionInfo))
		sb.WriteString("  ┌─ NEW SECRET ─────────────────────────────────────────────────────────┐\n")
		if showValues && change.DesiredValues != nil {
			for key, value := range change.DesiredValues {
				maskedValue := maskValue(value, showValues)
				sb.WriteString(fmt.Sprintf("  │ + %-20s: %s\n", key, maskedValue))
			}
		} else if len(change.DesiredKeys) > 0 {
			sb.WriteString(fmt.Sprintf("  │   Keys: %v\n", change.DesiredKeys))
		}
		sb.WriteString("  └──────────────────────────────────────────────────────────────────────┘\n")

	case ChangeTypeRemoved:
		sb.WriteString(fmt.Sprintf("- %s%s\n", change.Path, versionInfo))
		sb.WriteString("  ┌─ REMOVED SECRET ─────────────────────────────────────────────────────┐\n")
		if showValues && change.CurrentValues != nil {
			for key, value := range change.CurrentValues {
				maskedValue := maskValue(value, showValues)
				sb.WriteString(fmt.Sprintf("  │ - %-20s: %s\n", key, maskedValue))
			}
		} else if len(change.CurrentKeys) > 0 {
			sb.WriteString(fmt.Sprintf("  │   Keys: %v\n", change.CurrentKeys))
		}
		sb.WriteString("  └──────────────────────────────────────────────────────────────────────┘\n")

	case ChangeTypeModified:
		sb.WriteString(fmt.Sprintf("~ %s%s\n", change.Path, versionInfo))
		sb.WriteString("  ┌─ CURRENT ─────────────────┬─ DESIRED ─────────────────────────────────┐\n")

		// Show side-by-side comparison
		if showValues && change.CurrentValues != nil && change.DesiredValues != nil {
			allKeys := make(map[string]bool)
			for key := range change.CurrentValues {
				allKeys[key] = true
			}
			for key := range change.DesiredValues {
				allKeys[key] = true
			}

			for key := range allKeys {
				currentVal, currentExists := change.CurrentValues[key]
				desiredVal, desiredExists := change.DesiredValues[key]

				var currentStr, desiredStr string
				if currentExists {
					currentStr = maskValue(currentVal, showValues)
				} else {
					currentStr = "<not set>"
				}
				if desiredExists {
					desiredStr = maskValue(desiredVal, showValues)
				} else {
					desiredStr = "<removed>"
				}

				// Determine change indicator
				indicator := " "
				if !currentExists {
					indicator = "+"
				} else if !desiredExists {
					indicator = "-"
				} else if !utils.DeepEqual(currentVal, desiredVal) {
					indicator = "~"
				}

				sb.WriteString(fmt.Sprintf("  │%s%-10s: %-15s │%s%-10s: %-15s │\n",
					indicator, key, truncateString(currentStr, 15),
					indicator, key, truncateString(desiredStr, 15)))
			}
		} else {
			// Show key-level changes
			if len(change.KeysAdded) > 0 {
				sb.WriteString(fmt.Sprintf("  │ + Added keys: %-12s │                                           │\n",
					strings.Join(change.KeysAdded, ", ")))
			}
			if len(change.KeysRemoved) > 0 {
				sb.WriteString(fmt.Sprintf("  │ - Removed keys: %-10s │                                           │\n",
					strings.Join(change.KeysRemoved, ", ")))
			}
			if len(change.KeysModified) > 0 {
				sb.WriteString(fmt.Sprintf("  │ ~ Modified keys: %-9s │                                           │\n",
					strings.Join(change.KeysModified, ", ")))
			}
		}
		sb.WriteString("  └───────────────────────────┴───────────────────────────────────────────┘\n")
	}

	return sb.String()
}

// maskValue masks sensitive values based on showValues flag and value patterns
func maskValue(value interface{}, showValues bool) string {
	if value == nil {
		return "<nil>"
	}

	strValue := fmt.Sprintf("%v", value)

	if !showValues {
		// Always mask values unless explicitly requested
		return maskSensitiveValue(strValue)
	}

	// Even when showing values, mask obvious sensitive patterns
	if isSensitivePattern(strValue) {
		return maskSensitiveValue(strValue)
	}

	return strValue
}

// isSensitivePattern detects common sensitive value patterns
func isSensitivePattern(value string) bool {
	lowerValue := strings.ToLower(value)

	// Common sensitive patterns
	sensitivePatterns := []string{
		"password", "passwd", "secret", "key", "token", "auth",
		"credential", "private", "api_key", "apikey", "access_key",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerValue, pattern) {
			return true
		}
	}

	// Check for API key patterns (sk-, pk-, etc.)
	if strings.HasPrefix(lowerValue, "sk-") || strings.HasPrefix(lowerValue, "pk-") ||
		strings.HasPrefix(lowerValue, "rk_") || strings.HasPrefix(lowerValue, "xoxb-") {
		return true
	}

	// Check for common formats (base64, hex, etc.)
	if len(value) > 20 && (isBase64Like(value) || isHexLike(value)) {
		return true
	}

	return false
}

// maskSensitiveValue creates a masked representation of a sensitive value
func maskSensitiveValue(value string) string {
	if len(value) == 0 {
		return "<empty>"
	}

	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}

	// Show first 2 and last 2 characters with stars in between
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

// isBase64Like checks if a string looks like base64
func isBase64Like(s string) bool {
	if len(s)%4 != 0 {
		return false
	}

	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') &&
			(c < '0' || c > '9') && c != '+' && c != '/' && c != '=' {
			return false
		}
	}
	return true
}

// isHexLike checks if a string looks like hexadecimal
func isHexLike(s string) bool {
	if len(s) < 8 {
		return false
	}

	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// DiffSecretsWithValues compares secrets and includes values for side-by-side comparison (v1.2.0 - Requirement 25)
func DiffSecretsWithValues(current, desired map[string]interface{}, currentVersions, desiredVersions map[string]int, showValues bool) []SecretChange {
	changes := DiffSecretsWithVersions(current, desired, currentVersions, desiredVersions)

	// Add value information for enhanced diff output
	for i := range changes {
		change := &changes[i]
		change.ShowValues = showValues

		if currentVal, exists := current[change.Path]; exists {
			if m, ok := currentVal.(map[string]interface{}); ok {
				change.CurrentValues = m
			}
		}

		if desiredVal, exists := desired[change.Path]; exists {
			if m, ok := desiredVal.(map[string]interface{}); ok {
				change.DesiredValues = m
			}
		}
	}

	return changes
}
