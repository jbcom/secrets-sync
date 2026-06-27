package secrets_sync_test

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
	paths := []string{"docs/ARCHITECTURE.md", "docs/DEPLOYMENT.md"}
	for _, required := range []string{
		"secrets-sync pipeline",
		"--dry-run",
		"--diff",
		"--output json",
		"kind: CronJob",
	} {
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if !strings.Contains(string(content), required) {
				t.Fatalf("%s should document current deployment surface %q", path, required)
			}
		}
	}
	deploymentGuide, err := os.ReadFile("docs/DEPLOYMENT.md")
	if err != nil {
		t.Fatalf("read docs/DEPLOYMENT.md: %v", err)
	}
	if !strings.Contains(string(deploymentGuide), "jbcom/secrets-sync@secrets-sync-vX.Y.Z") {
		t.Fatal("docs/DEPLOYMENT.md should document the GitHub Action release tag")
	}

	for _, forbidden := range []string{
		"Vault Secrets Sync service",
		"Event Server",
		"Sync Operator",
		"-operator",
		"-events",
		"memory queue",
		"microservices mode",
		"REST webhook endpoint",
		"SecretSync resources",
	} {
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if strings.Contains(string(content), forbidden) {
				t.Fatalf("%s should not document stale deployment surface %q", path, forbidden)
			}
		}
	}
}

func TestStaleArchitectureDiagramsAreNotPublished(t *testing.T) {
	if _, err := os.Stat("docs/architecture"); !os.IsNotExist(err) {
		t.Fatal("docs/architecture should not publish stale operator architecture diagrams")
	}
}

func TestArchitectureAuditCurrentShapeReferencesExistingPaths(t *testing.T) {
	content, err := os.ReadFile("docs/ARCHITECTURE_AUDIT.md")
	if err != nil {
		t.Fatalf("read docs/ARCHITECTURE_AUDIT.md: %v", err)
	}

	text := string(content)
	start := strings.Index(text, "## Current Shape")
	end := strings.Index(text, "## Future Release Work")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("docs/ARCHITECTURE_AUDIT.md should have a current shape section before future release work")
	}

	currentShape := text[start:end]
	for _, forbidden := range []string{
		"api/v1alpha1",
		"Kubernetes API types",
	} {
		if strings.Contains(currentShape, forbidden) {
			t.Fatalf("architecture audit should not document removed current path %q", forbidden)
		}
	}
	if !strings.Contains(currentShape, "deploy/charts/secrets-sync") {
		t.Fatal("architecture audit should document the Helm runner chart path")
	}

	for _, match := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(currentShape, -1) {
		path := match[1]
		if !isArchitectureAuditRepoPath(path) {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("architecture audit references missing current path %s: %v", path, err)
		}
	}
}

func TestArchitectureAuditDoesNotKeepMigrationGapFilename(t *testing.T) {
	if _, err := os.Stat("docs/ARCHITECTURE_GAP_ANALYSIS.md"); !os.IsNotExist(err) {
		t.Fatal("docs/ARCHITECTURE_GAP_ANALYSIS.md should not be published in the standalone repository")
	}
}

func TestArchitectureAuditIsDiscoverableFromPublicDocs(t *testing.T) {
	for _, path := range []string{"README.md", "docs/ARCHITECTURE.md"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(content), "ARCHITECTURE_AUDIT.md") {
			t.Fatalf("%s should link to docs/ARCHITECTURE_AUDIT.md", path)
		}
	}
}

func TestContributingGuideUsesCurrentRepositoryShape(t *testing.T) {
	content, err := os.ReadFile("CONTRIBUTING.md")
	if err != nil {
		t.Fatalf("read CONTRIBUTING.md: %v", err)
	}

	text := string(content)
	for _, required := range []string{
		"pkg/client/",
		"pkg/driver",
		"pkg/pipeline",
		"driver.DriverName",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("CONTRIBUTING.md should document current repository shape %q", required)
		}
	}

	for _, forbidden := range []string{
		"stores/newstore",
		"github.com/jbcom/secrets-sync/pkg/store",
		"├── stores/",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("CONTRIBUTING.md should not document removed store surface %q", forbidden)
		}
	}
}

func TestPublicGitHubDirectoryLinksUseTreeURLs(t *testing.T) {
	brokenDirectoryLink := regexp.MustCompile(`https://github\.com/jbcom/secrets-sync/(docs|examples)(?:[)\s]|$)`)
	var offenders []string

	for _, root := range []string{"README.md", "docs", "CONTRIBUTING.md", "SECURITY.md"} {
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
			if brokenDirectoryLink.Match(content) {
				offenders = append(offenders, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("GitHub directory links should use /tree/main/... URLs:\n%s", strings.Join(offenders, "\n"))
	}
}

func isArchitectureAuditRepoPath(path string) bool {
	if strings.HasPrefix(path, "jbcom/") || strings.Contains(path, ":") {
		return false
	}
	for _, prefix := range []string{"cmd/", "deploy/", "docs/", "pkg/", "python/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return path == "pkg" || path == "action.yml" || strings.HasSuffix(path, ".go")
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
			"secrets-sync versions",
			"secrets-sync sync --version",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s should not document stale config shape %q", path, forbidden)
			}
		}
	}
}

func TestPythonDocsUseVendorFabricContract(t *testing.T) {
	paths := []string{"README.md", "docs/PYTHON_BINDINGS.md"}
	forbidden := []string{
		"bindings aren't installed",
		"from secrets_sync import get_tools",
		"from secrets_sync import SecretsSyncBridge",
		"is_valid, message",
		"print(result[\"diff_output\"])",
		"backend=\"auto\"",
		"bridge.cli_available",
		"bridge.native_available",
		"secrets_sync_native",
		"agentic-crew[secrets-sync]",
		"pip install secrets-sync-bridge",
	}
	required := []string{
		"vendor_fabric.secrets_sync",
		"vendor-fabric[secrets-sync]",
		"secrets-sync pipeline --config pipeline.yaml --output json",
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
		for _, phrase := range required {
			if !strings.Contains(text, phrase) {
				t.Fatalf("%s should document current Python integration boundary %q", path, phrase)
			}
		}
	}
}

func TestOwnershipMapDocumentsSplitBoundaries(t *testing.T) {
	content, err := os.ReadFile("docs/OWNERSHIP.md")
	if err != nil {
		t.Fatalf("read docs/OWNERSHIP.md: %v", err)
	}
	text := string(content)

	for _, required := range []string{
		"cmd/secrets-sync",
		"jbcom/extended-data",
		"jbcom/vendor-fabric",
		"vendor-fabric[secrets-sync]",
		"vendor-fabric[ai,secrets-sync]",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("docs/OWNERSHIP.md should document boundary %q", required)
		}
	}
	for _, forbidden := range []string{
		"packages/secrets-sync-bridge",
		"secrets_sync_native",
		"jbcom/agent-orchestration",
		"agentic-crew[secrets-sync]",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("docs/OWNERSHIP.md should not document retired boundary %q", forbidden)
		}
	}
}
