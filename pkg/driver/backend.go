package driver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Backend is the common lifecycle surface shared by every secret store
// implementation. Concrete clients (AWS, Vault, Azure, GCP, Kubernetes, HTTP,
// ...) embed this behavior so the pipeline can treat them uniformly.
type Backend interface {
	// Init establishes the underlying client connection and authentication.
	Init(ctx context.Context) error
	// Driver reports which DriverName this backend implements.
	Driver() DriverName
	// GetPath returns the path/prefix the backend is currently scoped to.
	GetPath() string
	// Close releases any resources held by the backend.
	Close() error
}

// SourceBackend is a backend that secrets can be read FROM during the fetch
// and merge phases.
type SourceBackend interface {
	Backend
	// ListSecrets enumerates secret names/paths beneath the given path.
	//
	// Path semantics are backend-specific. Flat stores (AWS Secrets Manager)
	// treat an empty path as "list everything". Hierarchical stores (Vault KV2)
	// require a non-empty mount-scoped path; callers must pass the backend's
	// configured scope rather than "" for those. The driver-generic fetch path
	// always supplies a concrete path, so this divergence is not observable
	// through the pipeline.
	ListSecrets(ctx context.Context, path string) ([]string, error)
	// GetSecret reads the raw secret payload at the given path.
	GetSecret(ctx context.Context, path string) ([]byte, error)
}

// TargetBackend is a backend that secrets can be written TO during the sync
// phase. Every TargetBackend is also a SourceBackend so the pipeline can read
// back current state to compute diffs and detect orphans.
type TargetBackend interface {
	SourceBackend
	// WriteSecret creates or updates the secret at path with the given bytes.
	WriteSecret(ctx context.Context, meta metav1.ObjectMeta, path string, secret []byte) ([]byte, error)
	// DeleteSecret removes the secret at the given path.
	DeleteSecret(ctx context.Context, path string) error
}

// BundleStore is the intermediate storage the merge phase writes a whole merged
// bundle to and the sync phase reads it back from. It is keyed by target name
// and a deterministic bundle ID (derived from the source sequence). This is the
// abstraction the pipeline's merge/sync orchestration depends on; wrappers
// (client-side encryption, regional replication) compose around it.
type BundleStore interface {
	// WriteMergedBundle persists the full merged secret set for a target.
	WriteMergedBundle(ctx context.Context, targetName, bundleID string, secrets map[string]interface{}) error
	// ReadMergedBundle reads back the merged secret set for a target. The outer
	// map is keyed by relative secret path; the inner map is the secret's data.
	ReadMergedBundle(ctx context.Context, targetName, bundleID string) (map[string]map[string]interface{}, error)
}

// MergeStore is the per-secret view of intermediate storage used by read-back
// and diff paths. It is keyed by target name and secret name rather than a flat
// path, reflecting the bundle-per-target layout.
type MergeStore interface {
	// WriteSecret persists a merged secret for a target.
	WriteSecret(ctx context.Context, targetName, secretName string, data map[string]interface{}) error
	// ReadSecret reads a previously merged secret for a target.
	ReadSecret(ctx context.Context, targetName, secretName string) (map[string]interface{}, error)
	// ListSecrets enumerates merged secret names for a target.
	ListSecrets(ctx context.Context, targetName string) ([]string, error)
	// DeleteSecret removes a merged secret for a target.
	DeleteSecret(ctx context.Context, targetName, secretName string) error
}
