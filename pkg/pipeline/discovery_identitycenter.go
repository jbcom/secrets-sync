package pipeline

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	log "github.com/sirupsen/logrus"
)

// discoverFromIdentityCenter discovers accounts from AWS Identity Center
func (d *DiscoveryService) discoverFromIdentityCenter(cfg *IdentityCenterDiscovery) ([]AccountInfo, error) {
	l := log.WithFields(log.Fields{
		"action": "discoverFromIdentityCenter",
		"group":  cfg.Group,
	})
	l.Debug("Discovering accounts from Identity Center")

	if !d.awsCtx.CanAccessIdentityCenter() {
		return nil, fmt.Errorf("no access to Identity Center from this execution context")
	}

	// Get Identity Center client
	ssoClient, err := d.awsCtx.GetIdentityCenterClient(d.ctx)
	if err != nil {
		return nil, err
	}

	// Get Identity Store client for group lookups
	idStoreClient := identitystore.NewFromConfig(d.awsCtx.BaseConfig)

	// List SSO instances to get the identity store ID
	instancesOutput, err := ssoClient.ListInstances(d.ctx, &ssoadmin.ListInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list SSO instances: %w", err)
	}

	if len(instancesOutput.Instances) == 0 {
		return nil, fmt.Errorf("no SSO instances found")
	}

	instance := instancesOutput.Instances[0]
	identityStoreID := aws.ToString(instance.IdentityStoreId)
	instanceARN := aws.ToString(instance.InstanceArn)

	var accounts []AccountInfo

	if cfg.Group != "" {
		// Find group by name
		groupID, err := d.findGroupByName(idStoreClient, identityStoreID, cfg.Group)
		if err != nil {
			return nil, fmt.Errorf("failed to find group %q: %w", cfg.Group, err)
		}

		// Get accounts assigned to this group
		accounts, err = d.getAccountsForGroup(ssoClient, instanceARN, groupID)
		if err != nil {
			return nil, err
		}
	}

	if cfg.PermissionSet != "" {
		// Get accounts with this permission set
		psAccounts, err := d.getAccountsWithPermissionSet(ssoClient, instanceARN, cfg.PermissionSet)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, psAccounts...)
	}

	// Deduplicate accounts
	accounts = deduplicateAccounts(accounts)

	l.WithField("count", len(accounts)).Debug("Discovered accounts from Identity Center")
	return accounts, nil
}

// findGroupByName finds an Identity Store group by display name
func (d *DiscoveryService) findGroupByName(client *identitystore.Client, storeID, groupName string) (string, error) {
	paginator := identitystore.NewListGroupsPaginator(client, &identitystore.ListGroupsInput{
		IdentityStoreId: aws.String(storeID),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(d.ctx)
		if err != nil {
			return "", err
		}

		for _, group := range output.Groups {
			if aws.ToString(group.DisplayName) == groupName {
				return aws.ToString(group.GroupId), nil
			}
		}
	}

	return "", fmt.Errorf("group not found: %s", groupName)
}

// getAccountsForGroup gets AWS accounts assigned to an Identity Center group
func (d *DiscoveryService) getAccountsForGroup(client *ssoadmin.Client, instanceARN, groupID string) ([]AccountInfo, error) {
	var accounts []AccountInfo
	seen := make(map[string]bool)

	// List permission sets for this group
	paginator := ssoadmin.NewListPermissionSetsPaginator(client, &ssoadmin.ListPermissionSetsInput{
		InstanceArn: aws.String(instanceARN),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(d.ctx)
		if err != nil {
			return nil, err
		}

		for _, psARN := range output.PermissionSets {
			// List account assignments for this permission set
			assignmentsPaginator := ssoadmin.NewListAccountAssignmentsPaginator(client, &ssoadmin.ListAccountAssignmentsInput{
				InstanceArn:      aws.String(instanceARN),
				PermissionSetArn: aws.String(psARN),
				AccountId:        nil, // List all accounts
			})

			for assignmentsPaginator.HasMorePages() {
				assignOutput, err := assignmentsPaginator.NextPage(d.ctx)
				if err != nil {
					continue // Skip errors for individual permission sets
				}

				for _, assignment := range assignOutput.AccountAssignments {
					if aws.ToString(assignment.PrincipalId) == groupID {
						accountID := aws.ToString(assignment.AccountId)
						if !seen[accountID] {
							seen[accountID] = true
							accounts = append(accounts, AccountInfo{
								ID: accountID,
							})
						}
					}
				}
			}
		}
	}

	// Enrich with account names from Organizations
	if d.awsCtx.CanAccessOrganizations() {
		allAccounts, _ := d.awsCtx.ListOrganizationAccounts(d.ctx)
		accountMap := make(map[string]AccountInfo)
		for _, a := range allAccounts {
			accountMap[a.ID] = a
		}
		for i, a := range accounts {
			if enriched, ok := accountMap[a.ID]; ok {
				accounts[i] = enriched
			}
		}
	}

	return accounts, nil
}

// getAccountsWithPermissionSet gets accounts with a specific permission set
func (d *DiscoveryService) getAccountsWithPermissionSet(client *ssoadmin.Client, instanceARN, permissionSetName string) ([]AccountInfo, error) {
	// First, find the permission set ARN by name
	var permissionSetARN string
	paginator := ssoadmin.NewListPermissionSetsPaginator(client, &ssoadmin.ListPermissionSetsInput{
		InstanceArn: aws.String(instanceARN),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(d.ctx)
		if err != nil {
			return nil, err
		}

		for _, psARN := range output.PermissionSets {
			// Get permission set details
			details, err := client.DescribePermissionSet(d.ctx, &ssoadmin.DescribePermissionSetInput{
				InstanceArn:      aws.String(instanceARN),
				PermissionSetArn: aws.String(psARN),
			})
			if err != nil {
				continue
			}

			if aws.ToString(details.PermissionSet.Name) == permissionSetName {
				permissionSetARN = psARN
				break
			}
		}

		if permissionSetARN != "" {
			break
		}
	}

	if permissionSetARN == "" {
		return nil, fmt.Errorf("permission set not found: %s", permissionSetName)
	}

	// List accounts provisioned with this permission set
	var accounts []AccountInfo
	accountsPaginator := ssoadmin.NewListAccountsForProvisionedPermissionSetPaginator(client, &ssoadmin.ListAccountsForProvisionedPermissionSetInput{
		InstanceArn:      aws.String(instanceARN),
		PermissionSetArn: aws.String(permissionSetARN),
	})

	for accountsPaginator.HasMorePages() {
		output, err := accountsPaginator.NextPage(d.ctx)
		if err != nil {
			return nil, err
		}

		for _, accountID := range output.AccountIds {
			accounts = append(accounts, AccountInfo{
				ID: accountID,
			})
		}
	}

	return accounts, nil
}
