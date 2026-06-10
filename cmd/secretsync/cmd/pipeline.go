package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jbcom/secrets-sync/pkg/diff"
	"github.com/jbcom/secrets-sync/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	targets         string
	mergeOnly       bool
	syncOnly        bool
	dryRun          bool
	discoverTargets bool
	outputFormat    string
	computeDiff     bool
	exitCodeMode    bool
	continueOnError bool
	parallelism     int
)

// pipelineCmd runs the full merge-then-sync pipeline
var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run the full secrets pipeline (merge → sync)",
	Long: `Runs the complete secrets synchronization pipeline:

1. MERGE PHASE: Aggregate secrets from sources into the merge store
   - Processes targets in dependency order (base before derived)
   - Supports inheritance (Prod inherits from Stg)
   - Uses Vault merge mode for aggregation

2. SYNC PHASE: Sync merged secrets to target AWS accounts
   - Assumes Control Tower execution role in each account
   - Runs in parallel (respects --parallelism or config settings)

3. DIFF REPORTING: Track and report all changes
   - Zero-sum validation for migration verification
   - Multiple output formats (human, JSON, GitHub Actions, compact, side-by-side)
   - CI/CD-friendly exit codes (0=no changes, 1=changes, 2=errors)

Examples:
  # Full pipeline
  secretsync pipeline --config config.yaml

  # Dry run with machine-readable result and nested diff output
  secretsync pipeline --config config.yaml --dry-run --output json

  # CI/CD mode with exit codes
  secretsync pipeline --config config.yaml --dry-run --exit-code
  # Returns: 0 if no changes, 1 if changes detected, 2 on errors

  # GitHub Actions compatible output
  secretsync pipeline --config config.yaml --dry-run --output github

  # Visual side-by-side diff output
  secretsync pipeline --config config.yaml --dry-run --output side-by-side

  # Specific targets only
  secretsync pipeline --config config.yaml --targets "Serverless_Stg,Serverless_Prod"

  # Merge only (no AWS sync)
  secretsync pipeline --config config.yaml --merge-only

  # Compute diff even when applying changes (for audit trail)
  secretsync pipeline --config config.yaml --diff`,
	RunE: runPipeline,
}

func init() {
	rootCmd.AddCommand(pipelineCmd)

	pipelineCmd.Flags().StringVar(&targets, "targets", "", "comma-separated list of targets (default: all)")
	pipelineCmd.Flags().BoolVar(&mergeOnly, "merge-only", false, "only run merge phase")
	pipelineCmd.Flags().BoolVar(&syncOnly, "sync-only", false, "only run sync phase")
	pipelineCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run mode (no changes)")
	pipelineCmd.Flags().BoolVar(&discoverTargets, "discover", false, "enable dynamic target discovery from AWS Organizations/Identity Center")
	pipelineCmd.Flags().BoolVar(&continueOnError, "continue-on-error", true, "continue processing remaining targets after an error")
	pipelineCmd.Flags().IntVar(&parallelism, "parallelism", 0, "max concurrent target operations (default: pipeline.merge.parallel config or 4)")

	// Diff and output options
	pipelineCmd.Flags().StringVarP(&outputFormat, "output", "o", "human", "output format: human, json, github, compact, side-by-side")
	pipelineCmd.Flags().BoolVar(&computeDiff, "diff", false, "compute and show diff even when not in dry-run mode")
	pipelineCmd.Flags().BoolVar(&exitCodeMode, "exit-code", false, "use exit codes: 0=no changes, 1=changes, 2=errors (useful for CI/CD)")
}

