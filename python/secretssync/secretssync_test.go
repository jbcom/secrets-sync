package secretssync

import "testing"

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
