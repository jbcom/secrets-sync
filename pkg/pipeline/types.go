// Package pipeline provides unified configuration and orchestration for secrets syncing pipelines.
package pipeline

// Config represents the unified pipeline configuration
type Config struct {
	Log            LogConfig                `mapstructure:"log" yaml:"log"`
	Vault          VaultConfig              `mapstructure:"vault" yaml:"vault"`
	AWS            AWSConfig                `mapstructure:"aws" yaml:"aws"`
	Sources        map[string]Source        `mapstructure:"sources" yaml:"sources"`
	MergeStore     MergeStoreConfig         `mapstructure:"merge_store" yaml:"merge_store"`
	Targets        map[string]Target        `mapstructure:"targets" yaml:"targets"`
	DynamicTargets map[string]DynamicTarget `mapstructure:"dynamic_targets" yaml:"dynamic_targets"`
	Pipeline       PipelineSettings         `mapstructure:"pipeline" yaml:"pipeline"`
}

// LogConfig controls logging behavior
type LogConfig struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
}

// VaultConfig configures Vault connection and authentication
type VaultConfig struct {
	Address   string          `mapstructure:"address" yaml:"address"`
	Namespace string          `mapstructure:"namespace" yaml:"namespace"`
	Auth      VaultAuthConfig `mapstructure:"auth" yaml:"auth"`

	// Traversal configuration for recursive secret listing
	// These settings control memory usage and performance during large Vault traversals
	MaxTraversalDepth        int `mapstructure:"max_traversal_depth" yaml:"max_traversal_depth,omitempty"`
	MaxSecretsPerMount       int `mapstructure:"max_secrets_per_mount" yaml:"max_secrets_per_mount,omitempty"`
	QueueCompactionThreshold int `mapstructure:"queue_compaction_threshold" yaml:"queue_compaction_threshold,omitempty"`
}

// VaultAuthConfig supports multiple authentication methods
type VaultAuthConfig struct {
	AppRole    *AppRoleAuth    `mapstructure:"approle" yaml:"approle"`
	Token      *TokenAuth      `mapstructure:"token" yaml:"token"`
	Kubernetes *KubernetesAuth `mapstructure:"kubernetes" yaml:"kubernetes"`
}

// AppRoleAuth configures AppRole authentication
type AppRoleAuth struct {
	Mount    string `mapstructure:"mount" yaml:"mount"`
	RoleID   string `mapstructure:"role_id" yaml:"role_id"`
	SecretID string `mapstructure:"secret_id" yaml:"secret_id"`
}

// TokenAuth configures token authentication
type TokenAuth struct {
	Token string `mapstructure:"token" yaml:"token"`
}

// KubernetesAuth configures Kubernetes authentication
type KubernetesAuth struct {
	Role      string `mapstructure:"role" yaml:"role"`
	MountPath string `mapstructure:"mount_path" yaml:"mount_path"`
}

// AWSConfig configures AWS with Control Tower / Organizations awareness
type AWSConfig struct {
	Region           string                 `mapstructure:"region" yaml:"region"`
	ExecutionContext ExecutionContextConfig `mapstructure:"execution_context" yaml:"execution_context"`
	ControlTower     ControlTowerConfig     `mapstructure:"control_tower" yaml:"control_tower"`
	Organizations    OrganizationsConfig    `mapstructure:"organizations" yaml:"organizations"`
	IdentityCenter   IdentityCenterConfig   `mapstructure:"identity_center" yaml:"identity_center"`
}

// ExecutionContextType defines where the pipeline runs from
type ExecutionContextType string

const (
	// ExecutionContextManagement runs from the AWS Organizations management account
	ExecutionContextManagement ExecutionContextType = "management_account"
	// ExecutionContextDelegated runs from a delegated administrator account
	ExecutionContextDelegated ExecutionContextType = "delegated_admin"
	// ExecutionContextHub runs from a custom secrets hub account
	ExecutionContextHub ExecutionContextType = "hub_account"
)