func runPipeline(cmd *cobra.Command, args []string) error {
	l := log.WithFields(log.Fields{
		"action": "runPipeline",
	})

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create pipeline from config file
	var p *pipeline.Pipeline
	var err error

	if discoverTargets {
		// Use context-aware constructor for dynamic target discovery
		l.Info("Dynamic target discovery enabled")
		p, err = pipeline.NewFromFileWithContext(ctx, cfgFile)
	} else {
		p, err = pipeline.NewFromFile(cfgFile)
	}
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		l.Warn("Received shutdown signal")
		cancel()
	}()

	// Parse targets
	var targetList []string
	if targets != "" {
		targetList = strings.Split(targets, ",")
		for i := range targetList {
			targetList[i] = strings.TrimSpace(targetList[i])
		}
	}

	// Determine operation
	op := pipeline.OperationPipeline
	if mergeOnly {
		op = pipeline.OperationMerge
	} else if syncOnly {
		op = pipeline.OperationSync
	}

	// Parse output format
	format := parseOutputFormat(outputFormat)

	// Run options
	opts := pipeline.Options{
		Operation:       op,
		Targets:         targetList,
		DryRun:          dryRun,
		ContinueOnError: continueOnError,
		Parallelism:     parallelism,
		OutputFormat:    format,
		ComputeDiff:     computeDiff || dryRun,
	}

	l.WithFields(log.Fields{
		"config":       cfgFile,
		"targets":      targetList,
		"operation":    op,
		"dryRun":       dryRun,
		"outputFormat": format,
	}).Info("Starting pipeline")

	// Run pipeline
	start := time.Now()
	results, err := p.Run(ctx, opts)
	duration := time.Since(start)

	// Print machine JSON as a stable result envelope for both diff and non-diff runs.
	pipelineDiff := p.Diff()
	diffOutput := ""
	if pipelineDiff != nil {
		diffOutput = p.FormatDiff(format)
	}
	if format == diff.OutputFormatJSON {
		if jsonErr := printPipelineJSONSummary(results, err, duration, diffOutput, pipelineDiff); jsonErr != nil {
			return jsonErr
		}
	} else if pipelineDiff != nil {
		if diffOutput != "" {
			fmt.Println(diffOutput)
		}
	} else {
		// Fall back to traditional results format
		printResults(results)
	}

	// Determine exit behavior
	hasErrors := pipelineHadErrors(err, results)
	if exitCodeMode {
		if hasErrors {
			cancel()
			os.Exit(2)
		}
		exitCode := p.ExitCode()
		if exitCode != 0 {
			cancel()
			os.Exit(exitCode)
		}
		return nil
	}

	if err != nil {
		return err
	}

	// Check for any failures
	if hasErrors {
		return fmt.Errorf("pipeline completed with errors")
	}

	l.Info("Pipeline completed successfully")
	return nil
}

func pipelineHadErrors(err error, results []pipeline.Result) bool {
	if err != nil {
		return true
	}

	for _, r := range results {
		if !r.Success {
			return true
		}
	}

	return false
}

// parseOutputFormat converts string to OutputFormat
func parseOutputFormat(s string) diff.OutputFormat {
	switch strings.ToLower(s) {
	case "json":
		return diff.OutputFormatJSON
	case "github":
		return diff.OutputFormatGitHub
	case "compact":
		return diff.OutputFormatCompact
	case "side-by-side", "sidebyside", "side_by_side":
		return diff.OutputFormatSideBySide
	default:
		return diff.OutputFormatHuman
	}
}

