// Package secrets_sync provides Python bindings for the secrets-sync pipeline.
//
// This package exposes the core secrets synchronization functionality for use
// from Python via gopy-generated bindings.
package secrets_sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jbcom/secrets-sync/pkg/diff"
	"github.com/jbcom/secrets-sync/pkg/pipeline"
)

// Version of the Python binding API contract. Wheel release versions are
// patched from the release tag during packaging.
const Version = "0.1.0"

// Operation constants for Python clients.
const (
	OperationPipeline = "pipeline"
	OperationMerge    = "merge"
	OperationSync     = "sync"
)

// OutputFormat constants for Python clients.
const (
	OutputFormatHuman      = "human"
	OutputFormatJSON       = "json"
	OutputFormatGitHub     = "github"
	OutputFormatCompact    = "compact"
	OutputFormatSideBySide = "side-by-side"
)

// PipelineConfig represents a pipeline configuration in a Python-friendly format.
type PipelineConfig struct {
	Path string // Path to YAML configuration file.
}

// SyncOptions configures pipeline execution.
type SyncOptions struct {
	DryRun          bool   // If true, do not make actual changes.
	Operation       string // "merge", "sync", or "pipeline".
	Targets         string // Comma-separated list of targets. Empty means all targets.
	ContinueOnError bool   // Continue on errors.
	Parallelism     int    // Number of parallel operations.
	ComputeDiff     bool   // Compute and return diff.
	OutputFormat    string // "human", "json", "github", "compact", or "side-by-side".
	ShowValues      bool   // If true, show unmasked secret values in diff output.
}

// ProviderSession carries authenticated provider material from an upstream
// Python package into the Go runtime. Set DelegateAuth to true when the caller
// wants secrets-sync to use its normal environment/config authentication path.
type ProviderSession struct {
	DelegateAuth       bool   // If true, ignore explicit session fields and let secrets-sync authenticate.
	VaultAddress       string // Vault address for an upstream-owned Vault session.
	VaultNamespace     string // Vault namespace for the upstream-owned Vault session.
	VaultToken         string // Vault token for the upstream-owned Vault session.
	AWSRegion          string // AWS region for the upstream-owned AWS session.
	AWSAccessKeyID     string // AWS access key ID for the upstream-owned AWS session.
	AWSSecretAccessKey string // AWS secret access key for the upstream-owned AWS session.
	AWSSessionToken    string // AWS session token for temporary credentials.
	AWSRoleARN         string // Optional role ARN to assume from the supplied AWS session.
	AWSEndpointURL     string // Optional AWS endpoint override for local or custom providers.
}

// SyncResult represents the outcome of a sync operation.
type SyncResult struct {
	Success          bool   // Overall success status.
	TargetCount      int    // Number of targets processed.
	SecretsProcessed int    // Total secrets processed.
	SecretsAdded     int    // Secrets added.
	SecretsModified  int    // Secrets modified.
	SecretsRemoved   int    // Secrets removed.
	SecretsUnchanged int    // Secrets unchanged.
	DurationMs       int64  // Duration in milliseconds.
	ErrorMessage     string // Error message if failed.
	ResultsJSON      string // Full results as JSON.
	DiffOutput       string // Diff output if computed.
}

// ValidationResult represents configuration validation status.
type ValidationResult struct {
	Valid        bool   // Whether the configuration is valid.
	Message      string // Human-readable validation message.
	ErrorMessage string // Error message if validation failed.
}

// StringListResult represents a named list returned through gopy.
type StringListResult struct {
	Success      bool     // Whether the list was read successfully.
	ErrorMessage string   // Error message if the read failed.
	Values       []string // Sorted values.
}

// DefaultSyncOptions returns sensible default options.
func DefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		DryRun:          false,
		Operation:       OperationPipeline,
		Targets:         "",
		ContinueOnError: true,
		Parallelism:     4,
		ComputeDiff:     false,
		OutputFormat:    OutputFormatHuman,
		ShowValues:      false,
	}
}

