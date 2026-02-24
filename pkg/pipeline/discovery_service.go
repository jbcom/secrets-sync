// Package pipeline provides dynamic target discovery from AWS Organizations and Identity Center.
package pipeline

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// DiscoveryService handles dynamic target discovery from AWS services
type DiscoveryService struct {
	ctx    context.Context
	awsCtx *AWSExecutionContext
	config *Config

	// OU caching (v1.2.0)
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
		dtLog := l.WithField("dynamicTarget", dynamicName)
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
				dtLog.WithField("accountID", acct.ID).Debug("Account excluded")
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

			dtLog.WithFields(log.Fields{
				"targetName":    targetName,
				"accountID":     acct.ID,
				"region":        region,
				"importsCount":  len(imports),
				"fuzzyMatching": nameMatcher != nil,
			}).Debug("Discovered target")
		}
	}

	l.WithField("count", len(discoveredTargets)).Info("Dynamic target discovery completed")
	return discoveredTargets, nil
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
			l.WithField("target", name).Warn("Dynamic target name conflicts with static target, skipping")
		}
	}

	l.WithField("totalTargets", len(cfg.Targets)).Info("Dynamic targets expanded")
	return nil
}
