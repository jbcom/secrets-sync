// Package pipeline provides a unified secrets synchronization pipeline.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                         Pipeline Configuration                          │
//	│  (YAML file or programmatic)                                            │
//	└─────────────────────────────────────────────────────────────────────────┘
//	                                    │
//	                                    ▼
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                           Pipeline Engine                               │
//	│  • Dependency graph resolution                                          │
//	│  • Topological ordering                                                 │
//	│  • Parallel execution within levels                                     │
//	│  • Each operation is distinct and idempotent                            │
//	└─────────────────────────────────────────────────────────────────────────┘
//	                                    │
//	          ┌─────────────────────────┴─────────────────────────┐
//	          ▼                                                   ▼
//	┌─────────────────┐                                 ┌─────────────────┐
//	│   Merge Phase   │                                 │   Sync Phase    │
//	│  Vault → Vault  │                                 │  Vault → AWS    │
//	│  (or S3)        │                                 │                 │
//	└─────────────────┘                                 └─────────────────┘
//
// Operations:
//   - merge:    Source stores → Merge store (with inheritance resolution)
//   - sync:     Merge store → Destination stores (AWS)
//   - pipeline: merge + sync in correct dependency order
//
// Each operation is distinct and idempotent. Running the same operation
// multiple times produces the same result.
package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	reqctx "github.com/jbcom/secrets-sync/pkg/context"
	"github.com/jbcom/secrets-sync/pkg/diff"
	"github.com/jbcom/secrets-sync/pkg/driver"
	"github.com/jbcom/secrets-sync/pkg/observability"
	"github.com/jbcom/secrets-sync/pkg/policy"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Operation defines what the pipeline should do
type Operation string

const (
	// OperationMerge only performs the merge phase (sources → merge store)
	OperationMerge Operation = "merge"
	// OperationSync only performs the sync phase (merge store → destinations)
	OperationSync Operation = "sync"
	// OperationPipeline performs both merge and sync in order
	OperationPipeline Operation = "pipeline"
)

// Pipeline is the main orchestrator for secrets synchronization
type Pipeline struct {
	config      *Config
	graph       *Graph
	initialized bool
	mu          sync.Mutex

	awsCtx      *AWSExecutionContext
	s3Store     *S3MergeStore
	runtimeAuth *RuntimeAuth
	backends    *driver.Registry
	tracing     *observability.TracerProvider
	policy      *policy.Engine

	results   []Result
	resultsMu sync.Mutex

	pipelineDiff *diff.PipelineDiff
	diffMu       sync.Mutex
}

// Options configures pipeline execution
type Options struct {
	Operation       Operation
	Targets         []string
	DryRun          bool
	ContinueOnError bool
	Parallelism     int
	ComputeDiff     bool
	OutputFormat    diff.OutputFormat
}

// DefaultOptions returns sensible default options
func DefaultOptions() Options {
	return Options{
		Operation:       OperationPipeline,
		DryRun:          false,
		ContinueOnError: true,
		Parallelism:     4,
		ComputeDiff:     false,
	}
}

// Result represents the outcome of a single target operation
type Result struct {
	Target    string           `json:"target"`
	Phase     string           `json:"phase"`
	Operation string           `json:"operation"`
	Success   bool             `json:"success"`
	Error     error            `json:"error,omitempty"`
	Duration  time.Duration    `json:"duration"`
	Details   ResultDetails    `json:"details,omitempty"`
	Diff      *diff.TargetDiff `json:"diff,omitempty"`
}

// ResultDetails contains additional information about the operation
type ResultDetails struct {
	SecretsProcessed int      `json:"secrets_processed,omitempty"`
	SecretsAdded     int      `json:"secrets_added,omitempty"`
	SecretsModified  int      `json:"secrets_modified,omitempty"`
	SecretsRemoved   int      `json:"secrets_removed,omitempty"`
	SecretsUnchanged int      `json:"secrets_unchanged,omitempty"`
	SourcePaths      []string `json:"source_paths,omitempty"`
	DestinationPath  string   `json:"destination_path,omitempty"`
	RoleARN          string   `json:"role_arn,omitempty"`
	FailedImports    []string `json:"failed_imports,omitempty"`
}