// NewProviderSession creates an empty provider session.
func NewProviderSession() *ProviderSession {
	return &ProviderSession{}
}

// NewPipelineConfig creates a new pipeline configuration from a file path.
func NewPipelineConfig(path string) *PipelineConfig {
	return &PipelineConfig{Path: path}
}

func loadConfig(configPath string) (*pipeline.Config, string) {
	cfg, err := pipeline.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Sprintf("failed to load config: %v", err)
	}
	return cfg, ""
}

func loadAndValidateConfig(configPath string) (*pipeline.Config, string) {
	cfg, errMsg := loadConfig(configPath)
	if errMsg != "" {
		return nil, errMsg
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Sprintf("invalid configuration: %v", err)
	}
	return cfg, ""
}

// ValidateConfig validates a pipeline configuration file.
func ValidateConfig(configPath string) *ValidationResult {
	_, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		return &ValidationResult{
			Valid:        false,
			Message:      errMsg,
			ErrorMessage: errMsg,
		}
	}
	return &ValidationResult{
		Valid:   true,
		Message: "configuration is valid",
	}
}

// RunPipeline executes the secrets synchronization pipeline.
func RunPipeline(configPath string, opts *SyncOptions) *SyncResult {
	return RunPipelineWithSession(configPath, opts, nil)
}

// RunPipelineWithSession executes the pipeline with caller-supplied provider
// session material.
func RunPipelineWithSession(configPath string, opts *SyncOptions, session *ProviderSession) *SyncResult {
	result := &SyncResult{}
	startTime := time.Now()

	if opts == nil {
		opts = DefaultSyncOptions()
	}

	cfg, errMsg := loadConfig(configPath)
	if errMsg != "" {
		result.ErrorMessage = errMsg
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	ctx := context.Background()
	p, err := pipeline.NewWithContextAndRuntimeAuth(ctx, cfg, runtimeAuthFromSession(session))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to create pipeline: %v", err)
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

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
		result.ErrorMessage = fmt.Sprintf("unknown operation: %s", opts.Operation)
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	pipelineOpts.OutputFormat = parseOutputFormat(opts.OutputFormat)
	pipelineOpts.Targets = splitTargets(opts.Targets)

	results, err := p.Run(ctx, pipelineOpts)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("pipeline execution failed: %v", err)
	}

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

	if jsonBytes, err := json.Marshal(results); err == nil {
		result.ResultsJSON = string(jsonBytes)
	}

	if opts.ComputeDiff {
		pipelineDiff := p.Diff()
		if pipelineDiff != nil {
			result.DiffOutput = diff.FormatDiffWithOptions(pipelineDiff, pipelineOpts.OutputFormat, opts.ShowValues)
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result
}

// DryRun performs a dry run of the pipeline and returns the diff.
func DryRun(configPath string) *SyncResult {
	return DryRunWithSession(configPath, nil)
}

// DryRunWithSession performs a dry run with caller-supplied provider session material.
func DryRunWithSession(configPath string, session *ProviderSession) *SyncResult {
	opts := DefaultSyncOptions()
	opts.DryRun = true
	opts.ComputeDiff = true
	return RunPipelineWithSession(configPath, opts, session)
}

// Merge runs only the merge phase of the pipeline.
func Merge(configPath string, dryRun bool) *SyncResult {
	return MergeWithSession(configPath, dryRun, nil)
}

// MergeWithSession runs only the merge phase with caller-supplied provider session material.
func MergeWithSession(configPath string, dryRun bool, session *ProviderSession) *SyncResult {
	opts := DefaultSyncOptions()
	opts.Operation = OperationMerge
	opts.DryRun = dryRun
	opts.ComputeDiff = dryRun
	return RunPipelineWithSession(configPath, opts, session)
}

// Sync runs only the sync phase of the pipeline.
func Sync(configPath string, dryRun bool) *SyncResult {
	return SyncWithSession(configPath, dryRun, nil)
}

// SyncWithSession runs only the sync phase with caller-supplied provider session material.
func SyncWithSession(configPath string, dryRun bool, session *ProviderSession) *SyncResult {
	opts := DefaultSyncOptions()
	opts.Operation = OperationSync
	opts.DryRun = dryRun
	opts.ComputeDiff = dryRun
	return RunPipelineWithSession(configPath, opts, session)
}

// GetTargets returns the target names from a configuration, sorted alphabetically.
func GetTargets(configPath string) *StringListResult {
	cfg, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		return &StringListResult{ErrorMessage: errMsg}
	}

	targets := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		targets = append(targets, name)
	}
	sort.Strings(targets)
	return &StringListResult{Success: true, Values: targets}
}

