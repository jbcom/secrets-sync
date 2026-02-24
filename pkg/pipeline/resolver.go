// Package pipeline provides automatic resolution of sources and destinations.
// Supports fuzzy matching of AWS account names with JSON key normalization.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ResourceType identifies the type of a source/destination
type ResourceType string

const (
	// ResourceTypeAWSAccount indicates an AWS account (discovered via Organizations)
	ResourceTypeAWSAccount ResourceType = "aws_account"
	// ResourceTypeVaultMount indicates a Vault KV2 mount
	ResourceTypeVaultMount ResourceType = "vault_mount"
	// ResourceTypeUnknown indicates the resource could not be resolved
	ResourceTypeUnknown ResourceType = "unknown"
)

// ResolvedResource represents a resolved source or destination
type ResolvedResource struct {
	// OriginalName is the name as provided by the user
	OriginalName string
	// ResolvedName is the normalized/matched name
	ResolvedName string
	// Type indicates whether this is an AWS account or Vault mount
	Type ResourceType
	// AccountID is set if Type is ResourceTypeAWSAccount
	AccountID string
	// VaultMount is set if Type is ResourceTypeVaultMount
	VaultMount string
	// MatchConfidence indicates how the match was made
	MatchConfidence MatchConfidence
}

// MatchConfidence indicates how confident the match is
type MatchConfidence string

const (
	// MatchExact means the name matched exactly
	MatchExact MatchConfidence = "exact"
	// MatchNormalized means the name matched after JSON key normalization
	MatchNormalized MatchConfidence = "normalized"
	// MatchFuzzy means the name matched via fuzzy/loose matching
	MatchFuzzy MatchConfidence = "fuzzy"
	// MatchAccountID means matched by AWS account ID
	MatchAccountID MatchConfidence = "account_id"
	// MatchNone means no match was found
	MatchNone MatchConfidence = "none"
)

// ResourceResolver resolves source/destination names to concrete resources
type ResourceResolver struct {
	ctx context.Context
	cfg *Config

	// Cached AWS accounts from Organizations discovery
	awsAccounts     []AccountInfo
	awsAccountsById map[string]AccountInfo
	// Normalized account name → AccountInfo for fuzzy matching
	awsAccountsByNormalizedName map[string]AccountInfo

	// Known Vault mounts
	vaultMounts map[string]bool

	// Fuzzy matcher
	nameMatcher *NameMatcher
}

// NewResourceResolver creates a resolver that auto-detects resources
func NewResourceResolver(ctx context.Context, cfg *Config) *ResourceResolver {
	return &ResourceResolver{
		ctx:                         ctx,
		cfg:                         cfg,
		awsAccountsById:             make(map[string]AccountInfo),
		awsAccountsByNormalizedName: make(map[string]AccountInfo),
		vaultMounts:                 make(map[string]bool),
		nameMatcher: NewNameMatcher(&NameMatchingConfig{
			Strategy:        "fuzzy",
			CaseInsensitive: true,
			NormalizeKeys:   true,
			// Common prefixes/suffixes in AWS account naming
			StripPrefixes: []string{"aws-", "account-", "acct-"},
			StripSuffixes: []string{"-account", "-acct", "-aws"},
		}),
	}
}

// Initialize loads AWS accounts and Vault mounts for resolution
func (r *ResourceResolver) Initialize(awsCtx *AWSExecutionContext) error {
	l := log.WithField("action", "ResourceResolver.Initialize")

	// Load AWS accounts if AWS is available
	if awsCtx != nil && awsCtx.CanAccessOrganizations() {
		accounts, err := awsCtx.ListOrganizationAccounts(r.ctx)
		if err != nil {
			l.WithError(err).Warn("Could not list AWS accounts - AWS resolution disabled")
		} else {
			r.awsAccounts = accounts
			for _, acct := range accounts {
				r.awsAccountsById[acct.ID] = acct
				// Index by multiple normalized forms for fuzzy matching
				normalized := r.nameMatcher.NormalizeAccountName(acct.Name)
				r.awsAccountsByNormalizedName[normalized] = acct
				// Also index without normalization for exact matching
				r.awsAccountsByNormalizedName[strings.ToLower(acct.Name)] = acct
			}
			l.WithField("count", len(accounts)).Info("Loaded AWS accounts for resolution")
		}
	}

	// Load known Vault mounts from config sources
	for name, source := range r.cfg.Sources {
		if source.Vault != nil {
			mount := source.Vault.Mount
			if mount == "" {
				mount = name
			}
			r.vaultMounts[mount] = true
			r.vaultMounts[strings.ToLower(mount)] = true
		}
	}

	l.WithFields(log.Fields{
		"awsAccounts": len(r.awsAccounts),
		"vaultMounts": len(r.vaultMounts),
	}).Debug("Resource resolver initialized")

	return nil
}

