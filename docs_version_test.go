package secretsync_test

import (
	"os"
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
