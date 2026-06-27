// Package azure implements an Azure Key Vault source + sync-target backend.
//
// Authentication uses azidentity.DefaultAzureCredential, which transparently
// supports service principals (env vars), managed identity, and workload
// identity federation. Cross-tenant sync is achieved by pointing different
// targets at vaults in different tenants with appropriate Azure RBAC role
// assignments; no special handling is required in this backend.
package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/jbcom/secrets-sync/pkg/circuitbreaker"
	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Compile-time assertions: Azure Key Vault is a full source + sync target.
var (
	_ driver.SourceBackend = (*Client)(nil)
	_ driver.TargetBackend = (*Client)(nil)
)

// secretsAPI is the subset of the azsecrets client the backend uses, abstracted
// so tests can supply a fake without a live vault.
type secretsAPI interface {
	SetSecret(ctx context.Context, name string, params azsecrets.SetSecretParameters, opts *azsecrets.SetSecretOptions) (azsecrets.SetSecretResponse, error)
	GetSecret(ctx context.Context, name, version string, opts *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
	DeleteSecret(ctx context.Context, name string, opts *azsecrets.DeleteSecretOptions) (azsecrets.DeleteSecretResponse, error)
	ListSecretNames(ctx context.Context) ([]string, error)
}

// Client is an Azure Key Vault backend scoped to a single vault URL.
type Client struct {
	// VaultURL is the Key Vault endpoint, e.g. "https://myvault.vault.azure.net/".
	VaultURL string

	api     secretsAPI
	breaker *circuitbreaker.CircuitBreaker
}

// New constructs an Azure backend from a driver.BackendSpec. Path (or
// options.vault_url) sets the vault endpoint.
func New(spec driver.BackendSpec) (*Client, error) {
	c := &Client{VaultURL: spec.Path}
	if spec.Options != nil {
		if v, ok := spec.Options["vault_url"].(string); ok && v != "" {
			c.VaultURL = v
		}
	}
	if c.VaultURL == "" {
		return nil, fmt.Errorf("azure: vault_url (or backend path) is required")
	}
	return c, nil
}

// Init builds the credential, azsecrets client, and circuit breaker. If an API
// was injected (tests), Init only ensures the breaker exists.
func (c *Client) Init(_ context.Context) error {
	if c.breaker == nil {
		c.breaker = circuitbreaker.New(circuitbreaker.DefaultConfig("azure:" + c.VaultURL))
	}
	if c.api != nil {
		return nil
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("azure: build credential: %w", err)
	}
	raw, err := azsecrets.NewClient(c.VaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("azure: build key vault client: %w", err)
	}
	c.api = &azsecretsAdapter{client: raw}
	return nil
}

// Driver reports the Azure driver name.
func (c *Client) Driver() driver.DriverName { return driver.DriverNameAzure }

// GetPath returns the vault URL.
func (c *Client) GetPath() string { return c.VaultURL }

// Close is a no-op; the azsecrets client holds no long-lived resources.
func (c *Client) Close() error { return nil }

// ListSecrets enumerates secret names in the vault.
func (c *Client) ListSecrets(ctx context.Context, _ string) ([]string, error) {
	if c.api == nil {
		return nil, fmt.Errorf("azure: backend not initialized")
	}
	return circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) ([]string, error) {
		return c.api.ListSecretNames(ctx)
	})
}

// GetSecret reads the latest version of a secret's value.
func (c *Client) GetSecret(ctx context.Context, path string) ([]byte, error) {
	if c.api == nil {
		return nil, fmt.Errorf("azure: backend not initialized")
	}
	return circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) ([]byte, error) {
		resp, err := c.api.GetSecret(ctx, secretName(path), "", nil)
		if err != nil {
			return nil, fmt.Errorf("azure: get secret %q: %w", path, err)
		}
		if resp.Value == nil {
			return []byte("{}"), nil
		}
		return []byte(*resp.Value), nil
	})
}

// WriteSecret creates or updates a secret. Key Vault SetSecret is upsert, so no
// existence check is needed.
func (c *Client) WriteSecret(ctx context.Context, _ metav1.ObjectMeta, path string, secret []byte) ([]byte, error) {
	if c.api == nil {
		return nil, fmt.Errorf("azure: backend not initialized")
	}
	value := string(secret)
	_, err := circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) (azsecrets.SetSecretResponse, error) {
		return c.api.SetSecret(ctx, secretName(path), azsecrets.SetSecretParameters{Value: &value}, nil)
	})
	if err != nil {
		return nil, fmt.Errorf("azure: set secret %q: %w", path, err)
	}
	return nil, nil
}

// DeleteSecret removes a secret from the vault.
func (c *Client) DeleteSecret(ctx context.Context, path string) error {
	if c.api == nil {
		return fmt.Errorf("azure: backend not initialized")
	}
	_, err := circuitbreaker.ExecuteTyped(c.breaker, ctx, func(ctx context.Context) (azsecrets.DeleteSecretResponse, error) {
		return c.api.DeleteSecret(ctx, secretName(path), nil)
	})
	if err != nil {
		return fmt.Errorf("azure: delete secret %q: %w", path, err)
	}
	return nil
}

// azsecretsAdapter wraps the real azsecrets client behind secretsAPI, collapsing
// the pager into a name slice.
type azsecretsAdapter struct {
	client *azsecrets.Client
}

func (a *azsecretsAdapter) SetSecret(ctx context.Context, name string, params azsecrets.SetSecretParameters, opts *azsecrets.SetSecretOptions) (azsecrets.SetSecretResponse, error) {
	return a.client.SetSecret(ctx, name, params, opts)
}

func (a *azsecretsAdapter) GetSecret(ctx context.Context, name, version string, opts *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	return a.client.GetSecret(ctx, name, version, opts)
}

func (a *azsecretsAdapter) DeleteSecret(ctx context.Context, name string, opts *azsecrets.DeleteSecretOptions) (azsecrets.DeleteSecretResponse, error) {
	return a.client.DeleteSecret(ctx, name, opts)
}

func (a *azsecretsAdapter) ListSecretNames(ctx context.Context) ([]string, error) {
	pager := a.client.NewListSecretPropertiesPager(nil)
	var names []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, prop := range page.Value {
			if prop != nil && prop.ID != nil {
				names = append(names, prop.ID.Name())
			}
		}
	}
	return names, nil
}

// secretName maps a path to an Azure Key Vault secret name. Key Vault names are
// restricted to alphanumerics and hyphens; "/", "_", and "." become "-".
func secretName(path string) string {
	out := make([]rune, 0, len(path))
	for _, r := range path {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	name := string(out)
	for len(name) > 0 && name[0] == '-' {
		name = name[1:]
	}
	for len(name) > 0 && name[len(name)-1] == '-' {
		name = name[:len(name)-1]
	}
	if name == "" {
		return "secret"
	}
	return name
}