// Resolve determines what type of resource a name refers to
// Priority: 1) Exact AWS account ID, 2) Exact AWS name, 3) Fuzzy AWS match, 4) Vault mount
func (r *ResourceResolver) Resolve(name string) ResolvedResource {
	l := log.WithFields(log.Fields{
		"action": "ResourceResolver.Resolve",
		"name":   name,
	})

	result := ResolvedResource{
		OriginalName:    name,
		Type:            ResourceTypeUnknown,
		MatchConfidence: MatchNone,
	}

	// 1. Check if it's a 12-digit AWS account ID
	if isValidAWSAccountID(name) {
		if acct, ok := r.awsAccountsById[name]; ok {
			result.Type = ResourceTypeAWSAccount
			result.AccountID = name
			result.ResolvedName = acct.Name
			result.MatchConfidence = MatchAccountID
			l.WithField("accountId", name).Debug("Resolved as AWS account by ID")
			return result
		}
	}

	// 2. Check for exact AWS account name match (case-insensitive)
	nameLower := strings.ToLower(name)
	if acct, ok := r.awsAccountsByNormalizedName[nameLower]; ok {
		result.Type = ResourceTypeAWSAccount
		result.AccountID = acct.ID
		result.ResolvedName = acct.Name
		result.MatchConfidence = MatchExact
		l.WithFields(log.Fields{
			"accountId":   acct.ID,
			"accountName": acct.Name,
		}).Debug("Resolved as AWS account by exact name")
		return result
	}

	// 3. Check for normalized AWS account name match
	normalized := r.nameMatcher.NormalizeAccountName(name)
	if acct, ok := r.awsAccountsByNormalizedName[normalized]; ok {
		result.Type = ResourceTypeAWSAccount
		result.AccountID = acct.ID
		result.ResolvedName = acct.Name
		result.MatchConfidence = MatchNormalized
		l.WithFields(log.Fields{
			"accountId":      acct.ID,
			"accountName":    acct.Name,
			"normalizedName": normalized,
		}).Debug("Resolved as AWS account by normalized name")
		return result
	}

	// 4. Fuzzy match against all AWS accounts
	if acct := r.fuzzyMatchAWSAccount(name); acct != nil {
		result.Type = ResourceTypeAWSAccount
		result.AccountID = acct.ID
		result.ResolvedName = acct.Name
		result.MatchConfidence = MatchFuzzy
		l.WithFields(log.Fields{
			"accountId":   acct.ID,
			"accountName": acct.Name,
		}).Debug("Resolved as AWS account by fuzzy match")
		return result
	}

	// 5. If not found in AWS, assume it's a Vault mount
	// This is the fallback - if AWS accounts are available and name wasn't found,
	// it must be a Vault KV2 mount path
	result.Type = ResourceTypeVaultMount
	result.VaultMount = name
	result.ResolvedName = name
	result.MatchConfidence = MatchExact // It's whatever the user specified
	l.Debug("Resolved as Vault mount (not found in AWS)")
	return result
}

// fuzzyMatchAWSAccount attempts fuzzy matching against all AWS accounts
func (r *ResourceResolver) fuzzyMatchAWSAccount(name string) *AccountInfo {
	normalized := r.nameMatcher.NormalizeAccountName(name)

	// Try substring matching with delimiter awareness
	for _, acct := range r.awsAccounts {
		acctNormalized := r.nameMatcher.NormalizeAccountName(acct.Name)

		// Check various fuzzy matching strategies
		if r.fuzzyMatch(normalized, acctNormalized) {
			return &acct
		}
	}

	return nil
}

// fuzzyMatch performs delimiter-aware fuzzy matching
func (r *ResourceResolver) fuzzyMatch(input, target string) bool {
	// Exact match after normalization
	if input == target {
		return true
	}

	// Substring match (input contained in target or vice versa)
	if strings.Contains(target, input) || strings.Contains(input, target) {
		return true
	}

	// Token-based matching: split by delimiters and check overlap
	inputTokens := tokenize(input)
	targetTokens := tokenize(target)

	// If input has multiple tokens, check if all are present in target
	if len(inputTokens) > 1 {
		matchCount := 0
		for _, it := range inputTokens {
			for _, tt := range targetTokens {
				if it == tt || strings.Contains(tt, it) || strings.Contains(it, tt) {
					matchCount++
					break
				}
			}
		}
		// Require majority of tokens to match
		if float64(matchCount)/float64(len(inputTokens)) >= 0.6 {
			return true
		}
	}

	return false
}

// tokenize splits a string by common delimiters
func tokenize(s string) []string {
	// Replace all common delimiters with space, then split
	delimiters := regexp.MustCompile(`[-_.\s]+`)
	normalized := delimiters.ReplaceAllString(s, " ")
	parts := strings.Fields(normalized)

	// Filter out very short tokens
	var result []string
	for _, p := range parts {
		if len(p) >= 2 {
			result = append(result, strings.ToLower(p))
		}
	}
	return result
}

