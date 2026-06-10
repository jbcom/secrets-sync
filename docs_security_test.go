package secretsync_test

import (
	"os"
	"strings"
	"testing"
)

func TestSecurityDocsDocumentLoggingContract(t *testing.T) {
	required := []string{
		"raw secret values",
		"raw Vault secret",
		"raw AWS secret",
		"raw client structures",
	}

	for _, path := range []string{"docs/SECURITY.md", "docs/OBSERVABILITY.md"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))
		for _, phrase := range required {
			if !strings.Contains(text, strings.ToLower(phrase)) {
				t.Fatalf("%s must document logging contract phrase %q", path, phrase)
			}
		}
	}
}