func printResults(results []pipeline.Result) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Pipeline Results")
	fmt.Println(strings.Repeat("=", 60))

	var mergeResults, syncResults []pipeline.Result
	for _, r := range results {
		if r.Phase == "merge" {
			mergeResults = append(mergeResults, r)
		} else {
			syncResults = append(syncResults, r)
		}
	}

	// Sort results by target name for deterministic output
	sort.Slice(mergeResults, func(i, j int) bool { return mergeResults[i].Target < mergeResults[j].Target })
	sort.Slice(syncResults, func(i, j int) bool { return syncResults[i].Target < syncResults[j].Target })

	if len(mergeResults) > 0 {
		fmt.Println("\nMerge Phase:")
		for _, r := range mergeResults {
			status := "✅"
			if !r.Success {
				status = "❌"
			}
			fmt.Printf("  %s %s (%.2fs)\n", status, r.Target, r.Duration.Seconds())
			if r.Error != nil {
				fmt.Printf("      Error: %v\n", r.Error)
			}
		}
	}

	if len(syncResults) > 0 {
		fmt.Println("\nSync Phase:")
		for _, r := range syncResults {
			status := "✅"
			if !r.Success {
				status = "❌"
			}
			fmt.Printf("  %s %s (%.2fs)\n", status, r.Target, r.Duration.Seconds())
			if r.Error != nil {
				fmt.Printf("      Error: %v\n", r.Error)
			}
		}
	}

	// Count successes/failures
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	fmt.Printf("\nTotal: %d/%d succeeded\n", successCount, len(results))
	fmt.Println(strings.Repeat("=", 60))
}

type pipelineJSONSummary struct {
	Success          bool               `json:"success"`
	TargetCount      int                `json:"target_count"`
	SecretsProcessed int                `json:"secrets_processed"`
	SecretsAdded     int                `json:"secrets_added"`
	SecretsModified  int                `json:"secrets_modified"`
	SecretsRemoved   int                `json:"secrets_removed"`
	SecretsUnchanged int                `json:"secrets_unchanged"`
	DurationMs       int64              `json:"duration_ms"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	Results          []pipelineJSONItem `json:"results"`
	DiffOutput       string             `json:"diff_output,omitempty"`
	Diff             *diff.PipelineDiff `json:"diff,omitempty"`
}

type pipelineJSONItem struct {
	Target     string                 `json:"target"`
	Phase      string                 `json:"phase"`
	Operation  string                 `json:"operation"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
	Details    pipeline.ResultDetails `json:"details,omitempty"`
	Diff       *diff.TargetDiff       `json:"diff,omitempty"`
}

func printPipelineJSONSummary(
	results []pipeline.Result,
	runErr error,
	duration time.Duration,
	diffOutput string,
	pipelineDiff *diff.PipelineDiff,
) error {
	payload := newPipelineJSONSummary(results, runErr, duration, diffOutput, pipelineDiff)
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode pipeline JSON output: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func newPipelineJSONSummary(
	results []pipeline.Result,
	runErr error,
	duration time.Duration,
	diffOutput string,
	pipelineDiff *diff.PipelineDiff,
) pipelineJSONSummary {
	summary := pipelineJSONSummary{
		Success:    runErr == nil,
		DurationMs: duration.Milliseconds(),
		Results:    make([]pipelineJSONItem, 0, len(results)),
		DiffOutput: diffOutput,
		Diff:       pipelineDiff,
	}
	if runErr != nil {
		summary.ErrorMessage = runErr.Error()
	}

	targetsSeen := make(map[string]struct{})
	for _, result := range results {
		if result.Target != "" {
			targetsSeen[result.Target] = struct{}{}
		}

		summary.SecretsProcessed += result.Details.SecretsProcessed
		summary.SecretsAdded += result.Details.SecretsAdded
		summary.SecretsModified += result.Details.SecretsModified
		summary.SecretsRemoved += result.Details.SecretsRemoved
		summary.SecretsUnchanged += result.Details.SecretsUnchanged

		item := pipelineJSONItem{
			Target:     result.Target,
			Phase:      result.Phase,
			Operation:  result.Operation,
			Success:    result.Success,
			DurationMs: result.Duration.Milliseconds(),
			Details:    result.Details,
			Diff:       result.Diff,
		}
		if result.Error != nil {
			item.Error = result.Error.Error()
		}
		summary.Results = append(summary.Results, item)

		if !result.Success {
			summary.Success = false
			if summary.ErrorMessage == "" {
				if item.Error != "" {
					summary.ErrorMessage = item.Error
				} else {
					summary.ErrorMessage = "pipeline completed with errors"
				}
			}
		}
	}

	summary.TargetCount = len(targetsSeen)
	return summary
}
