package secretsync_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/pipeline"
)

func TestExamplePipelineConfigsLoadAndValidate(t *testing.T) {
	paths, err := filepath.Glob("examples/*.yaml")
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected pipeline config examples")
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			cfg, err := pipeline.LoadConfigWithoutAutoDetect(path)
			if err != nil {
				t.Fatalf("load %s: %v", path, err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("validate %s: %v", path, err)
			}
		})
	}
}

func TestPublicDocsAndExamplesDoNotAdvertiseRemovedAPISurfaces(t *testing.T) {
	forbidden := []string{
		"apiVersion: secretsync.extendeddata.dev",
		"kind: SecretSync",
		"aws_secretsmanager:",
		"inherits:",
		"Kubernetes operator with CRD support",
		"secretsync-events",
		"secretsync-operator",
		"Event Server",
		"Sync Operator",
		"memory queue",
		"microservices mode",
		"Vault | GCP Secret Manager | ✅ Supported",
		"Vault | GitHub Secrets | ✅ Supported",
	}

	for _, root := range []string{"README.md", "docs", "examples"} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".md", ".yaml", ".yml":
			default:
				return nil
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			text := string(content)
			for _, phrase := range forbidden {
				if strings.Contains(text, phrase) {
					t.Fatalf("%s should not advertise removed API surface %q", path, phrase)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
