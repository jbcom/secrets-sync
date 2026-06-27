package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	clientkubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	Group          = "secrets-sync.jbcom.dev"
	Version        = "v1alpha1"
	Kind           = "CredentialSynchronization"
	Resource       = "credentialsynchronizations"
	DefaultImage   = "ghcr.io/jbcom/secrets-sync:v2.2.0"
	DefaultConfig  = "config.yaml"
	configMountDir = "/config"
)

var CredentialSynchronizationGVR = schema.GroupVersionResource{
	Group:    Group,
	Version:  Version,
	Resource: Resource,
}

// Config controls the Kubernetes controller loop.
type Config struct {
	Namespace        string
	ResyncPeriod     time.Duration
	DefaultImage     string
	DefaultConfigKey string
}

// Controller reconciles CredentialSynchronization resources into CronJobs.
type Controller struct {
	dynamic dynamic.Interface
	kube    clientkubernetes.Interface
	config  Config
}

// NewController creates a controller from Kubernetes clients.
func NewController(dynamicClient dynamic.Interface, kubeClient clientkubernetes.Interface, config Config) *Controller {
	if config.Namespace == "" {
		config.Namespace = metav1.NamespaceAll
	}
	if config.ResyncPeriod <= 0 {
		config.ResyncPeriod = time.Minute
	}
	if config.DefaultImage == "" {
		config.DefaultImage = DefaultImage
	}
	if config.DefaultConfigKey == "" {
		config.DefaultConfigKey = DefaultConfig
	}

	return &Controller{
		dynamic: dynamicClient,
		kube:    kubeClient,
		config:  config,
	}
}

// BuildRESTConfig builds cluster config from in-cluster auth, KUBECONFIG, or
// ~/.kube/config. This keeps local controller smoke tests possible.
func BuildRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return clientcmd.BuildConfigFromFlags("", env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory for kubeconfig: %w", err)
	}
	return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
}

// Run starts the reconciliation loop until the context is canceled.
func (c *Controller) Run(ctx context.Context) error {
	if err := c.ReconcileAll(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(c.config.ResyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.ReconcileAll(ctx); err != nil {
				return err
			}
		}
	}
}

// ReconcileAll reconciles all CredentialSynchronization resources in scope.
func (c *Controller) ReconcileAll(ctx context.Context) error {
	list, err := c.dynamic.Resource(CredentialSynchronizationGVR).Namespace(c.config.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list %s resources: %w", Resource, err)
	}

	for index := range list.Items {
		item := &list.Items[index]
		if err := c.Reconcile(ctx, item); err != nil {
			_ = c.updateStatus(ctx, item, "Failed", err.Error(), nil)
			fmt.Fprintf(os.Stderr, "failed to reconcile %s/%s: %v\n", item.GetNamespace(), item.GetName(), err)
		}
	}
	return nil
}

// Reconcile creates or updates the CronJob for a CredentialSynchronization.
func (c *Controller) Reconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	desired, err := c.BuildCronJob(obj)
	if err != nil {
		return err
	}

	ns := desired.Namespace
	existing, err := c.kube.BatchV1().CronJobs(ns).Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, createErr := c.kube.BatchV1().CronJobs(ns).Create(ctx, desired, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("create CronJob %s/%s: %w", ns, desired.Name, createErr)
		}
		return c.updateStatus(ctx, obj, phaseForCronJob(desired), "CronJob created", desired)
	}
	if err != nil {
		return fmt.Errorf("get CronJob %s/%s: %w", ns, desired.Name, err)
	}

	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations
	existing.OwnerReferences = desired.OwnerReferences
	existing.Spec = desired.Spec
	updated, err := c.kube.BatchV1().CronJobs(ns).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update CronJob %s/%s: %w", ns, desired.Name, err)
	}
	return c.updateStatus(ctx, obj, phaseForCronJob(updated), "CronJob reconciled", updated)
}