// New creates a new Pipeline from configuration
func New(cfg *Config) (*Pipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	graph, err := BuildGraph(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	policyEngine, err := policy.Compile(cfg.Policy)
	if err != nil {
		return nil, fmt.Errorf("invalid sync policy: %w", err)
	}

	return &Pipeline{
		config:   cfg,
		graph:    graph,
		backends: newBackendRegistry(),
		policy:   policyEngine,
	}, nil
}

// NewWithContext creates a new Pipeline with AWS execution context
func NewWithContext(ctx context.Context, cfg *Config) (*Pipeline, error) {
	return NewWithContextAndRuntimeAuth(ctx, cfg, nil)
}

// NewWithContextAndRuntimeAuth creates a new Pipeline with explicit runtime
// authentication material supplied by an embedding caller.
func NewWithContextAndRuntimeAuth(ctx context.Context, cfg *Config, auth *RuntimeAuth) (*Pipeline, error) {
	runtimeCfg := cfg
	if auth != nil {
		var err error
		runtimeCfg, err = cloneConfig(cfg)
		if err != nil {
			return nil, err
		}
		auth.applyToConfig(runtimeCfg)
	}

	p, err := New(runtimeCfg)
	if err != nil {
		return nil, err
	}
	p.runtimeAuth = auth.copy()

	// Initialize AWS execution context if configured
	if runtimeCfg.AWS.ExecutionContext.Type != "" {
		awsCtx, err := NewAWSExecutionContextWithRuntimeAuth(ctx, &runtimeCfg.AWS, p.runtimeAWSAuth())
		if err != nil {
			log.WithError(err).Warn("Failed to initialize AWS execution context")
		} else {
			p.awsCtx = awsCtx
		}
	}

	// Initialize S3 merge store if configured
	if runtimeCfg.MergeStore.S3 != nil {
		s3Store, err := NewS3MergeStoreWithRuntimeAuth(ctx, runtimeCfg.MergeStore.S3, runtimeCfg.AWS.Region, p.runtimeAWSAuth())
		if err != nil {
			log.WithError(err).Warn("Failed to initialize S3 merge store")
		} else {
			p.s3Store = s3Store
		}
	}

	// Initialize distributed tracing if configured. A disabled config installs
	// a no-op provider, so the rest of the pipeline traces unconditionally.
	tp, err := observability.InitTracing(ctx, runtimeCfg.Observability.Tracing)
	if err != nil {
		log.WithError(err).Warn("Failed to initialize tracing")
	} else {
		p.tracing = tp
	}

	return p, nil
}

// Shutdown flushes and releases pipeline-owned resources (currently the tracer
// provider). It is safe to call even when tracing was never configured.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	if p == nil || p.tracing == nil {
		return nil
	}
	return p.tracing.Shutdown(ctx)
}

func cloneConfig(cfg *Config) (*Config, error) {
	if cfg == nil {
		return nil, nil
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("clone config: %w", err)
	}
	var cloned Config
	if err := yaml.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("clone config: %w", err)
	}
	return &cloned, nil
}

// NewFromFile creates a Pipeline from a configuration file
func NewFromFile(path string) (*Pipeline, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return New(cfg)
}

// NewFromFileWithContext creates a Pipeline from a configuration file with context
func NewFromFileWithContext(ctx context.Context, path string) (*Pipeline, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return NewWithContext(ctx, cfg)
}

// Run executes the pipeline with the given options.
// Each operation (merge, sync) is distinct and idempotent.
func (p *Pipeline) Run(ctx context.Context, opts Options) ([]Result, error) {
	// Generate request ID and add to context
	reqCtx := reqctx.NewRequestContext()
	ctx = reqctx.WithRequestContext(ctx, reqCtx)

	p.mu.Lock()
	defer p.mu.Unlock()

	l := log.WithFields(log.Fields{
		"action":     "Pipeline.Run",
		"request_id": reqCtx.RequestID,
		"operation":  opts.Operation,
		"dry_run":    opts.DryRun,
	})

	p.resultsMu.Lock()
	p.results = nil
	p.resultsMu.Unlock()

	if opts.DryRun || opts.ComputeDiff {
		p.initDiff(opts.DryRun, "")
	}

	targets := p.resolveTargets(opts.Targets)
	l.WithField("target_count", len(targets)).Info("Starting pipeline execution")

	if opts.Parallelism <= 0 {
		opts.Parallelism = p.config.Pipeline.Merge.Parallel
		if opts.Parallelism <= 0 {
			opts.Parallelism = 4
		}
	}

	p.initialized = true

	var results []Result
	var err error

	switch opts.Operation {
	case OperationMerge:
		results, err = p.runMerge(ctx, targets, opts)
	case OperationSync:
		results, err = p.runSync(ctx, targets, opts)
	case OperationPipeline:
		results, err = p.runPipeline(ctx, targets, opts)
	default:
		return nil, fmt.Errorf("unknown operation: %s", opts.Operation)
	}

	if err != nil {
		l.WithError(err).WithFields(log.Fields{
			"duration_ms": reqctx.GetElapsedTime(ctx).Milliseconds(),
		}).Error("Pipeline execution failed")
	} else {
		l.WithFields(log.Fields{
			"duration_ms": reqctx.GetElapsedTime(ctx).Milliseconds(),
		}).Info("Pipeline execution completed successfully")
	}

	return results, err
}

// resolveTargets returns the targets to process, including dependencies
func (p *Pipeline) resolveTargets(requested []string) []string {
	if len(requested) == 0 {
		return p.graph.TopologicalOrder()
	}
	return p.graph.IncludeDependencies(requested)
}

// Config returns the pipeline configuration
func (p *Pipeline) Config() *Config {
	return p.config
}

// Graph returns the dependency graph
func (p *Pipeline) Graph() *Graph {
	return p.graph
}

// Results returns the results from the last Run
func (p *Pipeline) Results() []Result {
	p.resultsMu.Lock()
	defer p.resultsMu.Unlock()
	return p.results
}

// Diff returns the computed diff from the last Run
func (p *Pipeline) Diff() *diff.PipelineDiff {
	p.diffMu.Lock()
	defer p.diffMu.Unlock()
	return p.pipelineDiff
}
