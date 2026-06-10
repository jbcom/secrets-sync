package cmd

import (
	"testing"

	"github.com/jbcom/secrets-sync/pkg/diff"
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
