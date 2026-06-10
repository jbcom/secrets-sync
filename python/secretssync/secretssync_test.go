package secretssync

import (
	"testing"

	"github.com/jbcom/secrets-sync/pkg/pipeline"
)

func TestDefaultSyncOptions(t *testing.T) {
	opts := DefaultSyncOptions()

	if opts.Operation != OperationPipeline {
		t.Fatalf("Operation = %q, want %q", opts.Operation, OperationPipeline)
	}
	if opts.DryRun {
		t.Fatal("DryRun = true, want false")
	}
	if !opts.ContinueOnError {
		t.Fatal("ContinueOnError = false, want true")
	}
	if opts.Parallelism != 4 {
		t.Fatalf("Parallelism = %d, want 4", opts.Parallelism)
	}
	if opts.OutputFormat != OutputFormatHuman {
		t.Fatalf("OutputFormat = %q, want %q", opts.OutputFormat, OutputFormatHuman)
	}
}

func TestCountUniqueTargets(t *testing.T) {
	results := []pipeline.Result{
		{Target: "prod", Phase: "merge"},
		{Target: "prod", Phase: "sync"},
		{Target: "staging", Phase: "sync"},
		{Phase: "sync"},
	}

	if got := countUniqueTargets(results); got != 2 {
		t.Fatalf("countUniqueTargets() = %d, want 2", got)
	}
}
