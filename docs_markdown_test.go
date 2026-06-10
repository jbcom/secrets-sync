package secretsync_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMarkdownFencedCodeBlocksAreBalanced(t *testing.T) {
	var offenders []string
	for _, root := range []string{"README.md", "docs"} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if strings.Count(string(content), "```")%2 != 0 {
				offenders = append(offenders, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("markdown files have unbalanced fenced code blocks: %s", strings.Join(offenders, ", "))
	}
}

func TestDeploymentGuideUsesCurrentPipelineSurface(t *testing.T) {
	content, err := os.ReadFile("docs/DEPLOYMENT.md")
	if err != nil {
		t.Fatalf("read docs/DEPLOYMENT.md: %v", err)
	}

	text := string(content)
	for _, required := range []string{
		"secretsync pipeline",
		"--dry-run",
		"--diff",
		"--output json",
		"kind: CronJob",
		"jbcom/secrets-sync@secrets-sync-vX.Y.Z",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("docs/DEPLOYMENT.md should document current deployment surface %q", required)
		}
	}
	for _, forbidden := range []string{
		"Vault Secrets Sync service",
		"Event Server",
		"Sync Operator",
		"-operator",
		"-events",
		"memory queue",
		"microservices mode",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("docs/DEPLOYMENT.md should not document stale deployment surface %q", forbidden)
		}
	}
}

func TestGettingStartedUsesCurrentPipelineConfigShape(t *testing.T) {
	paths := []string{"README.md", "docs/GETTING_STARTED.md"}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, required := range []string{
			"merge_store:",
			"account_id:",
			"secret_prefix:",
			"dynamic_targets:",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s should document current pipeline config %q", path, required)
			}
		}
		for _, forbidden := range []string{
			"aws_secretsmanager:",
			"inherits:",
			"discovery:\n  aws_organizations:",
			"versioning:\n  enabled: true\n  s3_bucket:",
			"secretsync versions",
			"secretsync sync --version",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s should not document stale config shape %q", path, forbidden)
			}
		}
	}
}

func TestPythonDocsUseExtendedDataCLIContract(t *testing.T) {
	attributeStyleResult := regexp.MustCompile(`\bresult\.`)
	paths := []string{"README.md", "docs/PYTHON_BINDINGS.md"}
	forbidden := []string{
		"native_available",
		"Native bindings",
		"native bindings",
		"CLI fallback",
		"bindings aren't installed",
		"is_valid, message",
		"print(result[\"diff_output\"])",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)

		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Fatalf("%s should not document stale Python integration surface %q", path, phrase)
			}
		}
		if attributeStyleResult.MatchString(text) {
			t.Fatalf("%s should use mapping-style result access, not result.attribute examples", path)
		}
	}

	content, err := os.ReadFile("docs/PYTHON_BINDINGS.md")
	if err != nil {
		t.Fatalf("read docs/PYTHON_BINDINGS.md: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"connector.cli_available",
		"validation = connector.validate_config",
		"result[\"success\"]",
		"result['secrets_added']",
		"secretsync pipeline --output json",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("docs/PYTHON_BINDINGS.md should document current Python contract %q", required)
		}
	}
}
