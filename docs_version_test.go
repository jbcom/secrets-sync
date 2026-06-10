package secretsync_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsDoNotAdvertiseOldCurrentVersion(t *testing.T) {
	forbiddenByPath := map[string][]string{
		"docs/ROADMAP.md": {
			"Current Status: v1.2.0",
			"Future Considerations (v2.0+)",
			"### v1.3.0",
			"### v1.4.0",
			"### v1.5.0",
			"coming in v1.3.0",
		},
		"docs/FAQ.md": {
			"SecretSync v1.2.0 is production-ready",
		},
	}

	for path, forbidden := range forbiddenByPath {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Fatalf("%s should not advertise old current-version phrase %q", path, phrase)
			}
		}
	}
}

func TestPublicDocsDoNotAdvertiseOldFeatureReleaseLabels(t *testing.T) {
	forbidden := []string{"v1.1.0", "v1.2.0"}
	paths := []string{"README.md"}

	if err := filepath.WalkDir("docs", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatalf("walk docs: %v", err)
	}

	var offenders []string
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				offenders = append(offenders, path+": "+phrase)
			}
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("public docs should not advertise old feature-release labels:\n%s", strings.Join(offenders, "\n"))
	}
}
