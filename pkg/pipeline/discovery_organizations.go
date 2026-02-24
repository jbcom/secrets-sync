package pipeline

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// discoverFromOrganizations discovers accounts from AWS Organizations
func (d *DiscoveryService) discoverFromOrganizations(cfg *OrganizationsDiscovery) ([]AccountInfo, error) {
	l := log.WithFields(log.Fields{
		"action":    "discoverFromOrganizations",
		"ou":        cfg.OU,
		"ous":       cfg.OUs,
		"recursive": cfg.Recursive,
	})
	l.Debug("Discovering accounts from Organizations")

	if !d.awsCtx.CanAccessOrganizations() {
		return nil, fmt.Errorf("no access to Organizations API from this execution context")
	}

	var accounts []AccountInfo

	// Collect all OUs to process (legacy single OU + new multiple OUs)
	var ousToProcess []string
	if cfg.OU != "" {
		ousToProcess = append(ousToProcess, cfg.OU)
	}
	ousToProcess = append(ousToProcess, cfg.OUs...)

	// Discover by OUs
	if len(ousToProcess) > 0 {
		for _, ou := range ousToProcess {
			if cfg.Recursive {
				// Recursive traversal of OU and all child OUs
				ouAccounts, err := d.listAccountsInOURecursive(ou)
				if err != nil {
					return nil, fmt.Errorf("failed to discover accounts in OU %s: %w", ou, err)
				}
				accounts = append(accounts, ouAccounts...)
			} else {
				// Direct children only
				ouAccounts, err := d.awsCtx.ListAccountsInOU(d.ctx, ou)
				if err != nil {
					return nil, fmt.Errorf("failed to list accounts in OU %s: %w", ou, err)
				}
				accounts = append(accounts, ouAccounts...)
			}
		}
	}

	// If no OUs specified but tags are specified, list all accounts and filter
	if len(ousToProcess) == 0 && (len(cfg.Tags) > 0 || len(cfg.TagFilters) > 0) {
		allAccounts, err := d.awsCtx.ListOrganizationAccounts(d.ctx)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, allAccounts...)
	}

	// Filter by tags if specified (legacy format)
	if len(cfg.Tags) > 0 {
		accounts = filterAccountsByTags(accounts, cfg.Tags)
	}

	// Filter by enhanced tag filters if specified (v1.2.0)
	if len(cfg.TagFilters) > 0 {
		combination := cfg.TagCombination
		if combination == "" {
			combination = "AND" // default
		}
		accounts = filterAccountsByTagFilters(accounts, cfg.TagFilters, combination)
	}

	// Filter by account status if specified
	if len(cfg.ExcludeStatuses) > 0 {
		accounts = filterAccountsByStatus(accounts, cfg.ExcludeStatuses)
	}

	l.WithField("count", len(accounts)).Debug("Discovered accounts from Organizations")
	return accounts, nil
}

// listAccountsInOURecursive recursively lists accounts in an OU and all child OUs
func (d *DiscoveryService) listAccountsInOURecursive(ouID string) ([]AccountInfo, error) {
	var accounts []AccountInfo

	// Get accounts directly in this OU (with caching)
	ouAccounts, err := d.listAccountsInOUCached(ouID)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts in OU %s: %w", ouID, err)
	}
	accounts = append(accounts, ouAccounts...)

	// Get child OUs and recurse (with caching)
	childOUs, err := d.listChildOUsCached(ouID)
	if err != nil {
		// Log but continue - we might not have permission to list child OUs
		log.WithError(err).WithField("ou", ouID).Debug("Could not list child OUs")
		return accounts, nil
	}

	for _, childOU := range childOUs {
		childAccounts, err := d.listAccountsInOURecursive(childOU)
		if err != nil {
			log.WithError(err).WithField("childOU", childOU).Debug("Error recursing into child OU")
			continue
		}
		accounts = append(accounts, childAccounts...)
	}

	return accounts, nil
}

// listAccountsInOUCached lists accounts in an OU with caching
func (d *DiscoveryService) listAccountsInOUCached(ouID string) ([]AccountInfo, error) {
	// Check cache first
	if accounts, exists := d.ouCache[ouID]; exists {
		log.WithField("ou", ouID).Debug("Using cached OU accounts")
		return accounts, nil
	}

	// Fetch from API
	accounts, err := d.awsCtx.ListAccountsInOU(d.ctx, ouID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	d.ouCache[ouID] = accounts
	log.WithFields(log.Fields{
		"ou":    ouID,
		"count": len(accounts),
	}).Debug("Cached OU accounts")

	return accounts, nil
}

// listChildOUsCached lists child OUs with caching
func (d *DiscoveryService) listChildOUsCached(ouID string) ([]string, error) {
	// Check cache first
	if childOUs, exists := d.ouChildCache[ouID]; exists {
		log.WithField("ou", ouID).Debug("Using cached child OUs")
		return childOUs, nil
	}

	// Fetch from API
	childOUs, err := d.awsCtx.ListChildOUs(d.ctx, ouID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	d.ouChildCache[ouID] = childOUs
	log.WithFields(log.Fields{
		"ou":    ouID,
		"count": len(childOUs),
	}).Debug("Cached child OUs")

	return childOUs, nil
}
