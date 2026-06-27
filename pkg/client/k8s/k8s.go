// Package k8s implements a Kubernetes Secrets target backend for secrets-sync.
//
// It writes merged secrets directly into a Kubernetes cluster as native Secret
// objects, namespace-scoped, supporting Opaque, kubernetes.io/tls, and
// kubernetes.io/dockerconfigjson secret types. It reuses the same REST-config
// resolution (in-cluster, KUBECONFIG, ~/.kube/config) as the controller.
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jbcom/secrets-sync/pkg/driver"
	"github.com/jbcom/secrets-sync/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientkubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Compile-time assertions: the Kubernetes backend is a full sync target.
var (
	_ driver.SourceBackend = (*Client)(nil)
	_ driver.TargetBackend = (*Client)(nil)
)

// SecretsAPI is the subset of the typed CoreV1 Secrets interface the backend
// uses. It is an interface so tests can supply a fake clientset.
type SecretsAPI interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error)
	Create(ctx context.Context, secret *corev1.Secret, opts metav1.CreateOptions) (*corev1.Secret, error)
	Update(ctx context.Context, secret *corev1.Secret, opts metav1.UpdateOptions) (*corev1.Secret, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	List(ctx context.Context, opts metav1.ListOptions) (*corev1.SecretList, error)
}

// Client is a Kubernetes Secrets backend scoped to a single namespace.
type Client struct {
	// Namespace is the target namespace (the backend's GetPath()).
	Namespace string
	// Kubeconfig optionally points at an explicit kubeconfig file.
	Kubeconfig string
	// SecretType controls the created Secret's type. Defaults to Opaque.
	// Accepts "Opaque", "kubernetes.io/tls", "kubernetes.io/dockerconfigjson".
	SecretType corev1.SecretType
	// Labels applied to every managed Secret.
	Labels map[string]string

	api SecretsAPI
}

// New constructs a Kubernetes backend from a driver.BackendSpec. Path is the
// namespace; options.kubeconfig, options.secret_type, and options.labels are
// honored.
func New(spec driver.BackendSpec) (*Client, error) {
	c := &Client{
		Namespace:  spec.Path,
		SecretType: corev1.SecretTypeOpaque,
	}
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if spec.Options != nil {
		if v, ok := spec.Options["kubeconfig"].(string); ok {
			c.Kubeconfig = v
		}
		if v, ok := spec.Options["secret_type"].(string); ok && v != "" {
			c.SecretType = corev1.SecretType(v)
		}
		if v, ok := spec.Options["namespace"].(string); ok && v != "" {
			c.Namespace = v
		}
		if v, ok := spec.Options["labels"].(map[string]string); ok {
			c.Labels = v
		}
	}
	return c, nil
}

// Init resolves cluster config and builds the typed Secrets client. If a
// SecretsAPI was injected (tests), Init is a no-op.
func (c *Client) Init(ctx context.Context) error {
	if c.api != nil {
		return nil
	}
	cfg, err := kubernetes.BuildRESTConfig(c.Kubeconfig)
	if err != nil {
		return fmt.Errorf("build kube config: %w", err)
	}
	return c.initFromConfig(cfg)
}

func (c *Client) initFromConfig(cfg *rest.Config) error {
	cs, err := clientkubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("build kubernetes clientset: %w", err)
	}
	c.api = cs.CoreV1().Secrets(c.Namespace)
	return nil
}

// Driver reports the Kubernetes driver name.
func (c *Client) Driver() driver.DriverName { return driver.DriverNameKubernetes }

// GetPath returns the namespace the backend is scoped to.
func (c *Client) GetPath() string { return c.Namespace }

// Close is a no-op; the typed client holds no long-lived resources.
func (c *Client) Close() error { return nil }

// secretName sanitizes a path into a DNS-1123 compliant Secret name by
// lowercasing and replacing path separators and underscores with hyphens.
func secretName(path string) string {
	name := strings.ToLower(path)
	name = strings.NewReplacer("/", "-", "_", "-", ".", "-", " ", "-").Replace(name)
	return strings.Trim(name, "-")
}

// ListSecrets lists managed Secret names in the namespace.
func (c *Client) ListSecrets(ctx context.Context, _ string) ([]string, error) {
	if c.api == nil {
		return nil, fmt.Errorf("k8s backend not initialized")
	}
	list, err := c.api.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	out := make([]string, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, list.Items[i].Name)
	}
	return out, nil
}

// GetSecret reads a Secret and returns its data marshaled as a JSON object of
// string→string (base64-decoded values), matching the rest of the pipeline's
// JSON secret representation.
func (c *Client) GetSecret(ctx context.Context, path string) ([]byte, error) {
	if c.api == nil {
		return nil, fmt.Errorf("k8s backend not initialized")
	}
	sec, err := c.api.Get(ctx, secretName(path), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get secret %q: %w", path, err)
	}
	flat := make(map[string]string, len(sec.Data))
	for k, v := range sec.Data {
		flat[k] = string(v)
	}
	return json.Marshal(flat)
}

// WriteSecret creates or updates a namespace-scoped Secret. The secret bytes are
// a JSON object whose top-level keys become Secret data keys.
func (c *Client) WriteSecret(ctx context.Context, meta metav1.ObjectMeta, path string, secret []byte) ([]byte, error) {
	if c.api == nil {
		return nil, fmt.Errorf("k8s backend not initialized")
	}
	data, err := decodeSecretData(secret)
	if err != nil {
		return nil, err
	}

	name := secretName(path)
	labels := map[string]string{}
	for k, v := range c.Labels {
		labels[k] = v
	}
	for k, v := range meta.Labels {
		labels[k] = v
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Type: c.SecretType,
		Data: data,
	}

	existing, err := c.api.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := c.api.Create(ctx, desired, metav1.CreateOptions{}); err != nil {
			return nil, fmt.Errorf("create secret %q: %w", name, err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get secret %q: %w", name, err)
	}

	existing.Data = data
	existing.Type = c.SecretType
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range labels {
		existing.Labels[k] = v
	}
	if _, err := c.api.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return nil, fmt.Errorf("update secret %q: %w", name, err)
	}
	return nil, nil
}

// DeleteSecret removes a namespace-scoped Secret. A missing secret is not an
// error (idempotent delete).
func (c *Client) DeleteSecret(ctx context.Context, path string) error {
	if c.api == nil {
		return fmt.Errorf("k8s backend not initialized")
	}
	err := c.api.Delete(ctx, secretName(path), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete secret %q: %w", path, err)
	}
	return nil
}

// decodeSecretData turns a JSON secret payload into Secret Data. String values
// are stored verbatim; non-string values are JSON-encoded.
func decodeSecretData(secret []byte) (map[string][]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(secret, &raw); err != nil {
		return nil, fmt.Errorf("decode secret payload as JSON object: %w", err)
	}
	data := make(map[string][]byte, len(raw))
	for k, v := range raw {
		switch typed := v.(type) {
		case string:
			data[k] = []byte(typed)
		default:
			b, err := json.Marshal(typed)
			if err != nil {
				return nil, fmt.Errorf("encode value for key %q: %w", k, err)
			}
			data[k] = b
		}
	}
	return data, nil
}