// ExecutionContextConfig defines where the pipeline is running from
type ExecutionContextConfig struct {
	Type              ExecutionContextType `mapstructure:"type" yaml:"type"`
	AccountID         string               `mapstructure:"account_id" yaml:"account_id"`
	Delegation        *DelegationConfig    `mapstructure:"delegation" yaml:"delegation"`
	CustomRolePattern string               `mapstructure:"custom_role_pattern" yaml:"custom_role_pattern"`
}

// DelegationConfig defines delegated administrator settings
type DelegationConfig struct {
	Services []string `mapstructure:"services" yaml:"services"`
}

// ControlTowerConfig configures AWS Control Tower integration
type ControlTowerConfig struct {
	Enabled        bool                 `mapstructure:"enabled" yaml:"enabled"`
	ExecutionRole  ExecutionRoleConfig  `mapstructure:"execution_role" yaml:"execution_role"`
	AccountFactory AccountFactoryConfig `mapstructure:"account_factory" yaml:"account_factory"`
}

// ExecutionRoleConfig defines the cross-account execution role
type ExecutionRoleConfig struct {
	Name string `mapstructure:"name" yaml:"name"`
	Path string `mapstructure:"path" yaml:"path"`
}

// AccountFactoryConfig configures Account Factory integration
type AccountFactoryConfig struct {
	Enabled           bool `mapstructure:"enabled" yaml:"enabled"`
	OnAccountCreation bool `mapstructure:"on_account_creation" yaml:"on_account_creation"`
	AFTIntegration    bool `mapstructure:"aft_integration" yaml:"aft_integration"`
}

// OrganizationsConfig configures AWS Organizations integration
type OrganizationsConfig struct {
	AutoDiscover bool                `mapstructure:"auto_discover" yaml:"auto_discover"`
	RootID       string              `mapstructure:"root_id" yaml:"root_id"`
	OUs          map[string]OUConfig `mapstructure:"ous" yaml:"ous"`
}

// OUConfig represents an Organizational Unit
type OUConfig struct {
	ID       string              `mapstructure:"id" yaml:"id"`
	Accounts []string            `mapstructure:"accounts" yaml:"accounts"`
	Children map[string]OUConfig `mapstructure:"children" yaml:"children"`
}

// IdentityCenterConfig configures AWS Identity Center (SSO) integration
type IdentityCenterConfig struct {
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled"`
	AutoDiscover    bool   `mapstructure:"auto_discover" yaml:"auto_discover"`
	InstanceARN     string `mapstructure:"instance_arn" yaml:"instance_arn"`
	IdentityStoreID string `mapstructure:"identity_store_id" yaml:"identity_store_id"`
}

// Source defines where secrets can be imported from
type Source struct {
	Vault *VaultSource `mapstructure:"vault" yaml:"vault"`
	AWS   *AWSSource   `mapstructure:"aws" yaml:"aws"`
}

// VaultSource imports secrets from a Vault KV2 mount
type VaultSource struct {
	Address   string   `mapstructure:"address" yaml:"address"`
	Namespace string   `mapstructure:"namespace" yaml:"namespace"`
	Mount     string   `mapstructure:"mount" yaml:"mount"`
	Paths     []string `mapstructure:"paths" yaml:"paths"`

	// Traversal configuration for recursive secret listing
	// These settings control memory usage and performance during large Vault traversals
	MaxTraversalDepth        int `mapstructure:"max_traversal_depth" yaml:"max_traversal_depth,omitempty"`
	MaxSecretsPerMount       int `mapstructure:"max_secrets_per_mount" yaml:"max_secrets_per_mount,omitempty"`
	QueueCompactionThreshold int `mapstructure:"queue_compaction_threshold" yaml:"queue_compaction_threshold,omitempty"`
}

// AWSSource imports secrets from AWS Secrets Manager
type AWSSource struct {
	AccountID string            `mapstructure:"account_id" yaml:"account_id"`
	Region    string            `mapstructure:"region" yaml:"region"`
	Prefix    string            `mapstructure:"prefix" yaml:"prefix"`
	Tags      map[string]string `mapstructure:"tags" yaml:"tags"`
}