// ResolveAll resolves a list of names
func (r *ResourceResolver) ResolveAll(names []string) []ResolvedResource {
	results := make([]ResolvedResource, len(names))
	for i, name := range names {
		results[i] = r.Resolve(name)
	}
	return results
}

// DetectAuthProviders checks what authentication is available
type AuthProviders struct {
	VaultAvailable bool
	VaultMethod    string // token, approle, kubernetes
	AWSAvailable   bool
	AWSMethod      string // env, iam_role, profile
}

// DetectAuth automatically detects available authentication providers
func DetectAuth(cfg *Config) AuthProviders {
	result := AuthProviders{}

	// Check Vault auth
	if cfg.Vault.Address != "" {
		result.VaultAvailable = true
		if cfg.Vault.Auth.Token != nil && cfg.Vault.Auth.Token.Token != "" {
			result.VaultMethod = "token"
		} else if cfg.Vault.Auth.AppRole != nil {
			result.VaultMethod = "approle"
		} else if cfg.Vault.Auth.Kubernetes != nil {
			result.VaultMethod = "kubernetes"
		}
	}

	// Check AWS auth (via environment or config)
	// AWS SDK auto-detects credentials from env, IAM role, or profile
	if cfg.AWS.Region != "" {
		result.AWSAvailable = true
		// Detect method based on what's configured/available
		if hasAWSEnvCredentials() {
			result.AWSMethod = "env"
		} else if cfg.AWS.ExecutionContext.Type != "" {
			result.AWSMethod = "iam_role"
		} else {
			result.AWSMethod = "default_chain"
		}
	}

	log.WithFields(log.Fields{
		"vaultAvailable": result.VaultAvailable,
		"vaultMethod":    result.VaultMethod,
		"awsAvailable":   result.AWSAvailable,
		"awsMethod":      result.AWSMethod,
	}).Debug("Detected authentication providers")

	return result
}

// hasAWSEnvCredentials checks if AWS credentials are in environment
func hasAWSEnvCredentials() bool {
	// Check standard AWS environment variables
	envVars := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_PROFILE",
		"AWS_ROLE_ARN",
	}
	for _, v := range envVars {
		if val := getenv(v); val != "" {
			return true
		}
	}
	return false
}

// getenv is a wrapper for os.Getenv (allows testing)
var getenv = func(key string) string {
	return strings.TrimSpace(osGetenv(key))
}

// osGetenv wraps os.Getenv - can be replaced in tests
var osGetenv = os.Getenv

// AutoResolveConfig takes a minimal config with just names and resolves everything
func AutoResolveConfig(ctx context.Context, cfg *Config, awsCtx *AWSExecutionContext) error {
	l := log.WithField("action", "AutoResolveConfig")

	// Initialize resolver
	resolver := NewResourceResolver(ctx, cfg)
	if err := resolver.Initialize(awsCtx); err != nil {
		return fmt.Errorf("failed to initialize resolver: %w", err)
	}

	// Resolve sources that don't have explicit type
	for name, source := range cfg.Sources {
		if source.Vault == nil && source.AWS == nil {
			// Need to resolve what this source is
			resolved := resolver.Resolve(name)
			l.WithFields(log.Fields{
				"source":     name,
				"resolvedTo": resolved.Type,
				"confidence": resolved.MatchConfidence,
			}).Info("Auto-resolved source")

			switch resolved.Type {
			case ResourceTypeVaultMount:
				cfg.Sources[name] = Source{
					Vault: &VaultSource{Mount: resolved.ResolvedName},
				}
			case ResourceTypeAWSAccount:
				cfg.Sources[name] = Source{
					AWS: &AWSSource{
						AccountID: resolved.AccountID,
						Region:    cfg.AWS.Region,
					},
				}
			}
		}
	}

	// Resolve target imports
	for targetName, target := range cfg.Targets {
		for i, imp := range target.Imports {
			// Check if import is already a known source or target
			if _, ok := cfg.Sources[imp]; ok {
				continue // Already a known source
			}
			if _, ok := cfg.Targets[imp]; ok {
				continue // Already a known target (inheritance)
			}

			// Try to resolve
			resolved := resolver.Resolve(imp)
			if resolved.Type == ResourceTypeAWSAccount && resolved.MatchConfidence != MatchNone {
				l.WithFields(log.Fields{
					"target":     targetName,
					"import":     imp,
					"resolvedTo": resolved.ResolvedName,
					"accountId":  resolved.AccountID,
					"confidence": resolved.MatchConfidence,
				}).Info("Auto-resolved import to AWS account")

				// Update the import to use the resolved name
				target.Imports[i] = resolved.ResolvedName

				// Add as a source if not exists
				if _, ok := cfg.Sources[resolved.ResolvedName]; !ok {
					cfg.Sources[resolved.ResolvedName] = Source{
						AWS: &AWSSource{
							AccountID: resolved.AccountID,
							Region:    cfg.AWS.Region,
						},
					}
				}
			}
		}
		cfg.Targets[targetName] = target
	}

	return nil
}
