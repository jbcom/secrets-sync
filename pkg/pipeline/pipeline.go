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

	reqctx "github.com/jbcom/extended-data-library/packages/secretssync/pkg/context"
	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/diff"
	log "github.com/sirupsen/logrus"
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

	awsCtx  *AWSExecutionContext
	s3Store *S3MergeStore

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
		ContinueOnError: false,
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

	return &Pipeline{
		config: cfg,
		graph:  graph,
	}, nil
}

// NewWithContext creates a new Pipeline with AWS execution context
func NewWithContext(ctx context.Context, cfg *Config) (*Pipeline, error) {
	p, err := New(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize AWS execution context if configured
	if cfg.AWS.ExecutionContext.Type != "" {
		awsCtx, err := NewAWSExecutionContext(ctx, &cfg.AWS)
		if err != nil {
			log.WithError(err).Warn("Failed to initialize AWS execution context")
		} else {
			p.awsCtx = awsCtx
		}
	}

	// Initialize S3 merge store if configured
	if cfg.MergeStore.S3 != nil {
		s3Store, err := NewS3MergeStore(ctx, cfg.MergeStore.S3, cfg.AWS.Region)
		if err != nil {
			log.WithError(err).Warn("Failed to initialize S3 merge store")
		} else {
			p.s3Store = s3Store
		}
	}

	return p, nil
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
		"operation":  opts.Operation,
		"dryRun":     opts.DryRun,
		"request_id": reqCtx.RequestID,
	})

	p.resultsMu.Lock()
	p.results = nil
	p.resultsMu.Unlock()

	if opts.DryRun || opts.ComputeDiff {
		p.initDiff(opts.DryRun, "")
	}

	targets := p.resolveTargets(opts.Targets)
	l.WithField("targets", targets).Info("Starting pipeline execution")

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
			"request_id":  reqCtx.RequestID,
			"duration_ms": reqctx.GetElapsedTime(ctx).Milliseconds(),
		}).Error("Pipeline execution failed")
	} else {
		l.WithFields(log.Fields{
			"request_id":  reqCtx.RequestID,
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
