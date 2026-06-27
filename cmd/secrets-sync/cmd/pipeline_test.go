package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func TestNewPipelineJSONSummaryRedactsDiagnosticSecrets(t *testing.T) {
	results := []pipeline.Result{
		{
			Target:  "prod",
			Phase:   "sync",
			Success: false,
			Error: errors.New(
				"write failed api_key=key_123 Authorization: Bearer raw_token callback=https://example.test/hook?token=tok_456",
			),
		},
	}

	summary := newPipelineJSONSummary(
		results,
		errors.New("pipeline failed password=hunter2 client_secret=secret_123"),
		time.Second,
		"",
		nil,
	)

	if summary.Success {
		t.Fatal("Success = true, want false")
	}
	for _, raw := range []string{"hunter2", "secret_123", "key_123", "raw_token", "tok_456"} {
		if strings.Contains(summary.ErrorMessage, raw) {
			t.Fatalf("ErrorMessage leaked %q: %s", raw, summary.ErrorMessage)
		}
		if strings.Contains(summary.Results[0].Error, raw) {
			t.Fatalf("Results[0].Error leaked %q: %s", raw, summary.Results[0].Error)
		}
	}
	if !strings.Contains(summary.ErrorMessage, "[REDACTED]") {
		t.Fatalf("ErrorMessage missing redaction marker: %s", summary.ErrorMessage)
	}
	if !strings.Contains(summary.Results[0].Error, "[REDACTED]") {
		t.Fatalf("Results[0].Error missing redaction marker: %s", summary.Results[0].Error)
	}
	if strings.Contains(summary.Results[0].Error, "[REDACTED] [REDACTED]") || strings.Contains(summary.Results[0].Error, "[REDACTED]]") {
		t.Fatalf("Results[0].Error should not double-redact already redacted segments: %s", summary.Results[0].Error)
	}

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal(summary) failed: %v", err)
	}
	for _, raw := range []string{"hunter2", "secret_123", "key_123", "raw_token", "tok_456"} {
		if strings.Contains(string(encoded), raw) {
			t.Fatalf("encoded summary leaked %q: %s", raw, encoded)
		}
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

func TestPipelineExitCode(t *testing.T) {
	tests := map[string]struct {
		hasErrors    bool
		diffExitCode int
		want         int
	}{
		"pipeline errors exit as execution errors": {
			hasErrors:    true,
			diffExitCode: 0,
			want:         2,
		},
		"execution errors win over changed diff": {
			hasErrors:    true,
			diffExitCode: 1,
			want:         2,
		},
		"changed diff preserves diff exit code": {
			diffExitCode: 1,
			want:         1,
		},
		"diff errors preserve diff error exit code": {
			diffExitCode: 2,
			want:         2,
		},
		"clean run exits zero": {
			want: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := pipelineExitCode(tc.hasErrors, tc.diffExitCode); got != tc.want {
				t.Fatalf("pipelineExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWriteGitHubDiffOutputs(t *testing.T) {
	pipelineDiff := &diff.PipelineDiff{
		Summary: diff.ChangeSummary{
			Added:     2,
			Removed:   1,
			Modified:  3,
			Unchanged: 5,
			Total:     11,
		},
	}
	outputPath := filepath.Join(t.TempDir(), "github_output")

	if err := writeGitHubDiffOutputs(outputPath, pipelineDiff); err != nil {
		t.Fatalf("writeGitHubDiffOutputs() failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read GitHub output file: %v", err)
	}

	text := string(content)
	for _, expected := range []string{
		"changes=6\n",
		"added=2\n",
		"removed=1\n",
		"modified=3\n",
		"unchanged=5\n",
		"zero_sum=false\n",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("GitHub output missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "::set-output") {
		t.Fatalf("GitHub output file should not contain deprecated commands:\n%s", text)
	}
}

func TestWriteGitHubDiffOutputsNoopsWithoutOutputFile(t *testing.T) {
	if err := writeGitHubDiffOutputs("", &diff.PipelineDiff{}); err != nil {
		t.Fatalf("writeGitHubDiffOutputs() with empty path failed: %v", err)
	}
	if err := writeGitHubDiffOutputs(filepath.Join(t.TempDir(), "github_output"), nil); err != nil {
		t.Fatalf("writeGitHubDiffOutputs() with nil diff failed: %v", err)
	}
}

func TestPipelineCommandRegistersShowValuesFlag(t *testing.T) {
	flag := pipelineCmd.Flags().Lookup("show-values")
	if flag == nil {
		t.Fatal("pipeline command should register --show-values flag")
	}
	if flag.DefValue != "false" {
		t.Fatalf("show-values default = %q, want \"false\"", flag.DefValue)
	}
	if !strings.Contains(flag.Usage, "values") {
		t.Fatalf("show-values usage should mention values: %q", flag.Usage)
	}
}

func TestStripDiffValuesRemovesRawSecrets(t *testing.T) {
	pd := &diff.PipelineDiff{
		Targets: []diff.TargetDiff{
			{
				Target: "prod",
				Changes: []diff.SecretChange{
					{
						Path:          "app/db",
						CurrentValues: map[string]interface{}{"password": "hunter2"},
						DesiredValues: map[string]interface{}{"password": "swordfish"},
					},
				},
			},
		},
	}
	stripDiffValues(pd)
	c := pd.Targets[0].Changes[0]
	if c.CurrentValues != nil {
		t.Fatalf("CurrentValues = %v, want nil", c.CurrentValues)
	}
	if c.DesiredValues != nil {
		t.Fatalf("DesiredValues = %v, want nil", c.DesiredValues)
	}
}

func TestStripResultDiffValuesRemovesRawSecrets(t *testing.T) {
	r := &pipeline.Result{
		Target: "prod",
		Diff: &diff.TargetDiff{
			Changes: []diff.SecretChange{
				{
					CurrentValues: map[string]interface{}{"key": "secret"},
					DesiredValues: map[string]interface{}{"key": "new-secret"},
				},
			},
		},
	}
	stripResultDiffValues(r)
	c := r.Diff.Changes[0]
	if c.CurrentValues != nil {
		t.Fatalf("CurrentValues = %v, want nil", c.CurrentValues)
	}
	if c.DesiredValues != nil {
		t.Fatalf("DesiredValues = %v, want nil", c.DesiredValues)
	}
}

func TestStripDiffValuesNoopsOnNil(t *testing.T) {
	stripDiffValues(nil)
	stripResultDiffValues(nil)
}
