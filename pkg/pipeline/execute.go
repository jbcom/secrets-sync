package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	reqctx "github.com/extended-data-library/secretssync/pkg/context"
	"github.com/extended-data-library/secretssync/pkg/observability"
	log "github.com/sirupsen/logrus"
)

// runMerge executes only the merge phase
func (p *Pipeline) runMerge(ctx context.Context, targets []string, opts Options) ([]Result, error) {
	startTime := time.Now()
	requestID := reqctx.GetRequestID(ctx)
	defer func() {
		observability.RecordDuration(observability.PipelineExecutionDuration, startTime, "merge", string(opts.Operation))
	}()

	l := log.WithFields(log.Fields{
		"action":     "Pipeline.runMerge",
		"targets":    targets,
		"request_id": requestID,
	})
	l.Info("Starting merge phase")

	results, err := p.executeMergePhase(ctx, targets, opts)
	p.resultsMu.Lock()
	p.results = results
	p.resultsMu.Unlock()
	return results, err
}

// runSync executes only the sync phase
func (p *Pipeline) runSync(ctx context.Context, targets []string, opts Options) ([]Result, error) {
	startTime := time.Now()
	requestID := reqctx.GetRequestID(ctx)
	defer func() {
		observability.RecordDuration(observability.PipelineExecutionDuration, startTime, "sync", string(opts.Operation))
	}()

	l := log.WithFields(log.Fields{
		"action":     "Pipeline.runSync",
		"targets":    targets,
		"request_id": requestID,
	})
	l.Info("Starting sync phase")

	results, err := p.executeSyncPhase(ctx, targets, opts)
	p.resultsMu.Lock()
	p.results = results
	p.resultsMu.Unlock()
	return results, err
}

// runPipeline executes both merge and sync phases
func (p *Pipeline) runPipeline(ctx context.Context, targets []string, opts Options) ([]Result, error) {
	requestID := reqctx.GetRequestID(ctx)
	l := log.WithFields(log.Fields{
		"action":     "Pipeline.runPipeline",
		"targets":    targets,
		"request_id": requestID,
	})
	l.Info("Starting full pipeline (merge + sync)")

	var allResults []Result

	// Merge phase
	l.Info("Phase 1: Merge")
	mergeResults, mergeErr := p.executeMergePhase(ctx, targets, opts)
	allResults = append(allResults, mergeResults...)

	if mergeErr != nil && !opts.ContinueOnError {
		p.resultsMu.Lock()
		p.results = allResults
		p.resultsMu.Unlock()
		return allResults, fmt.Errorf("merge phase failed: %w", mergeErr)
	}

	// Sync phase
	l.Info("Phase 2: Sync")
	syncResults, syncErr := p.executeSyncPhase(ctx, targets, opts)
	allResults = append(allResults, syncResults...)

	p.resultsMu.Lock()
	p.results = allResults
	p.resultsMu.Unlock()

	if syncErr != nil {
		return allResults, fmt.Errorf("sync phase failed: %w", syncErr)
	}

	return allResults, nil
}

// executeMergePhase runs merge operations in dependency order
func (p *Pipeline) executeMergePhase(ctx context.Context, targets []string, opts Options) ([]Result, error) {
	var results []Result
	var lastErr error

	// Process by dependency level
	levels := p.graph.GroupByLevel()

	for levelIdx, level := range levels {
		// Filter to requested targets
		var levelTargets []string
		for _, t := range level {
			for _, requested := range targets {
				if t == requested {
					levelTargets = append(levelTargets, t)
					break
				}
			}
		}

		if len(levelTargets) == 0 {
			continue
		}

		log.WithFields(log.Fields{
			"level":   levelIdx,
			"targets": levelTargets,
		}).Debug("Processing merge level")

		// Execute level in parallel
		levelResults := p.executeParallel(ctx, levelTargets, opts.Parallelism, func(target string) Result {
			return p.mergeTarget(ctx, target, opts.DryRun)
		})

		results = append(results, levelResults...)

		// Check for errors and record metrics
		for _, r := range levelResults {
			if r.Success {
				observability.PipelineTargetsProcessed.WithLabelValues("merge", "success").Inc()
			} else {
				observability.PipelineTargetsProcessed.WithLabelValues("merge", "error").Inc()
				observability.PipelineErrors.WithLabelValues("merge", "target_error").Inc()
				lastErr = r.Error
				if !opts.ContinueOnError {
					return results, lastErr
				}
			}
		}
	}

	return results, lastErr
}

// executeSyncPhase runs sync operations (can be fully parallel)
func (p *Pipeline) executeSyncPhase(ctx context.Context, targets []string, opts Options) ([]Result, error) {
	results := p.executeParallel(ctx, targets, opts.Parallelism, func(target string) Result {
		return p.syncTarget(ctx, target, opts.DryRun)
	})

	var lastErr error
	for _, r := range results {
		if r.Success {
			observability.PipelineTargetsProcessed.WithLabelValues("sync", "success").Inc()
		} else {
			observability.PipelineTargetsProcessed.WithLabelValues("sync", "error").Inc()
			observability.PipelineErrors.WithLabelValues("sync", "target_error").Inc()
			lastErr = r.Error
			if !opts.ContinueOnError {
				return results, lastErr
			}
		}
	}

	return results, lastErr
}

// executeParallel runs a function for each target with limited concurrency
func (p *Pipeline) executeParallel(ctx context.Context, targets []string, maxParallel int, fn func(string) Result) []Result {
	if maxParallel <= 0 {
		maxParallel = 1
	}

	results := make([]Result, len(targets))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	for i, target := range targets {
		select {
		case <-ctx.Done():
			results[i] = Result{
				Target:  target,
				Success: false,
				Error:   ctx.Err(),
			}
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		observability.PipelineParallelWorkers.WithLabelValues("execute").Inc()

		go func(idx int, t string) {
			defer wg.Done()
			defer func() {
				<-sem
				observability.PipelineParallelWorkers.WithLabelValues("execute").Dec()
			}()
			results[idx] = fn(t)
		}(i, target)
	}

	wg.Wait()
	return results
}
