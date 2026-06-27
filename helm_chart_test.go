package secrets_sync_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelmChartUsesSecretSyncAPI(t *testing.T) {
	paths := []string{
		"deploy/charts/secrets-sync/Chart.yaml",
		"deploy/charts/secrets-sync/values.yaml",
		"docs/USAGE.md",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, forbidden := range []string{
			"vaultsecretsync.lestak.sh",
			"VaultSecretSync",
			"vaultsecretsync",
			" vss",
			"- vss",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s should not preserve old VaultSecretSync API surface %q", path, forbidden)
			}
		}
	}
}

func TestHelmChartUsesPipelineRunner(t *testing.T) {
	files := map[string]string{
		"chart":                  readTestFile(t, "deploy/charts/secrets-sync/Chart.yaml"),
		"values":                 readTestFile(t, "deploy/charts/secrets-sync/values.yaml"),
		"configmap":              readTestFile(t, "deploy/charts/secrets-sync/templates/configmap.yaml"),
		"cronjob":                readTestFile(t, "deploy/charts/secrets-sync/templates/cronjob.yaml"),
		"controller-deployment":  readTestFile(t, "deploy/charts/secrets-sync/templates/controller-deployment.yaml"),
		"controller-rbac":        readTestFile(t, "deploy/charts/secrets-sync/templates/controller-rbac.yaml"),
		"controller-serviceacct": readTestFile(t, "deploy/charts/secrets-sync/templates/controller-serviceaccount.yaml"),
	}

	for _, forbidden := range []string{
		"dependencies:",
		"secrets-sync-events",
		"secrets-sync-operator",
		"Legacy config format",
		"Kubernetes operator",
		"-operator",
		"-events",
	} {
		for name, text := range files {
			if strings.Contains(text, forbidden) {
				t.Fatalf("helm %s should not contain removed surface %q", name, forbidden)
			}
		}
	}

	required := map[string][]string{
		"values": {
			"pipeline:",
			"enabled: false",
			"schedule: \"\"",
			"config: {}",
			"continueOnError: true",
			"controller:",
			"resync: 1m",
		},
		"configmap": {
			".Values.pipeline.config",
			".Values.pipeline.existingConfigMap",
		},
		"cronjob": {
			"kind: CronJob",
			"- pipeline",
			"- --config",
			"/config/config.yaml",
			"--dry-run={{ .Values.pipeline.dryRun }}",
			"--continue-on-error={{ .Values.pipeline.continueOnError }}",
		},
		"controller-deployment": {
			"kind: Deployment",
			"secrets-sync-controller",
			"--resync={{ .Values.controller.resync }}",
			".Values.controller.watchNamespace",
		},
		"controller-rbac": {
			"kind: ClusterRole",
			"credentialsynchronizations",
			"credentialsynchronizations/status",
			"cronjobs",
		},
		"controller-serviceacct": {
			"kind: ServiceAccount",
			"app.kubernetes.io/component: controller",
		},
	}

	for name, needles := range required {
		for _, needle := range needles {
			if !strings.Contains(files[name], needle) {
				t.Fatalf("helm %s missing %q", name, needle)
			}
		}
	}
}

func TestHelmChartDoesNotShipDeadSubcharts(t *testing.T) {
	for _, path := range []string{
		"deploy/charts/secrets-sync/charts/secrets-sync-events",
		"deploy/charts/secrets-sync/charts/secrets-sync-operator",
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist after removing the unsupported operator/events runtimes", path)
		}
	}
}

func TestHelmTemplatesDoNotUseRemovedCLIFlags(t *testing.T) {
	err := filepath.WalkDir("deploy/charts/secrets-sync", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".yaml", ".yml", ".tpl":
		default:
			return nil
		}
		text := readTestFile(t, path)
		for _, forbidden := range []string{"-operator", "-events"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s should not use removed CLI flag %q", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk helm chart: %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
