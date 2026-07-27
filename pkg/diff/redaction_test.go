package diff

import (
	"strings"
	"testing"
)

// Raw secret values live on SecretChange so the side-by-side formatter can show
// them on request. Every other output path must redact them. Formatting is the
// single chokepoint every caller goes through, so the guarantee belongs there
// rather than in each caller -- a caller that forgets is a silent credential
// leak, which is exactly how the serverless adapter came to emit plaintext.
const (
	secretValue    = "super-secret-password-do-not-leak"
	secretKeyName  = "db_password"
	anotherSecret  = "AKIA-not-a-real-key-but-shaped-like-one"
	targetName     = "production"
	secretPathName = "app/db"
)

func diffWithValues() *PipelineDiff {
	summary := ChangeSummary{Modified: 1, Total: 1}

	return &PipelineDiff{
		DryRun:  true,
		Summary: summary,
		Targets: []TargetDiff{
			{
				Target:  targetName,
				Summary: summary,
				Changes: []SecretChange{
					{
						Path:          secretPathName,
						ChangeType:    ChangeTypeModified,
						Target:        targetName,
						KeysModified:  []string{secretKeyName},
						CurrentValues: map[string]interface{}{secretKeyName: secretValue},
						DesiredValues: map[string]interface{}{secretKeyName: anotherSecret},
					},
				},
			},
		},
	}
}

// Every format must withhold values when showValues is false. Table-driven so a
// newly added OutputFormat is covered by adding one line, not by remembering to
// write a whole test.
func TestFormatDiffRedactsValuesByDefault(t *testing.T) {
	formats := []struct {
		name   string
		format OutputFormat
	}{
		{"json", OutputFormatJSON},
		{"github", OutputFormatGitHub},
		{"compact", OutputFormatCompact},
		{"side-by-side", OutputFormatSideBySide},
		{"human", OutputFormatHuman},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			out := FormatDiffWithOptions(diffWithValues(), tc.format, false)

			for _, leaked := range []string{secretValue, anotherSecret} {
				if strings.Contains(out, leaked) {
					t.Errorf("%s output leaked a secret value with showValues=false:\n%s", tc.name, out)
				}
			}

			// Redaction must not cost the operator the information they need:
			// which secret changed. The github and compact formats are
			// deliberately summary-only and never name individual paths, so
			// this applies to the per-secret formats.
			switch tc.format {
			case OutputFormatJSON, OutputFormatSideBySide, OutputFormatHuman:
				if !strings.Contains(out, secretPathName) {
					t.Errorf("%s output dropped the secret path, making the diff unusable:\n%s", tc.name, out)
				}
			}
		})
	}
}

// Redacting must not mutate the caller's diff. The pipeline reuses the struct
// after formatting -- for audit records and for the returned Result -- so a
// destructive strip would blank data those consumers legitimately hold.
func TestFormatDiffDoesNotMutateCallerDiff(t *testing.T) {
	d := diffWithValues()

	FormatDiffWithOptions(d, OutputFormatJSON, false)

	got := d.Targets[0].Changes[0].CurrentValues
	if got == nil {
		t.Fatal("formatting destroyed the caller's CurrentValues; redaction must operate on a copy")
	}
	if got[secretKeyName] != secretValue {
		t.Errorf("formatting altered the caller's value: got %v, want %q", got[secretKeyName], secretValue)
	}
}

// The escape hatch must still work, otherwise --show-values is broken.
func TestFormatDiffShowsValuesWhenRequested(t *testing.T) {
	out := FormatDiffWithOptions(diffWithValues(), OutputFormatJSON, true)

	if !strings.Contains(out, secretValue) {
		t.Errorf("showValues=true should reveal values, got:\n%s", out)
	}
}

// A change carrying no values must not be corrupted into carrying empty maps,
// which would make "no values captured" indistinguishable from "values redacted".
func TestFormatDiffHandlesChangesWithoutValues(t *testing.T) {
	d := &PipelineDiff{
		Targets: []TargetDiff{{
			Target:  targetName,
			Changes: []SecretChange{{Path: secretPathName, ChangeType: ChangeTypeAdded}},
		}},
	}

	if out := FormatDiffWithOptions(d, OutputFormatJSON, false); !strings.Contains(out, secretPathName) {
		t.Errorf("expected the path in output, got:\n%s", out)
	}
}
