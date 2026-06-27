// Package gcp implements a Google Cloud Secret Manager source + sync-target
// backend.
//
// Authentication uses Application Default Credentials, which covers service
// account keys, workload identity, and metadata-server credentials. Secrets are
// isolated per GCP project; the backend is scoped to a single project.
package gcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/jbcom/secrets-sync/pkg/circuitbreaker"
	"github.com/jbcom/secrets-sync/pkg/driver"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Compile-time assertions: GCP Secret Manager is a full source + sync target.
var (
	_ driver.SourceBackend = (*Client)(nil)
	_ driver.TargetBackend = (*Client)(nil)
)

// secretsAPI is the plain-typed subset of Secret Manager the backend uses,
// abstracted so tests can supply a fake without the proto-heavy real client.
type secretsAPI interface {
	// CreateSecret creates the secret container; AlreadyExists is not an error.
	CreateSecret(ctx context.Context, project, id string) error
	// AddVersion adds a new payload version to a secret.
	AddVersion(ctx context.Context, project, id string, data []byte) error
	// Access reads the latest version payload.
	Access(ctx context.Context, project, id string) ([]byte, error)
	// Delete removes a secret and all its versions.
	Delete(ctx context.Context, project, id string) error
	// List enumerates secret IDs in the project.
	List(ctx context.Context, project string) ([]string, error)
	// Close releases the client.
	Close() error
}

// Client is a GCP Secret Manager backend scoped to a single project.
type Client struct {
	// Project is the GCP project ID (the backend's GetPath()).
	Project string

	api     secretsAPI
	breaker *circuitbreaker.CircuitBreaker
}

// New constructs a GCP backend from a driver.BackendSpec. Path (or
// options.project) sets the GCP project ID.
func New(spec driver.BackendSpec) (*Client, error) {
	c := &Client{Project: spec.Path}
	if spec.Options != nil {
		if v, ok := spec.Options["project"].(string); ok && v != "" {
			c.Project = v
		}
	}
	if c.Project == "" {
		return nil, fmt.Errorf("gcp: project (or backend path) is required")
	}
	return c, nil
}

// Init builds the Secret Manager client and circuit breaker.
func (c *Client) Init(ctx context.Context) error {
	if c.breaker == nil {
		c.breaker = circuitbreaker.New(circuitbreaker.DefaultConfig("gcp:" + c.Project))
	}
	if c.api != nil {
		return nil
	}
	raw, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("gcp: build secret manager client: %w", err)
	}
	c.api = &smAdapter{client: raw}
	return nil
}

// Driver reports the GCP driver name.
func (c *Client) Driver() driver.DriverName { return driver.DriverNameGCP }

// GetPath returns the project ID.
func (c *Client) GetPath() string { return c.Project }

// Close releases the underlying client.
func (c *Client) Close() error {
	if c.api != nil {
		return c.api.Close()
	}
	return nil
}

// ListSecrets enumerates secret IDs in the project.
func (c *Client) ListSecrets(ctx context.Context, _ string) ([]string, error) {
	if c.api == nil {
		return nil, fmt.Errorf("gcp: backend not initialized")
	}
	return circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) ([]string, error) {
		return c.api.List(ctx, c.Project)
	})
}

// GetSecret reads the latest version payload of a secret.
func (c *Client) GetSecret(ctx context.Context, path string) ([]byte, error) {
	if c.api == nil {
		return nil, fmt.Errorf("gcp: backend not initialized")
	}
	return circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) ([]byte, error) {
		data, err := c.api.Access(ctx, c.Project, secretID(path))
		if err != nil {
			return nil, fmt.Errorf("gcp: access secret %q: %w", path, err)
		}
		return data, nil
	})
}

