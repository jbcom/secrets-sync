// Package secretssync provides Python bindings for the secrets-sync pipeline.
//
// This package exposes the core secrets synchronization functionality
// for use from Python via gopy-generated bindings.
package secretssync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/extended-data-library/secretssync/pkg/diff"
	"github.com/extended-data-library/secretssync/pkg/pipeline"
)

// Version of the Python bindings
const Version = "0.1.0"

// Operation constants for Python clients
const (
	OperationPipeline = "pipeline"
	OperationMerge    = "merge"
	OperationSync     = "sync"
)

// OutputFormat constants for Python clients
const (
	OutputFormatHuman      = "human"
	OutputFormatJSON       = "json"
	OutputFormatGitHub     = "github"
	OutputFormatCompact    = "compact"
	OutputFormatSideBySide = "side-by-side"
)

// PipelineConfig represents the pipeline configuration in a Python-friendly format
type PipelineConfig struct {
	Path string // Path to YAML configuration file
}

// SyncOptions configures pipeline execution
type SyncOptions struct {
	DryRun          bool   // If true, don't make actual changes
	Operation       string // "merge", "sync", or "pipeline" (use Operation* constants)
	Targets         string // Comma-separated list of targets (empty for all)
	ContinueOnError bool   // Continue on errors
	Parallelism     int    // Number of parallel operations
	ComputeDiff     bool   // Compute and return diff
	OutputFormat    string // "human", "json", "github", "compact", "side-by-side" (use OutputFormat* constants)
	ShowValues      bool   // If true, show unmasked secret values in diff output
}

// SyncResult represents the outcome of a sync operation
type SyncResult struct {
	Success          bool   // Overall success status
	TargetCount      int    // Number of targets processed
	SecretsProcessed int    // Total secrets processed
	SecretsAdded     int    // Secrets added
	SecretsModified  int    // Secrets modified
	SecretsRemoved   int    // Secrets removed
	SecretsUnchanged int    // Secrets unchanged
	DurationMs       int64  // Duration in milliseconds
	ErrorMessage     string // Error message if failed
	ResultsJSON      string // Full results as JSON
	DiffOutput       string // Diff output if computed
}

// DefaultSyncOptions returns sensible default options
func DefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		DryRun:          false,
		Operation:       OperationPipeline,
		Targets:         "",
		ContinueOnError: false,
		Parallelism:     4,
		ComputeDiff:     false,
		OutputFormat:    OutputFormatHuman,
		ShowValues:      false,
	}
}

// NewPipelineConfig creates a new pipeline configuration from a file path
func NewPipelineConfig(path string) *PipelineConfig {
	return &PipelineConfig{Path: path}
}

// loadConfig loads a configuration file and returns the config or an error message
func loadConfig(configPath string) (*pipeline.Config, string) {
	cfg, err := pipeline.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Sprintf("Failed to load config: %v", err)
	}
	return cfg, ""
}

// loadAndValidateConfig loads and validates a configuration file
func loadAndValidateConfig(configPath string) (*pipeline.Config, string) {
	cfg, errMsg := loadConfig(configPath)
	if errMsg != "" {
		return nil, errMsg
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Sprintf("Invalid configuration: %v", err)
	}
	return cfg, ""
}

// ValidateConfig validates a pipeline configuration file
func ValidateConfig(configPath string) (bool, string) {
	_, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		return false, errMsg
	}
	return true, "Configuration is valid"
}

