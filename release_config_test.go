package secrets_sync_test

import (
	"os"
	"strings"
	"testing"
)

func TestGoReleaserUsesProductNameInReleaseNotes(t *testing.T) {
	content, err := os.ReadFile(".goreleaser.yml")
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "project_name: secrets-sync") {
		t.Fatalf(".goreleaser.yml should keep lowercase secrets-sync artifact naming")
	}
	if !strings.Contains(text, "## SecretSync {{ .Tag }}") {
		t.Fatalf(".goreleaser.yml should use SecretSync product casing in release notes")
	}
	if strings.Contains(text, "## secrets-sync {{ .Tag }}") {
		t.Fatalf(".goreleaser.yml should not use lowercase product name in release notes")
	}
}