// MergeStoreConfig defines intermediate storage for merged secrets
type MergeStoreConfig struct {
	Vault *MergeStoreVault `mapstructure:"vault" yaml:"vault"`
	S3    *MergeStoreS3    `mapstructure:"s3" yaml:"s3"`
}

// MergeStoreVault uses Vault as the merge store
type MergeStoreVault struct {
	Mount string `mapstructure:"mount" yaml:"mount"`
}

// MergeStoreS3 uses S3 as the merge store
type MergeStoreS3 struct {
	Bucket   string `mapstructure:"bucket" yaml:"bucket"`
	Prefix   string `mapstructure:"prefix" yaml:"prefix"`
	KMSKeyID string `mapstructure:"kms_key_id" yaml:"kms_key_id"`

	// Version management (v1.2.0 - Requirement 24)
	Versioning *VersioningConfig `mapstructure:"versioning" yaml:"versioning"`
}

// VersioningConfig configures secret versioning (v1.2.0 - Requirement 24)
type VersioningConfig struct {
	Enabled        bool `mapstructure:"enabled" yaml:"enabled"`
	RetainVersions int  `mapstructure:"retain_versions" yaml:"retain_versions"`
}

// Target defines a sync destination.
// Supports two YAML formats:
//  1. Explicit: target: {account_id: "...", imports: [...]}
//  2. Shorthand inheritance: target: [parent1, parent2]  (list IS the imports)
type Target struct {
	AccountID    string   `mapstructure:"account_id" yaml:"account_id"`
	Imports      []string `mapstructure:"imports" yaml:"imports"`
	Region       string   `mapstructure:"region" yaml:"region"`
	SecretPrefix string   `mapstructure:"secret_prefix" yaml:"secret_prefix"`
	RoleARN      string   `mapstructure:"role_arn" yaml:"role_arn"`
}

// UnmarshalYAML implements custom YAML unmarshaling to support shorthand format.
func (t *Target) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First try to unmarshal as a list (shorthand format)
	var shorthand []string
	if err := unmarshal(&shorthand); err == nil {
		t.Imports = shorthand
		return nil
	}

	// Otherwise unmarshal as the full struct
	type targetAlias Target // avoid infinite recursion
	var full targetAlias
	if err := unmarshal(&full); err != nil {
		return err
	}
	*t = Target(full)
	return nil
}

// DynamicTarget defines targets discovered at runtime
type DynamicTarget struct {
	Discovery DiscoveryConfig `mapstructure:"discovery" yaml:"discovery"`
	Imports   []string        `mapstructure:"imports" yaml:"imports"`
	Exclude   []string        `mapstructure:"exclude" yaml:"exclude"`

	// AccountNamePatterns maps discovered accounts to specific targets using regex
	AccountNamePatterns []AccountNamePattern `mapstructure:"account_name_patterns" yaml:"account_name_patterns"`

	Region       string `mapstructure:"region" yaml:"region"`
	SecretPrefix string `mapstructure:"secret_prefix" yaml:"secret_prefix"`
	RoleARN      string `mapstructure:"role_arn" yaml:"role_arn"`
}

// DiscoveryConfig defines how to discover dynamic targets
type DiscoveryConfig struct {
	IdentityCenter *IdentityCenterDiscovery `mapstructure:"identity_center" yaml:"identity_center"`
	Organizations  *OrganizationsDiscovery  `mapstructure:"organizations" yaml:"organizations"`
	AccountsList   *AccountsListDiscovery   `mapstructure:"accounts_list" yaml:"accounts_list"`
}

// IdentityCenterDiscovery discovers accounts from Identity Center
type IdentityCenterDiscovery struct {
	Group         string `mapstructure:"group" yaml:"group"`
	PermissionSet string `mapstructure:"permission_set" yaml:"permission_set"`
}