// BuildCronJob renders the managed CronJob for a CredentialSynchronization.
func (c *Controller) BuildCronJob(obj *unstructured.Unstructured) (*batchv1.CronJob, error) {
	spec, err := parseSpec(obj, c.config)
	if err != nil {
		return nil, err
	}

	name := cronJobName(obj.GetName())
	labels := map[string]string{
		"app.kubernetes.io/name":       "secrets-sync",
		"app.kubernetes.io/component":  "credential-synchronization",
		"app.kubernetes.io/managed-by": "secrets-sync-controller",
		"secrets-sync.jbcom.dev/name":  obj.GetName(),
	}

	args := []string{
		"pipeline",
		"--config",
		fmt.Sprintf("%s/%s", configMountDir, spec.ConfigKey),
		fmt.Sprintf("--continue-on-error=%t", spec.ContinueOnError),
		fmt.Sprintf("--diff=%t", spec.ComputeDiff),
		fmt.Sprintf("--dry-run=%t", spec.DryRun),
		"--output",
		spec.OutputFormat,
		"--parallelism",
		fmt.Sprintf("%d", spec.Parallelism),
	}
	switch spec.Operation {
	case "merge":
		args = append(args, "--merge-only")
	case "sync":
		args = append(args, "--sync-only")
	}
	if spec.Targets != "" {
		args = append(args, "--targets", spec.Targets)
	}
	if spec.Discover {
		args = append(args, "--discover")
	}

	controller := true
	blockOwnerDeletion := true
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: obj.GetNamespace(),
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         Group + "/" + Version,
					Kind:               Kind,
					Name:               obj.GetName(),
					UID:                obj.GetUID(),
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          spec.Schedule,
			Suspend:           &spec.Suspend,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: int32Ptr(1),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							ServiceAccountName: spec.ServiceAccountName,
							RestartPolicy:      corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:         "secrets-sync",
									Image:        spec.Image,
									Args:         args,
									Env:          spec.Env,
									EnvFrom:      spec.EnvFrom,
									Resources:    spec.Resources,
									VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: configMountDir, ReadOnly: true}},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{Name: spec.ConfigName},
											Items: []corev1.KeyToPath{
												{
													Key:  spec.ConfigKey,
													Path: spec.ConfigKey,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

type credentialSynchronizationSpec struct {
	Schedule           string
	Suspend            bool
	Image              string
	ConfigName         string
	ConfigKey          string
	Targets            string
	Operation          string
	DryRun             bool
	Discover           bool
	ComputeDiff        bool
	OutputFormat       string
	ContinueOnError    bool
	Parallelism        int
	ServiceAccountName string
	Env                []corev1.EnvVar
	EnvFrom            []corev1.EnvFromSource
	Resources          corev1.ResourceRequirements
}

func parseSpec(obj *unstructured.Unstructured, config Config) (credentialSynchronizationSpec, error) {
	schedule, _, _ := unstructured.NestedString(obj.Object, "spec", "schedule")
	if schedule == "" {
		return credentialSynchronizationSpec{}, fmt.Errorf("%s/%s spec.schedule is required", obj.GetNamespace(), obj.GetName())
	}

	configName, _, _ := unstructured.NestedString(obj.Object, "spec", "configRef", "name")
	if configName == "" {
		return credentialSynchronizationSpec{}, fmt.Errorf("%s/%s spec.configRef.name is required", obj.GetNamespace(), obj.GetName())
	}
	configKey, _, _ := unstructured.NestedString(obj.Object, "spec", "configRef", "key")
	if configKey == "" {
		configKey = config.DefaultConfigKey
	}

	image, _, _ := unstructured.NestedString(obj.Object, "spec", "image")
	if image == "" {
		image = config.DefaultImage
	}
	operation, _, _ := unstructured.NestedString(obj.Object, "spec", "operation")
	if operation == "" {
		operation = "pipeline"
	}
	if operation != "pipeline" && operation != "merge" && operation != "sync" {
		return credentialSynchronizationSpec{}, fmt.Errorf("%s/%s spec.operation must be pipeline, merge, or sync", obj.GetNamespace(), obj.GetName())
	}

	outputFormat, _, _ := unstructured.NestedString(obj.Object, "spec", "outputFormat")
	if outputFormat == "" {
		outputFormat = "json"
	}
	parallelism, _, _ := unstructured.NestedInt64(obj.Object, "spec", "parallelism")
	targets, _, _ := unstructured.NestedString(obj.Object, "spec", "targets")
	serviceAccountName, _, _ := unstructured.NestedString(obj.Object, "spec", "serviceAccountName")
	suspend, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	dryRun, _, _ := unstructured.NestedBool(obj.Object, "spec", "dryRun")
	discover, _, _ := unstructured.NestedBool(obj.Object, "spec", "discover")
	computeDiff, foundDiff, _ := unstructured.NestedBool(obj.Object, "spec", "computeDiff")
	if !foundDiff {
		computeDiff = true
	}
	continueOnError, foundContinue, _ := unstructured.NestedBool(obj.Object, "spec", "continueOnError")
	if !foundContinue {
		continueOnError = true
	}

	var extras struct {
		Env       []corev1.EnvVar             `json:"env,omitempty"`
		EnvFrom   []corev1.EnvFromSource      `json:"envFrom,omitempty"`
		Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	}
	rawSpec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawSpec, &extras); err != nil {
		return credentialSynchronizationSpec{}, fmt.Errorf("decode pod settings for %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}

	return credentialSynchronizationSpec{
		Schedule:           schedule,
		Suspend:            suspend,
		Image:              image,
		ConfigName:         configName,
		ConfigKey:          configKey,
		Targets:            targets,
		Operation:          operation,
		DryRun:             dryRun,
		Discover:           discover,
		ComputeDiff:        computeDiff,
		OutputFormat:       outputFormat,
		ContinueOnError:    continueOnError,
		Parallelism:        int(parallelism),
		ServiceAccountName: serviceAccountName,
		Env:                extras.Env,
		EnvFrom:            extras.EnvFrom,
		Resources:          extras.Resources,
	}, nil
}

func (c *Controller) updateStatus(ctx context.Context, obj *unstructured.Unstructured, phase, message string, cronJob *batchv1.CronJob) error {
	status := map[string]any{
		"observedGeneration": obj.GetGeneration(),
		"phase":              phase,
		"message":            message,
	}
	if cronJob != nil {
		if cronJob.Status.LastScheduleTime != nil {
			status["lastRunTime"] = cronJob.Status.LastScheduleTime.Format(time.RFC3339)
		}
		if cronJob.Status.LastSuccessfulTime != nil {
			status["lastSuccessTime"] = cronJob.Status.LastSuccessfulTime.Format(time.RFC3339)
		}
	}

	copy := obj.DeepCopy()
	copy.Object["status"] = status
	_, err := c.dynamic.Resource(CredentialSynchronizationGVR).Namespace(obj.GetNamespace()).UpdateStatus(ctx, copy, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("update %s/%s status: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

func phaseForCronJob(cronJob *batchv1.CronJob) string {
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		return "Suspended"
	}
	if len(cronJob.Status.Active) > 0 {
		return "Running"
	}
	if cronJob.Status.LastSuccessfulTime != nil {
		return "Succeeded"
	}
	return "Pending"
}

func cronJobName(name string) string {
	const maxCronJobNameLength = 52
	if len(name) <= maxCronJobNameLength {
		return name
	}

	hash := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(hash[:])[:8]
	prefixLength := maxCronJobNameLength - len(suffix) - 1
	return strings.TrimSuffix(name[:prefixLength], "-") + "-" + suffix
}

func int32Ptr(value int32) *int32 {
	return &value
}

// NamespacedName returns a stable string identifier for controller logs.
func NamespacedName(obj *unstructured.Unstructured) string {
	return types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}.String()
}
