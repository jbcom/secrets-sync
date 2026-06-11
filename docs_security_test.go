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
		"machine-readable `secretsync pipeline --output json` result envelopes redact",
		"GitHub Actions annotation output escapes workflow-command data",
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

func TestSecurityDocsUseProjectReportingContacts(t *testing.T) {
	required := []string{
		"https://github.com/jbcom/secrets-sync/security/advisories",
		"security@jbcom.dev",
	}

	for _, path := range []string{"SECURITY.md", "docs/SECURITY.md"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		if strings.Contains(text, "robert@lestak.sh") {
			t.Fatalf("%s should not use the old fork-era security contact", path)
		}
		for _, phrase := range required {
			if !strings.Contains(text, phrase) {
				t.Fatalf("%s must document reporting contact %q", path, phrase)
			}
		}
	}
}

func TestPublicUsageDocsDoNotUseForkEraOwners(t *testing.T) {
	for _, path := range []string{"docs/USAGE.md"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, forbidden := range []string{
			"robertlestak",
			"vault-secret-sync",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s should not use fork-era owner or package identifier %q", path, forbidden)
			}
		}
	}
}

func TestSecurityPolicyDocumentsCurrentMajorOnly(t *testing.T) {
	content, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatalf("read SECURITY.md: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "| 2.x") {
		t.Fatalf("SECURITY.md should document current 2.x support")
	}
	for _, oldVersion := range []string{"| 1.2.x", "| 1.1.x", "| 1.0.x"} {
		if strings.Contains(text, oldVersion) {
			t.Fatalf("SECURITY.md should not advertise old support line %q", oldVersion)
		}
	}
	for _, oldExample := range []string{"1.2.1", "1.2.2"} {
		if strings.Contains(text, oldExample) {
			t.Fatalf("SECURITY.md should not use unsupported 1.x patch example %q", oldExample)
		}
	}
	if !strings.Contains(text, "2.0.1") || !strings.Contains(text, "2.0.2") {
		t.Fatalf("SECURITY.md should use a supported 2.x patch-release example")
	}
}
