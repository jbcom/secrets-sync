// Package pipeline provides dynamic target discovery from AWS Organizations and Identity Center.
package pipeline

import (
	"context"
	"fmt"
	"strings"

	reqctx "github.com/jbcom/secrets-sync/pkg/context"
	log "github.com/sirupsen/logrus"
)

// DiscoveryService handles dynamic target discovery from AWS services
type DiscoveryService struct {
	ctx    context.Context
	awsCtx *AWSExecutionContext
	config *Config

	// OU caching
	ouCache      map[string][]AccountInfo // Cache OU -> accounts mapping
	ouChildCache map[string][]string      // Cache OU -> child OUs mapping
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(ctx context.Context, awsCtx *AWSExecutionContext, cfg *Config) *DiscoveryService {
	return &DiscoveryService{
		ctx:          ctx,
		awsCtx:       awsCtx,
		config:       cfg,
		ouCache:      make(map[string][]AccountInfo),
		ouChildCache: make(map[string][]string),
	}
}

// DiscoverTargets discovers and expands dynamic targets into concrete targets
func (d *DiscoveryService) DiscoverTargets() (map[string]Target, error) {
	l := log.WithFields(log.Fields{
		"action": "DiscoveryService.DiscoverTargets",
	})
	l.Info("Starting dynamic target discovery")

	discoveredTargets := make(map[string]Target)

	for dynamicName, dynamicTarget := range d.config.DynamicTargets {
		dtLog := l.WithField("dynamicTarget", reqctx.SafeLogValue(dynamicName))
		dtLog.Debug("Processing dynamic target")

		var accounts []AccountInfo
		var err error

		// Discover from Identity Center
		if dynamicTarget.Discovery.IdentityCenter != nil {
			accounts, err = d.discoverFromIdentityCenter(dynamicTarget.Discovery.IdentityCenter)
			if err != nil {
				dtLog.WithError(err).Warn("Failed to discover from Identity Center")
				continue
			}
		}

		// Discover from Organizations
		if dynamicTarget.Discovery.Organizations != nil {
			orgAccounts, err := d.discoverFromOrganizations(dynamicTarget.Discovery.Organizations)
			if err != nil {
				dtLog.WithError(err).Warn("Failed to discover from Organizations")
				continue
			}
			accounts = append(accounts, orgAccounts...)
		}

		// Discover from external account list (e.g., SSM Parameter Store)
		if dynamicTarget.Discovery.AccountsList != nil {
			listAccounts, err := d.discoverFromAccountsList(dynamicTarget.Discovery.AccountsList)
			if err != nil {
				dtLog.WithError(err).Warn("Failed to discover from accounts list")
				continue
			}
			accounts = append(accounts, listAccounts...)
		}

		// Deduplicate accounts
		accounts = deduplicateAccounts(accounts)

		// Initialize name matcher for fuzzy matching if configured
		var nameMatcher *NameMatcher
		if dynamicTarget.Discovery.Organizations != nil && dynamicTarget.Discovery.Organizations.NameMatching != nil {
			nameMatcher = NewNameMatcher(dynamicTarget.Discovery.Organizations.NameMatching)
		}

		// Convert discovered accounts to targets
		for _, acct := range accounts {
			// Check exclusions
			if isExcluded(acct.ID, dynamicTarget.Exclude) {
				dtLog.WithField("accountID", reqctx.SafeLogValue(acct.ID)).Debug("Account excluded")
				continue
			}

			// Create target name from account name or ID
			targetName := sanitizeTargetName(acct.Name)
			if targetName == "" {
				targetName = fmt.Sprintf("account_%s", acct.ID)
			}

			// Ensure uniqueness by appending account ID suffix
			if _, exists := discoveredTargets[targetName]; exists {
				targetName = fmt.Sprintf("%s_%s", targetName, acct.ID[:6])
			}

			// Apply dynamic target options with fallbacks to config defaults
			region := dynamicTarget.Region
			if region == "" {
				region = d.config.AWS.Region
			}

			// Process role ARN template (supports {{.AccountID}})
			roleARN := dynamicTarget.RoleARN
			if roleARN != "" {
				roleARN = strings.ReplaceAll(roleARN, "{{.AccountID}}", acct.ID)
			}

			// Resolve imports using fuzzy matching if patterns configured
			imports := dynamicTarget.Imports
			if nameMatcher != nil && len(dynamicTarget.AccountNamePatterns) > 0 {
				imports = nameMatcher.ResolveAccountImports(
					acct,
					dynamicTarget.AccountNamePatterns,
					dynamicTarget.Imports,
					d.config.Targets,
				)
			}

			discoveredTargets[targetName] = Target{
				AccountID:    acct.ID,
				Imports:      imports,
				Region:       region,
				SecretPrefix: dynamicTarget.SecretPrefix,
				RoleARN:      roleARN,
			}

			// Account IDs, names and regions come back from the Organizations
			// and Identity Center APIs, so they are external input here.
			// targetName is only constrained by sanitizeTargetName when it was
			// derived from acct.Name; both fallbacks above interpolate the raw
			// account ID, so it needs sanitizing too.
			dtLog.WithFields(log.Fields{
				"targetName":    reqctx.SafeLogValue(targetName),
				"accountID":     reqctx.SafeLogValue(acct.ID),
				"region":        reqctx.SafeLogValue(region),
				"importsCount":  len(imports),
				"fuzzyMatching": nameMatcher != nil,
			}).Debug("Discovered target")
		}
	}

	l.WithField("count", len(discoveredTargets)).Info("Dynamic target discovery completed")
	return discoveredTargets, nil
}

// expandDynamicTargets resolves the config's dynamic targets into concrete ones
// and rebuilds the dependency graph so the newly discovered targets are
// actually executed.
//
// Discovery needs AWS, so a config that asks for dynamic targets without a
// usable AWS execution context is a hard error rather than a warning: the
// alternative is a pipeline that runs over an empty target set and reports
// success, which for a credential-syncing tool is worse than failing.
func (p *Pipeline) expandDynamicTargets(ctx context.Context, cfg *Config) error {
	if len(cfg.DynamicTargets) == 0 {
		return nil
	}

	// A nil awsCtx is normal rather than fatal: it just means
	// aws.execution_context.type was omitted, which is the ambient-credential
	// setup where the SDK's default chain applies. Discovery handles that, so
	// only a discovery failure is an error.
	staticCount := len(cfg.Targets)

	if err := ExpandDynamicTargets(ctx, cfg, p.awsCtx); err != nil {
		return err
	}

	// Compare against the pre-expansion count. Checking len(cfg.Targets) alone
	// would be satisfied by any static target, hiding the case where every
	// discovery provider failed or returned nothing -- DiscoverTargets logs
	// provider errors and returns what it has, so silence here is not success.
	if len(cfg.Targets) == staticCount {
		return fmt.Errorf("dynamic target discovery resolved no targets; " +
			"refusing to run a pipeline whose dynamic targets would all be missing")
	}

	graph, err := BuildGraph(cfg)
	if err != nil {
		return fmt.Errorf("failed to rebuild dependency graph after expanding dynamic targets: %w", err)
	}
	p.graph = graph

	return nil
}

// ExpandDynamicTargets expands dynamic targets in the config and merges them with static targets
func ExpandDynamicTargets(ctx context.Context, cfg *Config, awsCtx *AWSExecutionContext) error {
	if len(cfg.DynamicTargets) == 0 {
		return nil
	}

	l := log.WithFields(log.Fields{
		"action": "ExpandDynamicTargets",
	})
	l.Info("Expanding dynamic targets")

	discovery := NewDiscoveryService(ctx, awsCtx, cfg)
	discovered, err := discovery.DiscoverTargets()
	if err != nil {
		return fmt.Errorf("failed to discover dynamic targets: %w", err)
	}

	// Merge discovered targets with static targets
	if cfg.Targets == nil {
		cfg.Targets = make(map[string]Target)
	}

	for name, target := range discovered {
		// Don't overwrite static targets
		if _, exists := cfg.Targets[name]; !exists {
			cfg.Targets[name] = target
		} else {
			l.WithField("target", reqctx.SafeLogValue(name)).Warn("Dynamic target name conflicts with static target, skipping")
		}
	}

	l.WithField("totalTargets", len(cfg.Targets)).Info("Dynamic targets expanded")
	return nil
}
