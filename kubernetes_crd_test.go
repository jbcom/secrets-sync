package secrets_sync_test

import (
	"os"
	"strings"
	"testing"
)

func TestKubernetesCRDDefinesSecretsSyncAPIContract(t *testing.T) {
	content, err := os.ReadFile("deploy/crds/secrets-sync.jbcom.dev_credentialsynchronizations.yaml")
	if err != nil {
		t.Fatalf("read CRD: %v", err)
	}

	text := string(content)
	required := []string{
		"kind: CustomResourceDefinition",
		"name: credentialsynchronizations.secrets-sync.jbcom.dev",
		"group: secrets-sync.jbcom.dev",
		"kind: CredentialSynchronization",
		"name: v1alpha1",
		"configRef:",
		"ghcr.io/jbcom/secrets-sync:latest",
		"status: {}",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("CRD missing %q", needle)
		}
	}
}

func TestKubernetesCRDExampleUsesCurrentImageAndKind(t *testing.T) {
	content, err := os.ReadFile("deploy/crds/examples/kubernetes-credential-synchronization.yaml")
	if err != nil {
		t.Fatalf("read CRD example: %v", err)
	}

	text := string(content)
	for _, needle := range []string{
		"apiVersion: secrets-sync.jbcom.dev/v1alpha1",
		"kind: CredentialSynchronization",
		"image: ghcr.io/jbcom/secrets-sync:latest",
		"configRef:",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CRD example missing %q", needle)
		}
	}
}

func TestKubernetesControllerManifestsWireCredentialSynchronization(t *testing.T) {
	files := map[string]string{
		"namespace":     readKubernetesFile(t, "deploy/controller/namespace.yaml"),
		"rbac":          readKubernetesFile(t, "deploy/controller/rbac.yaml"),
		"deployment":    readKubernetesFile(t, "deploy/controller/deployment.yaml"),
		"kustomize":     readKubernetesFile(t, "deploy/controller/kustomization.yaml"),
		"controller go": readKubernetesFile(t, "cmd/secrets-sync-controller/main.go"),
	}

	required := map[string][]string{
		"namespace": {
			"kind: Namespace",
			"name: secrets-sync",
		},
		"rbac": {
			"kind: ServiceAccount",
			"kind: ClusterRole",
			"kind: ClusterRoleBinding",
			"credentialsynchronizations",
			"credentialsynchronizations/status",
			"cronjobs",
		},
		"deployment": {
			"kind: Deployment",
			"image: ghcr.io/jbcom/secrets-sync:latest",
			"secrets-sync-controller",
			"--resync=1m",
		},
		"kustomize": {
			"namespace.yaml",
			"rbac.yaml",
			"deployment.yaml",
		},
		"controller go": {
			"NewController",
			"dynamic.NewForConfig",
			"clientkubernetes.NewForConfig",
		},
	}

	for name, needles := range required {
		for _, needle := range needles {
			if !strings.Contains(files[name], needle) {
				t.Fatalf("%s missing %q", name, needle)
			}
		}
	}
}

func readKubernetesFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
