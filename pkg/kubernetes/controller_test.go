package kubernetes

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildCronJobRendersCredentialSynchronization(t *testing.T) {
	obj := credentialSynchronization("platform-secrets", map[string]any{
		"schedule":           "*/30 * * * *",
		"image":              "ghcr.io/jbcom/secrets-sync:test",
		"operation":          "pipeline",
		"targets":            "prod,staging",
		"dryRun":             true,
		"discover":           true,
		"computeDiff":        true,
		"outputFormat":       "json",
		"continueOnError":    false,
		"parallelism":        int64(7),
		"serviceAccountName": "secrets-sync",
		"configRef": map[string]any{
			"name": "platform-secrets-sync-config",
			"key":  "pipeline.yaml",
		},
		"envFrom": []any{
			map[string]any{
				"secretRef": map[string]any{"name": "provider-creds"},
			},
		},
	})

	controller := NewController(nil, nil, Config{})
	cronJob, err := controller.BuildCronJob(obj)
	if err != nil {
		t.Fatalf("BuildCronJob failed: %v", err)
	}

	if cronJob.Name != "platform-secrets" {
		t.Fatalf("CronJob name = %q", cronJob.Name)
	}
	if cronJob.Namespace != "secrets-sync" {
		t.Fatalf("CronJob namespace = %q", cronJob.Namespace)
	}
	if cronJob.Spec.Schedule != "*/30 * * * *" {
		t.Fatalf("Schedule = %q", cronJob.Spec.Schedule)
	}
	if cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName != "secrets-sync" {
		t.Fatalf("ServiceAccountName = %q", cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName)
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if container.Image != "ghcr.io/jbcom/secrets-sync:test" {
		t.Fatalf("Image = %q", container.Image)
	}
	args := strings.Join(container.Args, " ")
	for _, expected := range []string{
		"pipeline",
		"--config /config/pipeline.yaml",
		"--dry-run=true",
		"--diff=true",
		"--continue-on-error=false",
		"--parallelism 7",
		"--targets prod,staging",
		"--discover",
	} {
		if !strings.Contains(args, expected) {
			t.Fatalf("CronJob args missing %q: %s", expected, args)
		}
	}
	if len(container.EnvFrom) != 1 || container.EnvFrom[0].SecretRef == nil || container.EnvFrom[0].SecretRef.Name != "provider-creds" {
		t.Fatalf("EnvFrom was not rendered from CRD spec: %#v", container.EnvFrom)
	}
	if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].MountPath != configMountDir {
		t.Fatalf("VolumeMounts = %#v", container.VolumeMounts)
	}
	volume := cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0]
	if volume.ConfigMap == nil || volume.ConfigMap.Name != "platform-secrets-sync-config" {
		t.Fatalf("ConfigMap volume = %#v", volume.ConfigMap)
	}
	if len(cronJob.OwnerReferences) != 1 || cronJob.OwnerReferences[0].Kind != Kind {
		t.Fatalf("OwnerReferences = %#v", cronJob.OwnerReferences)
	}
}

func TestBuildCronJobDefaultsAndOperationModes(t *testing.T) {
	obj := credentialSynchronization("merge-only", map[string]any{
		"schedule":  "0 * * * *",
		"operation": "merge",
		"configRef": map[string]any{
			"name": "config",
		},
	})

	controller := NewController(nil, nil, Config{})
	cronJob, err := controller.BuildCronJob(obj)
	if err != nil {
		t.Fatalf("BuildCronJob failed: %v", err)
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if container.Image != DefaultImage {
		t.Fatalf("default image = %q", container.Image)
	}
	if !strings.Contains(strings.Join(container.Args, " "), "--merge-only") {
		t.Fatalf("merge operation should add --merge-only: %#v", container.Args)
	}
	volume := cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0]
	if volume.ConfigMap.Items[0].Key != DefaultConfig {
		t.Fatalf("default config key = %q", volume.ConfigMap.Items[0].Key)
	}
}

func TestBuildCronJobRejectsInvalidSpec(t *testing.T) {
	controller := NewController(nil, nil, Config{})
	for _, obj := range []*unstructured.Unstructured{
		credentialSynchronization("missing-schedule", map[string]any{
			"configRef": map[string]any{"name": "config"},
		}),
		credentialSynchronization("missing-config", map[string]any{
			"schedule": "0 * * * *",
		}),
		credentialSynchronization("bad-operation", map[string]any{
			"schedule":  "0 * * * *",
			"operation": "delete",
			"configRef": map[string]any{"name": "config"},
		}),
	} {
		if _, err := controller.BuildCronJob(obj); err == nil {
			t.Fatalf("BuildCronJob(%s) succeeded, want error", NamespacedName(obj))
		}
	}
}

func TestCronJobNameIsSafeForKubernetesCronJobs(t *testing.T) {
	name := cronJobName("this-is-a-very-long-credential-synchronization-name-that-must-be-shortened")
	if len(name) > 52 {
		t.Fatalf("cronJobName length = %d, want <= 52", len(name))
	}
	if !strings.Contains(name, "-") {
		t.Fatalf("cronJobName should include hash separator, got %q", name)
	}
}

func TestNamespacedName(t *testing.T) {
	obj := credentialSynchronization("example", map[string]any{
		"schedule":  "0 * * * *",
		"configRef": map[string]any{"name": "config"},
	})
	if got := NamespacedName(obj); got != "secrets-sync/example" {
		t.Fatalf("NamespacedName = %q", got)
	}
}

func credentialSynchronization(name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": Group + "/" + Version,
			"kind":       Kind,
			"metadata": map[string]any{
				"name":       name,
				"namespace":  "secrets-sync",
				"uid":        string(types.UID("test-" + name)),
				"generation": int64(3),
			},
			"spec": spec,
		},
	}
}

var _ = corev1.RestartPolicyNever
var _ = metav1.NamespaceAll