// RunPipeline executes the secrets synchronization pipeline
func RunPipeline(configPath string, opts *SyncOptions) *SyncResult {
	result := &SyncResult{}
	startTime := time.Now()

	// Handle nil options by using defaults
	if opts == nil {
		opts = DefaultSyncOptions()
	}

	ctx := context.Background()

	// Load configuration
	cfg, errMsg := loadConfig(configPath)
	if errMsg != "" {
		result.ErrorMessage = errMsg
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	// Create pipeline
	p, err := pipeline.NewWithContext(ctx, cfg)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create pipeline: %v", err)
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	// Parse options
	pipelineOpts := pipeline.DefaultOptions()
	pipelineOpts.DryRun = opts.DryRun
	pipelineOpts.ContinueOnError = opts.ContinueOnError
	pipelineOpts.ComputeDiff = opts.ComputeDiff

	if opts.Parallelism > 0 {
		pipelineOpts.Parallelism = opts.Parallelism
	}

	switch opts.Operation {
	case OperationMerge:
		pipelineOpts.Operation = pipeline.OperationMerge
	case OperationSync:
		pipelineOpts.Operation = pipeline.OperationSync
	case OperationPipeline, "":
		pipelineOpts.Operation = pipeline.OperationPipeline
	default:
		result.ErrorMessage = fmt.Sprintf("Unknown operation: %s", opts.Operation)
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	// Parse output format
	switch opts.OutputFormat {
	case OutputFormatJSON:
		pipelineOpts.OutputFormat = diff.OutputFormatJSON
	case OutputFormatGitHub:
		pipelineOpts.OutputFormat = diff.OutputFormatGitHub
	case OutputFormatCompact:
		pipelineOpts.OutputFormat = diff.OutputFormatCompact
	case OutputFormatSideBySide:
		pipelineOpts.OutputFormat = diff.OutputFormatSideBySide
	default:
		pipelineOpts.OutputFormat = diff.OutputFormatHuman
	}

	// Parse targets using strings.Split (consistent with CLI behavior)
	if opts.Targets != "" {
		pipelineOpts.Targets = splitTargets(opts.Targets)
	}

	// Run pipeline
	results, err := p.Run(ctx, pipelineOpts)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Pipeline execution failed: %v", err)
	}

	// Process results
	result.TargetCount = len(results)
	result.Success = err == nil

	for _, r := range results {
		result.SecretsProcessed += r.Details.SecretsProcessed
		result.SecretsAdded += r.Details.SecretsAdded
		result.SecretsModified += r.Details.SecretsModified
		result.SecretsRemoved += r.Details.SecretsRemoved
		result.SecretsUnchanged += r.Details.SecretsUnchanged

		if !r.Success && result.Success {
			result.Success = false
			if r.Error != nil && result.ErrorMessage == "" {
				result.ErrorMessage = r.Error.Error()
			}
		}
	}

	// Serialize results to JSON
	if jsonBytes, err := json.Marshal(results); err == nil {
		result.ResultsJSON = string(jsonBytes)
	}

	// Get diff output if computed
	if opts.ComputeDiff {
		pipelineDiff := p.Diff()
		if pipelineDiff != nil {
			result.DiffOutput = diff.FormatDiffWithOptions(pipelineDiff, pipelineOpts.OutputFormat, opts.ShowValues)
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result
}

// DryRun performs a dry run of the pipeline and returns the diff
func DryRun(configPath string) *SyncResult {
	opts := DefaultSyncOptions()
	opts.DryRun = true
	opts.ComputeDiff = true
	return RunPipeline(configPath, opts)
}

// Merge runs only the merge phase of the pipeline
func Merge(configPath string, dryRun bool) *SyncResult {
	opts := DefaultSyncOptions()
	opts.Operation = OperationMerge
	opts.DryRun = dryRun
	opts.ComputeDiff = dryRun
	return RunPipeline(configPath, opts)
}

// Sync runs only the sync phase of the pipeline
func Sync(configPath string, dryRun bool) *SyncResult {
	opts := DefaultSyncOptions()
	opts.Operation = OperationSync
	opts.DryRun = dryRun
	opts.ComputeDiff = dryRun
	return RunPipeline(configPath, opts)
}

// GetTargets returns the list of targets from a configuration (sorted alphabetically)
func GetTargets(configPath string) ([]string, string) {
	cfg, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		return nil, errMsg
	}

	targets := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		targets = append(targets, name)
	}
	sort.Strings(targets)
	return targets, ""
}

// GetSources returns the list of sources from a configuration (sorted alphabetically)
func GetSources(configPath string) ([]string, string) {
	cfg, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		return nil, errMsg
	}

	sources := make([]string, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		sources = append(sources, name)
	}
	sort.Strings(sources)
	return sources, ""
}

// splitTargets splits a comma-separated string of targets into a slice.
// Uses strings.Split for consistency with CLI behavior.
func splitTargets(targets string) []string {
	if targets == "" {
		return nil
	}

	parts := strings.Split(targets, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ConfigInfo returns information about a configuration file
type ConfigInfo struct {
	Valid         bool     // Whether the configuration is valid
	ErrorMessage  string   // Error message if invalid
	SourceCount   int      // Number of sources
	TargetCount   int      // Number of targets
	Sources       []string // List of source names (sorted alphabetically)
	Targets       []string // List of target names (sorted alphabetically)
	HasMergeStore bool     // Whether a merge store is configured
	VaultAddress  string   // Vault address if configured
	AWSRegion     string   // AWS region if configured
}

// GetConfigInfo returns detailed information about a configuration
func GetConfigInfo(configPath string) *ConfigInfo {
	info := &ConfigInfo{}

	cfg, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		info.ErrorMessage = errMsg
		return info
	}

	info.Valid = true
	info.SourceCount = len(cfg.Sources)
	info.TargetCount = len(cfg.Targets)
	info.VaultAddress = cfg.Vault.Address
	info.AWSRegion = cfg.AWS.Region
	info.HasMergeStore = cfg.MergeStore.Vault != nil || cfg.MergeStore.S3 != nil

	// Build sorted slices for deterministic output
	info.Sources = make([]string, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		info.Sources = append(info.Sources, name)
	}
	sort.Strings(info.Sources)

	info.Targets = make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		info.Targets = append(info.Targets, name)
	}
	sort.Strings(info.Targets)

	return info
}
