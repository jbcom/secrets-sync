package secretsync_test

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
	if !strings.Contains(text, "project_name: secretsync") {
		t.Fatalf(".goreleaser.yml should keep lowercase secretsync artifact naming")
	}
	if !strings.Contains(text, "## SecretSync {{ .Tag }}") {
		t.Fatalf(".goreleaser.yml should use SecretSync product casing in release notes")
	}
	if strings.Contains(text, "## secretsync {{ .Tag }}") {
		t.Fatalf(".goreleaser.yml should not use lowercase product name in release notes")
	}
}
