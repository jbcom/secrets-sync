package secrets_sync_test

import (
	"os"
	"regexp"
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

	text := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))
	vssToken := regexp.MustCompile(`\bvss\b`)
	if strings.Contains(text, "./vss") || vssToken.MatchString(text) {
		t.Fatalf("%s should advertise secrets-sync, not vss", path)
	}
}

func TestForkBreakScriptIsNotShipped(t *testing.T) {
	path := "scripts/break-fork.sh"
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("%s should not ship in the independent repository", path)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
