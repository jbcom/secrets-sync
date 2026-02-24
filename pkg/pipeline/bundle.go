package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// BundleID generates a deterministic, reproducible identifier for a merge bundle
// based on the ordered sequence of sources. Same sources in same order = same ID.
func BundleID(sources []string) string {
	// Join sources with a delimiter that won't appear in paths
	joined := strings.Join(sources, "\x00")

	// SHA256 hash for deterministic output
	hash := sha256.Sum256([]byte(joined))

	// Use first 16 bytes (32 hex chars) for reasonable uniqueness without being too long
	return hex.EncodeToString(hash[:16])
}

// BundlePath returns the full path in the merge store for a given bundle.
// Format: {mount}/bundles/{bundle_id}
func BundlePath(mount string, sources []string) string {
	id := BundleID(sources)
	return fmt.Sprintf("%s/bundles/%s", mount, id)
}

// TargetBundlePath returns the merge store path for a specific target.
// This includes the target name for organization but the bundle ID for reproducibility.
// Format: {mount}/targets/{target_name}/{bundle_id}
func TargetBundlePath(mount, targetName string, sources []string) string {
	id := BundleID(sources)
	return fmt.Sprintf("%s/targets/%s/%s", mount, targetName, id)
}

// MergeRequest represents a request to merge N sources into a bundle
type MergeRequest struct {
	// Sources in priority order (later sources override earlier on conflict)
	Sources []string

	// Target name (for organizational purposes)
	Target string

	// DryRun if true, don't actually write
	DryRun bool
}

// SyncRequest represents a request to sync a bundle to target(s)
type SyncRequest struct {
	// BundlePath is the merge store path containing the merged secrets
	BundlePath string

	// Targets to sync to (account IDs or target names)
	Targets []string

	// DryRun if true, don't actually write
	DryRun bool
}

// PipelineRequest is merge + sync as a single operation
type PipelineRequest struct {
	// Sources in priority order for merge
	Sources []string

	// Targets to sync the merged bundle to
	Targets []string

	// DryRun if true, don't actually write
	DryRun bool
}

// GetBundleID returns the deterministic bundle ID for this request
func (r *PipelineRequest) GetBundleID() string {
	return BundleID(r.Sources)
}

// GetMergePath returns the merge store path for this request
func (r *PipelineRequest) GetMergePath(mount string) string {
	return BundlePath(mount, r.Sources)
}