// GetSources returns the source names from a configuration, sorted alphabetically.
func GetSources(configPath string) *StringListResult {
	cfg, errMsg := loadAndValidateConfig(configPath)
	if errMsg != "" {
		return &StringListResult{ErrorMessage: errMsg}
	}

	sources := make([]string, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		sources = append(sources, name)
	}
	sort.Strings(sources)
	return &StringListResult{Success: true, Values: sources}
}

// ConfigInfo returns information about a configuration file.
type ConfigInfo struct {
	Valid         bool     // Whether the configuration is valid.
	ErrorMessage  string   // Error message if invalid.
	SourceCount   int      // Number of sources.
	TargetCount   int      // Number of targets.
	Sources       []string // List of source names, sorted alphabetically.
	Targets       []string // List of target names, sorted alphabetically.
	HasMergeStore bool     // Whether a merge store is configured.
	VaultAddress  string   // Vault address if configured.
	AWSRegion     string   // AWS region if configured.
}

// GetConfigInfo returns detailed information about a configuration.
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

func parseOutputFormat(format string) diff.OutputFormat {
	switch strings.ToLower(format) {
	case OutputFormatJSON:
		return diff.OutputFormatJSON
	case OutputFormatGitHub:
		return diff.OutputFormatGitHub
	case OutputFormatCompact:
		return diff.OutputFormatCompact
	case OutputFormatSideBySide, "sidebyside", "side_by_side":
		return diff.OutputFormatSideBySide
	default:
		return diff.OutputFormatHuman
	}
}

func splitTargets(targets string) []string {
	if targets == "" {
		return nil
	}

	parts := strings.Split(targets, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func runtimeAuthFromSession(session *ProviderSession) *pipeline.RuntimeAuth {
	if session == nil {
		return nil
	}

	auth := &pipeline.RuntimeAuth{DelegateAuth: session.DelegateAuth}
	if auth.DelegateAuth {
		return auth
	}
	if session.VaultAddress != "" || session.VaultNamespace != "" || session.VaultToken != "" {
		auth.Vault = &pipeline.VaultRuntimeAuth{
			Address:   session.VaultAddress,
			Namespace: session.VaultNamespace,
			Token:     session.VaultToken,
		}
	}
	if session.AWSRegion != "" ||
		session.AWSAccessKeyID != "" ||
		session.AWSSecretAccessKey != "" ||
		session.AWSSessionToken != "" ||
		session.AWSRoleARN != "" ||
		session.AWSEndpointURL != "" {
		auth.AWS = &pipeline.AWSRuntimeAuth{
			Region:          session.AWSRegion,
			AccessKeyID:     session.AWSAccessKeyID,
			SecretAccessKey: session.AWSSecretAccessKey,
			SessionToken:    session.AWSSessionToken,
			RoleARN:         session.AWSRoleARN,
			EndpointURL:     session.AWSEndpointURL,
		}
	}

	if auth.DelegateAuth && auth.Vault == nil && auth.AWS == nil {
		return auth
	}
	if auth.Vault == nil && auth.AWS == nil {
		return nil
	}
	return auth
}
