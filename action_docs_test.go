package secrets_sync_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionMetadata struct {
	Inputs  map[string]actionInput  `yaml:"inputs"`
	Outputs map[string]actionOutput `yaml:"outputs"`
}

type actionInput struct {
	Default string `yaml:"default"`
}

type actionOutput struct {
	Description string `yaml:"description"`
}

func TestActionInputDocsMatchMetadata(t *testing.T) {
	actionInputs := readActionInputDefaults(t)

	for _, doc := range []struct {
		path    string
		heading string
	}{
		{"docs/GITHUB_ACTIONS.md", "## Input Parameters"},
		{"docs/ACTION_QUICK_REFERENCE.md", "## All Inputs"},
	} {
		docInputs := readDocumentedInputDefaults(t, doc.path, doc.heading)
		if diff := compareInputDefaults(actionInputs, docInputs); len(diff) > 0 {
			t.Fatalf("%s action input table must match action.yml:\n%s", doc.path, strings.Join(diff, "\n"))
		}
	}
}

func TestActionOutputDocsMatchMetadata(t *testing.T) {
	metadata := readActionMetadata(t)
	if len(metadata.Outputs) == 0 {
		t.Fatal("action.yml should declare outputs")
	}

	for _, path := range []string{"docs/GITHUB_ACTIONS.md", "docs/ACTION_QUICK_REFERENCE.md"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		for name := range metadata.Outputs {
			if !strings.Contains(text, "`"+name+"`") {
				t.Fatalf("%s should document action output %q", path, name)
			}
		}
	}
}

func readActionInputDefaults(t *testing.T) map[string]string {
	t.Helper()

	metadata := readActionMetadata(t)
	inputs := make(map[string]string, len(metadata.Inputs))
	for name, input := range metadata.Inputs {
		inputs[name] = input.Default
	}
	return inputs
}

func readActionMetadata(t *testing.T) actionMetadata {
	t.Helper()

	content, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}

	var metadata actionMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("parse action.yml: %v", err)
	}
	return metadata
}

func readDocumentedInputDefaults(t *testing.T, path string, heading string) map[string]string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	lines := strings.Split(string(content), "\n")
	inSection := false
	inputColumn := -1
	defaultColumn := -1
	inputs := map[string]string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == heading {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}

		cells := splitMarkdownTableRow(trimmed)
		if len(cells) == 0 || isMarkdownSeparatorRow(cells) {
			continue
		}
		if inputColumn == -1 || defaultColumn == -1 {
			for index, cell := range cells {
				switch strings.ToLower(strings.TrimSpace(cell)) {
				case "input":
					inputColumn = index
				case "default":
					defaultColumn = index
				}
			}
			continue
		}
		if len(cells) <= inputColumn || len(cells) <= defaultColumn {
			continue
		}

		inputName := trimMarkdownCode(cells[inputColumn])
		if inputName == "" {
			continue
		}
		inputs[inputName] = normalizeDefaultCell(cells[defaultColumn])
	}

	if len(inputs) == 0 {
		t.Fatalf("%s has no action input table under %q", path, heading)
	}
	return inputs
}

func splitMarkdownTableRow(row string) []string {
	trimmed := strings.Trim(row, "|")
	parts := strings.Split(trimmed, "|")
	for index, part := range parts {
		parts[index] = strings.TrimSpace(part)
	}
	return parts
}

func isMarkdownSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

func trimMarkdownCode(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`")
}

func normalizeDefaultCell(value string) string {
	defaultValue := strings.TrimSpace(value)
	if strings.HasPrefix(defaultValue, "`") {
		withoutOpeningTick := strings.TrimPrefix(defaultValue, "`")
		if codeSpan, _, ok := strings.Cut(withoutOpeningTick, "`"); ok {
			defaultValue = codeSpan
		}
	} else {
		defaultValue = trimMarkdownCode(defaultValue)
	}
	if before, _, ok := strings.Cut(defaultValue, " "); ok {
		defaultValue = before
	}
	return strings.Trim(defaultValue, `"`)
}

func compareInputDefaults(want map[string]string, got map[string]string) []string {
	var diff []string
	for name, defaultValue := range want {
		documented, ok := got[name]
		if !ok {
			diff = append(diff, "missing input: "+name)
			continue
		}
		if documented != defaultValue {
			diff = append(diff, name+": documented default "+documented+" != action default "+defaultValue)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			diff = append(diff, "extra input: "+name)
		}
	}
	return diff
}