// WriteSecret ensures the secret container exists then adds a new version. The
// container create is idempotent (AlreadyExists is swallowed by the adapter).
func (c *Client) WriteSecret(ctx context.Context, _ metav1.ObjectMeta, path string, secret []byte) ([]byte, error) {
	if c.api == nil {
		return nil, fmt.Errorf("gcp: backend not initialized")
	}
	id := secretID(path)
	_, err := circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) (struct{}, error) {
		if err := c.api.CreateSecret(ctx, c.Project, id); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, c.api.AddVersion(ctx, c.Project, id, secret)
	})
	if err != nil {
		return nil, fmt.Errorf("gcp: write secret %q: %w", path, err)
	}
	return nil, nil
}

// DeleteSecret removes a secret and all versions. A missing secret is not an
// error.
func (c *Client) DeleteSecret(ctx context.Context, path string) error {
	if c.api == nil {
		return fmt.Errorf("gcp: backend not initialized")
	}
	_, err := circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, c.api.Delete(ctx, c.Project, secretID(path))
	})
	if err != nil {
		return fmt.Errorf("gcp: delete secret %q: %w", path, err)
	}
	return nil
}

// maxGCPIDLen is Secret Manager's secret-ID length limit.
const maxGCPIDLen = 255

// secretID maps a path to a collision-resistant, length-bounded Secret Manager
// secret ID: [A-Za-z0-9_-]. Sanitization is lossy (a "/" and a "." both map to
// "-"), so any altered, over-length, or empty-after-sanitize input gets a
// sha256-derived suffix of the original path to keep the mapping injective and
// prevent silent cross-path overwrite.
func secretID(path string) string {
	out := make([]rune, 0, len(path))
	for _, r := range path {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	// GCP secret IDs must not start or end with a hyphen or underscore.
	sanitized := strings.Trim(string(out), "-_")

	sum := sha256.Sum256([]byte(path))
	suffix := "-" + hex.EncodeToString(sum[:])[:10]

	if sanitized == path && sanitized != "" && len(sanitized) <= maxGCPIDLen {
		return sanitized
	}
	if sanitized == "" {
		return "secret" + suffix
	}
	maxBase := maxGCPIDLen - len(suffix)
	if len(sanitized) > maxBase {
		sanitized = strings.TrimRight(sanitized[:maxBase], "-")
	}
	return sanitized + suffix
}

// smAdapter wraps the real Secret Manager client behind secretsAPI, building the
// proto requests and resource names.
type smAdapter struct {
	client *secretmanager.Client
}

func parent(project string) string { return "projects/" + project }

func secretName(project, id string) string {
	return fmt.Sprintf("projects/%s/secrets/%s", project, id)
}

func (a *smAdapter) CreateSecret(ctx context.Context, project, id string) error {
	_, err := a.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   parent(project),
		SecretId: id,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if status.Code(err) == codes.AlreadyExists {
		return nil
	}
	return err
}

func (a *smAdapter) AddVersion(ctx context.Context, project, id string, data []byte) error {
	_, err := a.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  secretName(project, id),
		Payload: &secretmanagerpb.SecretPayload{Data: data},
	})
	return err
}

func (a *smAdapter) Access(ctx context.Context, project, id string) ([]byte, error) {
	resp, err := a.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName(project, id) + "/versions/latest",
	})
	if err != nil {
		return nil, err
	}
	// A nil payload means the version has no data (e.g. an orphaned container
	// whose AddVersion never landed). Surface it as an error rather than
	// returning "{}", which the sync phase would otherwise write to the target
	// as a real empty-object value — silent data corruption.
	if resp.GetPayload() == nil {
		return nil, fmt.Errorf("secret version has no payload")
	}
	return resp.GetPayload().GetData(), nil
}

func (a *smAdapter) Delete(ctx context.Context, project, id string) error {
	err := a.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: secretName(project, id),
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	return err
}

func (a *smAdapter) List(ctx context.Context, project string) ([]string, error) {
	it := a.client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{Parent: parent(project)})
	var ids []string
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		name := s.GetName()
		if i := strings.LastIndex(name, "/secrets/"); i >= 0 {
			ids = append(ids, name[i+len("/secrets/"):])
		} else {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

func (a *smAdapter) Close() error { return a.client.Close() }
