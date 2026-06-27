package pipeline

import (
	"github.com/jbcom/secrets-sync/pkg/client/aws"
	"github.com/jbcom/secrets-sync/pkg/client/azure"
	"github.com/jbcom/secrets-sync/pkg/client/gcp"
	"github.com/jbcom/secrets-sync/pkg/client/httpstore"
	"github.com/jbcom/secrets-sync/pkg/client/k8s"
	"github.com/jbcom/secrets-sync/pkg/client/vault"
	"github.com/jbcom/secrets-sync/pkg/driver"
)

// bundleStore returns the configured bundle-level merge store as a
// driver.BundleStore, or nil when the Vault (path-based) merge path is in use.
// Cross-cutting wrappers (client-side encryption, regional replication) compose
// around this interface rather than the concrete S3 store.
func (p *Pipeline) bundleStore() driver.BundleStore {
	if p.replicated != nil {
		return p.replicated
	}
	if p.s3Store == nil {
		return nil
	}
	return p.s3Store
}

// newBackendRegistry returns a registry pre-populated with the built-in AWS and
// Vault backends. Each Pipeline owns its own registry instance so registration
// is deterministic and free of global state — additional providers are
// registered here as they are implemented.
func newBackendRegistry() *driver.Registry {
	r := driver.NewRegistry()

	r.RegisterTarget(driver.DriverNameVault, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		c := &vault.VaultClient{Path: spec.Path}
		applyVaultOptions(c, spec.Options)
		return c, nil
	})

	r.RegisterTarget(driver.DriverNameAws, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		c := &aws.AwsClient{Name: optString(spec.Options, "name"), RoleArn: optString(spec.Options, "role_arn")}
		applyAWSOptions(c, spec.Options)
		if c.Region == "" {
			c.Region = spec.Path
		}
		return c, nil
	})

	r.RegisterTarget(driver.DriverNameKubernetes, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		return k8s.New(spec)
	})

	r.RegisterTarget(driver.DriverNameHTTP, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		return httpstore.New(spec)
	})

	r.RegisterTarget(driver.DriverNameAzure, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		return azure.New(spec)
	})

	r.RegisterTarget(driver.DriverNameGCP, func(spec driver.BackendSpec) (driver.TargetBackend, error) {
		return gcp.New(spec)
	})

	return r
}

// applyVaultOptions threads typed options onto a VaultClient.
func applyVaultOptions(c *vault.VaultClient, opts map[string]any) {
	c.Address = optStringOr(opts, "address", c.Address)
	c.Namespace = optStringOr(opts, "namespace", c.Namespace)
	c.Token = optStringOr(opts, "token", c.Token)
	c.AuthMethod = optStringOr(opts, "auth_method", c.AuthMethod)
	c.Role = optStringOr(opts, "role", c.Role)
	c.MaxTraversalDepth = optIntOr(opts, "max_traversal_depth", c.MaxTraversalDepth)
	c.MaxSecretsPerMount = optIntOr(opts, "max_secrets_per_mount", c.MaxSecretsPerMount)
	c.QueueCompactionThreshold = optIntOr(opts, "queue_compaction_threshold", c.QueueCompactionThreshold)
}

// applyAWSOptions threads typed options onto an AwsClient.
func applyAWSOptions(c *aws.AwsClient, opts map[string]any) {
	c.Region = optStringOr(opts, "region", c.Region)
	c.RuntimeAccessKeyID = optStringOr(opts, "access_key_id", c.RuntimeAccessKeyID)
	c.RuntimeSecretAccessKey = optStringOr(opts, "secret_access_key", c.RuntimeSecretAccessKey)
	c.RuntimeSessionToken = optStringOr(opts, "session_token", c.RuntimeSessionToken)
	c.Endpoint = optStringOr(opts, "endpoint", c.Endpoint)
}

func optString(opts map[string]any, key string) string {
	return optStringOr(opts, key, "")
}

func optStringOr(opts map[string]any, key, fallback string) string {
	if opts == nil {
		return fallback
	}
	if v, ok := opts[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func optIntOr(opts map[string]any, key string, fallback int) int {
	if opts == nil {
		return fallback
	}
	if v, ok := opts[key].(int); ok {
		return v
	}
	return fallback
}
