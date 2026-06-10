package cmd

import (
	"errors"
	"testing"

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
