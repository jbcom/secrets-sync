package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jbcom/secrets-sync/pkg/diff"
	"github.com/jbcom/secrets-sync/pkg/pipeline"
)

func TestParseOutputFormat(t *testing.T) {
	tests := map[string]diff.OutputFormat{
		"human":        diff.OutputFormatHuman,
		"json":         diff.OutputFormatJSON,
		"github":       diff.OutputFormatGitHub,
		"compact":      diff.OutputFormatCompact,
		"side-by-side": diff.OutputFormatSideBySide,
		"sidebyside":   diff.OutputFormatSideBySide,
		"side_by_side": diff.OutputFormatSideBySide,
		"unknown":      diff.OutputFormatHuman,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if actual := parseOutputFormat(input); actual != expected {
				t.Fatalf("parseOutputFormat(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func TestNewPipelineJSONSummaryAggregatesResults(t *testing.T) {
	results := []pipeline.Result{
		{
			Target:    "prod",
			Phase:     "merge",
			Operation: "merge",
			Success:   true,
			Duration:  1500 * time.Millisecond,
			Details: pipeline.ResultDetails{
				SecretsProcessed: 2,
				SecretsAdded:     1,
				SecretsUnchanged: 1,
			},
		},
		{
			Target:    "prod",
			Phase:     "sync",
			Operation: "sync",
			Success:   true,
			Duration:  2500 * time.Millisecond,
			Details: pipeline.ResultDetails{
				SecretsProcessed: 2,
				SecretsModified:  1,
				SecretsRemoved:   1,
			},
		},
		{
			Target:    "staging",
			Phase:     "sync",
			Operation: "sync",
			Success:   true,
			Duration:  time.Second,
			Details: pipeline.ResultDetails{
				SecretsProcessed: 1,
				SecretsUnchanged: 1,
			},
		},
	}
	pipelineDiff := &diff.PipelineDiff{
		Summary: diff.ChangeSummary{Added: 1, Modified: 1, Total: 2},
		DryRun:  true,
	}

	summary := newPipelineJSONSummary(results, nil, 4200*time.Millisecond, `{"summary":{"added":1}}`, pipelineDiff)

	if !summary.Success {
		t.Fatal("Success = false, want true")
	}
	if summary.TargetCount != 2 {
		t.Fatalf("TargetCount = %d, want 2", summary.TargetCount)
	}
	if summary.SecretsProcessed != 5 {
		t.Fatalf("SecretsProcessed = %d, want 5", summary.SecretsProcessed)
	}
	if summary.SecretsAdded != 1 || summary.SecretsModified != 1 || summary.SecretsRemoved != 1 || summary.SecretsUnchanged != 2 {
		t.Fatalf("unexpected secret counts: %+v", summary)
	}
	if summary.DurationMs != 4200 {
		t.Fatalf("DurationMs = %d, want 4200", summary.DurationMs)
	}
	if len(summary.Results) != len(results) {
		t.Fatalf("len(Results) = %d, want %d", len(summary.Results), len(results))
	}
	if summary.Results[0].DurationMs != 1500 {
		t.Fatalf("Results[0].DurationMs = %d, want 1500", summary.Results[0].DurationMs)
	}
	if summary.Diff == nil {
		t.Fatal("Diff = nil, want structured diff")
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal(summary) failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"target_count":2`) {
		t.Fatalf("encoded summary missing target_count: %s", encoded)
	}
}

func TestNewPipelineJSONSummaryReportsFailures(t *testing.T) {
	results := []pipeline.Result{
		{
			Target:  "prod",
			Phase:   "sync",
			Success: false,
			Error:   errors.New("assume role failed"),
		},
	}

	summary := newPipelineJSONSummary(results, nil, time.Second, "", nil)

	if summary.Success {
		t.Fatal("Success = true, want false")
	}
	if summary.ErrorMessage != "assume role failed" {
		t.Fatalf("ErrorMessage = %q, want %q", summary.ErrorMessage, "assume role failed")
	}
	if summary.Results[0].Error != "assume role failed" {
		t.Fatalf("Results[0].Error = %q, want %q", summary.Results[0].Error, "assume role failed")
	}
}

func TestPipelineHadErrors(t *testing.T) {
	tests := map[string]struct {
		err     error
		results []pipeline.Result
		want    bool
	}{
		"run error": {
			err:  errors.New("connection failed"),
			want: true,
		},
		"target failure": {
			results: []pipeline.Result{{Target: "prod", Success: false}},
			want:    true,
		},
		"all targets successful": {
			results: []pipeline.Result{{Target: "prod", Success: true}},
			want:    false,
		},
		"no results": {
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := pipelineHadErrors(tc.err, tc.results); got != tc.want {
				t.Fatalf("pipelineHadErrors() = %v, want %v", got, tc.want)
			}
		})
	}
}