// OrganizationsDiscovery discovers accounts from AWS Organizations
type OrganizationsDiscovery struct {
	OU           string              `mapstructure:"ou" yaml:"ou"`
	Tags         map[string][]string `mapstructure:"tags" yaml:"tags"`
	Recursive    bool                `mapstructure:"recursive" yaml:"recursive"`
	NameMatching *NameMatchingConfig `mapstructure:"name_matching" yaml:"name_matching"`

	// Enhanced filtering (v1.2.0)
	OUs              []string    `mapstructure:"ous" yaml:"ous"` // Multiple OUs support
	TagFilters       []TagFilter `mapstructure:"tag_filters" yaml:"tag_filters"`
	TagCombination   string      `mapstructure:"tag_combination" yaml:"tag_combination"`       // "AND" or "OR", default "AND"
	ExcludeStatuses  []string    `mapstructure:"exclude_statuses" yaml:"exclude_statuses"`     // e.g., ["SUSPENDED", "CLOSED"]
	CacheOUStructure bool        `mapstructure:"cache_ou_structure" yaml:"cache_ou_structure"` // Cache OU hierarchy
}

// TagFilter represents a single tag filtering condition with wildcard support
type TagFilter struct {
	Key      string   `mapstructure:"key" yaml:"key"`
	Values   []string `mapstructure:"values" yaml:"values"`
	Operator string   `mapstructure:"operator" yaml:"operator"` // "equals", "contains", "wildcard", default "equals"
}

// NameMatchingConfig configures fuzzy account name matching
type NameMatchingConfig struct {
	// Strategy: "exact", "fuzzy", or "loose" (default: "exact")
	// - exact: names must match exactly (case-insensitive by default)
	// - fuzzy: partial substring matching with normalization
	// - loose: most permissive, applies all normalizations
	Strategy string `mapstructure:"strategy" yaml:"strategy"`

	// NormalizeKeys: apply JSON key normalization (default: false)
	// Converts underscores to hyphens, removes special chars
	NormalizeKeys bool `mapstructure:"normalize_keys" yaml:"normalize_keys"`

	// CaseInsensitive: case-insensitive matching (default: true)
	CaseInsensitive bool `mapstructure:"case_insensitive" yaml:"case_insensitive"`

	// StripPrefixes: prefixes to remove before matching
	// Common values: ["aws-", "fsc-", "org-"]
	StripPrefixes []string `mapstructure:"strip_prefixes" yaml:"strip_prefixes"`

	// StripSuffixes: suffixes to remove before matching
	// Common values: ["-account", "-acct"]
	StripSuffixes []string `mapstructure:"strip_suffixes" yaml:"strip_suffixes"`
}

// AccountNamePattern maps discovered accounts to targets
type AccountNamePattern struct {
	// Pattern is a regex pattern to match against normalized account names
	Pattern string `mapstructure:"pattern" yaml:"pattern"`
	// Target is the target name to use when pattern matches
	Target string `mapstructure:"target" yaml:"target"`
}

// AccountsListDiscovery discovers accounts from an external source
type AccountsListDiscovery struct {
	Source string `mapstructure:"source" yaml:"source"`
}

// PipelineSettings configures pipeline execution
type PipelineSettings struct {
	Merge           MergeSettings `mapstructure:"merge" yaml:"merge"`
	Sync            SyncSettings  `mapstructure:"sync" yaml:"sync"`
	DryRun          bool          `mapstructure:"dry_run" yaml:"dry_run"`
	ContinueOnError bool          `mapstructure:"continue_on_error" yaml:"continue_on_error"`
}

// MergeSettings configures the merge phase
type MergeSettings struct {
	Parallel int `mapstructure:"parallel" yaml:"parallel"`
}

// SyncSettings configures the sync phase
type SyncSettings struct {
	Parallel      int  `mapstructure:"parallel" yaml:"parallel"`
	DeleteOrphans bool `mapstructure:"delete_orphans" yaml:"delete_orphans"`
}
