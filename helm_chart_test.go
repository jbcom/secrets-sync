package secretsync_test

import (
	"os"
	"strings"
	"testing"
)

func TestHelmChartUsesSecretSyncAPI(t *testing.T) {
	paths := []string{
		"deploy/charts/secretsync/charts/secretsync-operator/crds/secretsync.extendeddata.dev_secretsyncs.yaml",
		"deploy/charts/secretsync/charts/secretsync-operator/templates/clusterrole.yaml",
		"docs/architecture/HLA-microservice.drawio",
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

func TestHelmCRDMatchesGoAPIGroup(t *testing.T) {
	crdPath := "deploy/charts/secretsync/charts/secretsync-operator/crds/secretsync.extendeddata.dev_secretsyncs.yaml"
	content, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("read %s: %v", crdPath, err)
	}

	text := string(content)
	for _, required := range []string{
		"name: secretsyncs.secretsync.extendeddata.dev",
		"group: secretsync.extendeddata.dev",
		"kind: SecretSync",
		"listKind: SecretSyncList",
		"plural: secretsyncs",
		"- ss",
		"singular: secretsync",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("%s missing required SecretSync CRD field %q", crdPath, required)
		}
	}
}
