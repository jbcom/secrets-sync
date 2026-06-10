package secretsync_test

import (
	"os"
	"strings"
	"testing"
)

func TestDockerfileDoesNotShipVSSAlias(t *testing.T) {
	content, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	text := string(content)
	for _, forbidden := range []string{"/usr/local/bin/vss", "backwards compatibility", "backward compatibility"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Dockerfile should not ship vss compatibility alias %q", forbidden)
		}
	}
}

func TestOrganizationsTestingDocsDoNotAdvertiseVSSAlias(t *testing.T) {
	path := "docs/testing/organizations-discovery-integration-tests.md"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	if strings.Contains(text, "./vss") || strings.Contains(text, " vss ") {
		t.Fatalf("%s should advertise secretsync, not vss", path)
	}
}
