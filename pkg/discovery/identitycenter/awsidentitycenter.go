package identitycenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/driver"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IdentityCenterClient provides AWS Identity Center (SSO) account discovery
// This enables dynamic discovery of AWS accounts based on group membership
// which is useful for sandbox/developer account targeting patterns.
type IdentityCenterClient struct {
	// Region for Identity Center (typically us-east-1)
	Region string `yaml:"region,omitempty" json:"region,omitempty"`
	// IdentityStoreID is the Identity Store ID (auto-discovered if empty)
	IdentityStoreID string `yaml:"identityStoreId,omitempty" json:"identityStoreId,omitempty"`
	// InstanceARN is the SSO Instance ARN (auto-discovered if empty)
	InstanceARN string `yaml:"instanceArn,omitempty" json:"instanceArn,omitempty"`
	// RoleArn for cross-account access to Identity Center
	RoleArn string `yaml:"roleArn,omitempty" json:"roleArn,omitempty"`

	// GroupName to discover members from
	GroupName string `yaml:"groupName,omitempty" json:"groupName,omitempty"`
	// GroupID is resolved from GroupName (or can be specified directly)
	GroupID string `yaml:"groupId,omitempty" json:"groupId,omitempty"`

	// AccountMapping maps user emails to account configurations
	// Key is email pattern (supports wildcards), value is account config
	AccountMapping map[string]AccountConfig `yaml:"accountMapping,omitempty" json:"accountMapping,omitempty"`

	// OutputFormat controls how discovered accounts are formatted
	// Options: "json", "yaml", "list"
	OutputFormat string `yaml:"outputFormat,omitempty" json:"outputFormat,omitempty"`

	// Enhanced discovery (v1.2.0)
	DiscoverPermissionSets bool `yaml:"discoverPermissionSets,omitempty" json:"discoverPermissionSets,omitempty"`
	CacheAssignments       bool `yaml:"cacheAssignments,omitempty" json:"cacheAssignments,omitempty"`

	// DiscoveredAccounts holds the results after ListSecrets is called
	DiscoveredAccounts []DiscoveredAccount `yaml:"-" json:"-"`
	// PermissionSets holds discovered permission sets
	PermissionSets []PermissionSet `yaml:"-" json:"-"`
	// AccountAssignments holds discovered account assignments
	AccountAssignments []AccountAssignment `yaml:"-" json:"-"`

	identityStoreClient *identitystore.Client `yaml:"-" json:"-"`
	ssoAdminClient      *ssoadmin.Client      `yaml:"-" json:"-"`

	// Caching (v1.2.0)
	assignmentCache    map[string][]AccountAssignment `yaml:"-" json:"-"`
	permissionSetCache map[string]PermissionSet       `yaml:"-" json:"-"`
}

// AccountConfig defines the configuration for an AWS account
type AccountConfig struct {
	AccountID        string            `yaml:"accountId,omitempty" json:"accountId,omitempty"`
	AccountName      string            `yaml:"accountName,omitempty" json:"accountName,omitempty"`
	ExecutionRoleArn string            `yaml:"executionRoleArn,omitempty" json:"executionRoleArn,omitempty"`
	Classification   string            `yaml:"classification,omitempty" json:"classification,omitempty"`
	Tags             map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// DiscoveredAccount represents an account discovered via Identity Center
type DiscoveredAccount struct {
	Email            string            `json:"email"`
	UserID           string            `json:"userId"`
	Username         string            `json:"username"`
	AccountID        string            `json:"accountId"`
	AccountName      string            `json:"accountName"`
	ExecutionRoleArn string            `json:"executionRoleArn"`
	Classification   string            `json:"classification"`
	Tags             map[string]string `json:"tags,omitempty"`
}

// PermissionSet represents an Identity Center permission set (v1.2.0)
type PermissionSet struct {
	ARN         string `json:"arn"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedDate string `json:"createdDate"`
}

// AccountAssignment represents an account assignment in Identity Center (v1.2.0)
type AccountAssignment struct {
	AccountID        string `json:"accountId"`
	PermissionSetARN string `json:"permissionSetArn"`
	PrincipalType    string `json:"principalType"` // USER or GROUP
	PrincipalID      string `json:"principalId"`
}

// DeepCopyInto copies the receiver into out
func (in *IdentityCenterClient) DeepCopyInto(out *IdentityCenterClient) {
	*out = *in
	if in.AccountMapping != nil {
		out.AccountMapping = make(map[string]AccountConfig, len(in.AccountMapping))
		for k, v := range in.AccountMapping {
			tagsCopy := make(map[string]string, len(v.Tags))
			for tk, tv := range v.Tags {
				tagsCopy[tk] = tv
			}
			v.Tags = tagsCopy
			out.AccountMapping[k] = v
		}
	}
	if in.DiscoveredAccounts != nil {
		out.DiscoveredAccounts = make([]DiscoveredAccount, len(in.DiscoveredAccounts))
		copy(out.DiscoveredAccounts, in.DiscoveredAccounts)
	}
}

// DeepCopy creates a deep copy of the client
func (in *IdentityCenterClient) DeepCopy() *IdentityCenterClient {
	if in == nil {
		return nil
	}
	out := new(IdentityCenterClient)
	in.DeepCopyInto(out)
	return out
}

// Validate ensures required fields are set
func (c *IdentityCenterClient) Validate() error {
	l := log.WithFields(log.Fields{
		"action": "Validate",
		"driver": "awsidentitycenter",
	})
	l.Trace("start")

	if c.GroupName == "" && c.GroupID == "" {
		return errors.New("either groupName or groupId is required")
	}
	return nil
}

// NewClient creates a new Identity Center client from configuration
func NewClient(cfg *IdentityCenterClient) (*IdentityCenterClient, error) {
	l := log.WithFields(log.Fields{
		"action": "NewClient",
		"driver": "awsidentitycenter",
	})
	l.Trace("start")

	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	vc := cfg.DeepCopy()

	if vc.Region == "" {
		vc.Region = "us-east-1"
	}
	if vc.OutputFormat == "" {
		vc.OutputFormat = "json"
	}

	l.Debugf("client created for region=%s groupName=%s", vc.Region, vc.GroupName)
	l.Trace("end")
	return vc, nil
}

// Init initializes the Identity Center client
func (c *IdentityCenterClient) Init(ctx context.Context) error {
	l := log.WithFields(log.Fields{
		"action": "Init",
		"driver": "awsidentitycenter",
	})
	l.Trace("start")

	if err := c.Validate(); err != nil {
		return err
	}

	// Load AWS config
	awscfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Apply role assumption if specified
	if c.RoleArn != "" {
		stsclient := sts.NewFromConfig(awscfg)
		provider := stscreds.NewAssumeRoleProvider(stsclient, c.RoleArn)
		awscfg.Credentials = provider
	}

	// Create clients
	c.identityStoreClient = identitystore.NewFromConfig(awscfg)
	c.ssoAdminClient = ssoadmin.NewFromConfig(awscfg)

	// Initialize caches
	if c.CacheAssignments {
		c.assignmentCache = make(map[string][]AccountAssignment)
		c.permissionSetCache = make(map[string]PermissionSet)
	}

	// Auto-discover Identity Store ID and Instance ARN if not provided
	if c.IdentityStoreID == "" || c.InstanceARN == "" {
		storeID, instanceARN, err := c.discoverInstanceInfo(ctx)
		if err != nil {
			return fmt.Errorf("failed to discover instance info: %w", err)
		}
		if c.IdentityStoreID == "" {
			c.IdentityStoreID = storeID
			l.Infof("discovered identity store ID: %s", c.IdentityStoreID)
		}
		if c.InstanceARN == "" {
			c.InstanceARN = instanceARN
			l.Infof("discovered instance ARN: %s", c.InstanceARN)
		}
	}

	// Resolve group ID from group name if needed
	if c.GroupID == "" && c.GroupName != "" {
		groupID, err := c.resolveGroupID(ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve group ID: %w", err)
		}
		c.GroupID = groupID
		l.Infof("resolved group '%s' to ID: %s", c.GroupName, c.GroupID)
	}

	l.Trace("end")
	return nil
}

// discoverInstanceInfo auto-discovers the Identity Store ID and Instance ARN from SSO instances
func (c *IdentityCenterClient) discoverInstanceInfo(ctx context.Context) (string, string, error) {
	resp, err := c.ssoAdminClient.ListInstances(ctx, &ssoadmin.ListInstancesInput{})
	if err != nil {
		return "", "", err
	}
	if len(resp.Instances) == 0 {
		return "", "", errors.New("no SSO instances found")
	}

	instance := resp.Instances[0]
	return aws.ToString(instance.IdentityStoreId), aws.ToString(instance.InstanceArn), nil
}

// resolveGroupID resolves a group name to its ID
func (c *IdentityCenterClient) resolveGroupID(ctx context.Context) (string, error) {
	resp, err := c.identityStoreClient.ListGroups(ctx, &identitystore.ListGroupsInput{
		IdentityStoreId: aws.String(c.IdentityStoreID),
		Filters: []identitystoretypes.Filter{
			{
				AttributePath:  aws.String("DisplayName"),
				AttributeValue: aws.String(c.GroupName),
			},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Groups) == 0 {
		return "", fmt.Errorf("group '%s' not found", c.GroupName)
	}
	return aws.ToString(resp.Groups[0].GroupId), nil
}

// Driver returns the driver name
func (c *IdentityCenterClient) Driver() driver.DriverName {
	return driver.DriverNameIdentityCenter
}

// GetPath returns the path identifier for this store
func (c *IdentityCenterClient) GetPath() string {
	return fmt.Sprintf("identitycenter/%s/%s", c.IdentityStoreID, c.GroupID)
}

// Meta returns metadata about the client configuration
func (c *IdentityCenterClient) Meta() map[string]any {
	return map[string]any{
		"region":          c.Region,
		"identityStoreId": c.IdentityStoreID,
		"groupName":       c.GroupName,
		"groupId":         c.GroupID,
	}
}

// GetSecret retrieves discovered account info (not typically used)
func (c *IdentityCenterClient) GetSecret(ctx context.Context, name string) ([]byte, error) {
	// Find the account by name
	for _, account := range c.DiscoveredAccounts {
		if account.AccountName == name || account.Email == name {
			return json.Marshal(account)
		}
	}
	return nil, fmt.Errorf("account not found: %s", name)
}

// WriteSecret is not supported for Identity Center (read-only discovery)
func (c *IdentityCenterClient) WriteSecret(ctx context.Context, meta metav1.ObjectMeta, path string, bSecrets []byte) ([]byte, error) {
	return nil, errors.New("identity center store is read-only (discovery only)")
}

// DeleteSecret is not supported for Identity Center (read-only discovery)
func (c *IdentityCenterClient) DeleteSecret(ctx context.Context, name string) error {
	return errors.New("identity center store is read-only (discovery only)")
}

// ListSecrets discovers accounts from the Identity Center group
func (c *IdentityCenterClient) ListSecrets(ctx context.Context, path string) ([]string, error) {
	l := log.WithFields(log.Fields{
		"action":  "ListSecrets",
		"driver":  "awsidentitycenter",
		"groupId": c.GroupID,
	})
	l.Trace("start")
	defer l.Trace("end")

	// Get group members
	members, err := c.listGroupMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list group members: %w", err)
	}
	l.Infof("found %d members in group", len(members))

	// Match members to accounts
	c.DiscoveredAccounts = c.matchMembersToAccounts(members)
	l.Infof("matched %d accounts", len(c.DiscoveredAccounts))

	// Return account names as "secrets"
	var names []string
	for _, account := range c.DiscoveredAccounts {
		names = append(names, account.AccountName)
	}

	// Discover permission sets if enabled
	if c.DiscoverPermissionSets {
		permissionSets, err := c.listPermissionSets(ctx)
		if err != nil {
			l.Warnf("failed to discover permission sets: %v", err)
		} else {
			c.PermissionSets = permissionSets
			l.Infof("discovered %d permission sets", len(permissionSets))
		}

		// Discover account assignments
		assignments, err := c.listAccountAssignments(ctx)
		if err != nil {
			l.Warnf("failed to discover account assignments: %v", err)
		} else {
			c.AccountAssignments = assignments
			l.Infof("discovered %d account assignments", len(assignments))
		}
	}

	return names, nil
}

// GroupMember represents a member of an Identity Center group
type GroupMember struct {
	UserID   string
	Username string
	Email    string
}

// listGroupMembers retrieves all members of the configured group
func (c *IdentityCenterClient) listGroupMembers(ctx context.Context) ([]GroupMember, error) {
	var members []GroupMember

	paginator := identitystore.NewListGroupMembershipsPaginator(c.identityStoreClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: aws.String(c.IdentityStoreID),
		GroupId:         aws.String(c.GroupID),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, membership := range page.GroupMemberships {
			// Extract user ID from MemberId
			if membership.MemberId == nil {
				continue
			}
			userMember, ok := membership.MemberId.(*identitystoretypes.MemberIdMemberUserId)
			if !ok {
				continue
			}

			// Get user details
			userResp, err := c.identityStoreClient.DescribeUser(ctx, &identitystore.DescribeUserInput{
				IdentityStoreId: aws.String(c.IdentityStoreID),
				UserId:          aws.String(userMember.Value),
			})
			if err != nil {
				log.Warnf("failed to describe user %s: %v", userMember.Value, err)
				continue
			}

			// Find primary email
			var email string
			for _, e := range userResp.Emails {
				if e.Primary {
					email = aws.ToString(e.Value)
					break
				}
			}
			if email == "" && len(userResp.Emails) > 0 {
				email = aws.ToString(userResp.Emails[0].Value)
			}

			if email != "" {
				members = append(members, GroupMember{
					UserID:   userMember.Value,
					Username: aws.ToString(userResp.UserName),
					Email:    strings.ToLower(email),
				})
			}
		}
	}

	return members, nil
}

// matchMembersToAccounts matches group members to account configurations
func (c *IdentityCenterClient) matchMembersToAccounts(members []GroupMember) []DiscoveredAccount {
	var accounts []DiscoveredAccount

	for _, member := range members {
		// Try to match email to account mapping
		for pattern, accountCfg := range c.AccountMapping {
			if matchEmailPattern(member.Email, pattern) {
				accounts = append(accounts, DiscoveredAccount{
					Email:            member.Email,
					UserID:           member.UserID,
					Username:         member.Username,
					AccountID:        accountCfg.AccountID,
					AccountName:      accountCfg.AccountName,
					ExecutionRoleArn: accountCfg.ExecutionRoleArn,
					Classification:   accountCfg.Classification,
					Tags:             accountCfg.Tags,
				})
			}
		}
	}

	return accounts
}

// matchEmailPattern checks if an email matches a pattern (supports * wildcard)
func matchEmailPattern(email, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(email, parts[0]) && strings.HasSuffix(email, parts[1])
		}
	}
	return strings.EqualFold(email, pattern)
}

// Close cleans up the client
func (c *IdentityCenterClient) Close() error {
	c.identityStoreClient = nil
	c.ssoAdminClient = nil
	return nil
}

// SetDefaults applies default values from configuration
func (c *IdentityCenterClient) SetDefaults(cfg any) error {
	jd, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	nc := &IdentityCenterClient{}
	if err := json.Unmarshal(jd, nc); err != nil {
		return err
	}

	if c.Region == "" && nc.Region != "" {
		c.Region = nc.Region
	}
	if c.IdentityStoreID == "" && nc.IdentityStoreID != "" {
		c.IdentityStoreID = nc.IdentityStoreID
	}
	if c.RoleArn == "" && nc.RoleArn != "" {
		c.RoleArn = nc.RoleArn
	}
	if c.OutputFormat == "" && nc.OutputFormat != "" {
		c.OutputFormat = nc.OutputFormat
	}

	return nil
}

// listPermissionSets discovers all permission sets in the Identity Center instance (v1.2.0)
func (c *IdentityCenterClient) listPermissionSets(ctx context.Context) ([]PermissionSet, error) {
	var permissionSets []PermissionSet

	// List permission set ARNs
	paginator := ssoadmin.NewListPermissionSetsPaginator(c.ssoAdminClient, &ssoadmin.ListPermissionSetsInput{
		InstanceArn: aws.String(c.InstanceARN),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list permission sets: %w", err)
		}

		// Get details for each permission set
		for _, psArn := range page.PermissionSets {
			// Check cache first
			if c.CacheAssignments {
				if cached, exists := c.permissionSetCache[psArn]; exists {
					permissionSets = append(permissionSets, cached)
					continue
				}
			}

			// Fetch from API
			resp, err := c.ssoAdminClient.DescribePermissionSet(ctx, &ssoadmin.DescribePermissionSetInput{
				InstanceArn:      aws.String(c.InstanceARN),
				PermissionSetArn: aws.String(psArn),
			})
			if err != nil {
				log.Warnf("failed to describe permission set %s: %v", psArn, err)
				continue
			}

			ps := PermissionSet{
				ARN:         psArn,
				Name:        aws.ToString(resp.PermissionSet.Name),
				Description: aws.ToString(resp.PermissionSet.Description),
				CreatedDate: resp.PermissionSet.CreatedDate.String(),
			}

			// Cache the result
			if c.CacheAssignments {
				c.permissionSetCache[psArn] = ps
			}

			permissionSets = append(permissionSets, ps)
		}
	}

	return permissionSets, nil
}

// listAccountAssignments discovers all account assignments for permission sets (v1.2.0)
func (c *IdentityCenterClient) listAccountAssignments(ctx context.Context) ([]AccountAssignment, error) {
	var assignments []AccountAssignment

	// Get all permission set ARNs first
	psResp, err := c.ssoAdminClient.ListPermissionSets(ctx, &ssoadmin.ListPermissionSetsInput{
		InstanceArn: aws.String(c.InstanceARN),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list permission sets for assignments: %w", err)
	}

	// For each permission set, list its account assignments
	for _, psArn := range psResp.PermissionSets {
		// Check cache first
		cacheKey := fmt.Sprintf("assignments:%s", psArn)
		if c.CacheAssignments {
			if cached, exists := c.assignmentCache[cacheKey]; exists {
				assignments = append(assignments, cached...)
				continue
			}
		}

		// Fetch from API
		psAssignments, err := c.listAccountAssignmentsForPermissionSet(ctx, psArn)
		if err != nil {
			log.Warnf("failed to list assignments for permission set %s: %v", psArn, err)
			continue
		}

		// Cache the result
		if c.CacheAssignments {
			c.assignmentCache[cacheKey] = psAssignments
		}

		assignments = append(assignments, psAssignments...)
	}

	return assignments, nil
}

// listAccountAssignmentsForPermissionSet lists assignments for a specific permission set
func (c *IdentityCenterClient) listAccountAssignmentsForPermissionSet(ctx context.Context, permissionSetArn string) ([]AccountAssignment, error) {
	var assignments []AccountAssignment

	paginator := ssoadmin.NewListAccountAssignmentsPaginator(c.ssoAdminClient, &ssoadmin.ListAccountAssignmentsInput{
		InstanceArn:      aws.String(c.InstanceARN),
		PermissionSetArn: aws.String(permissionSetArn),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, assignment := range page.AccountAssignments {
			assignments = append(assignments, AccountAssignment{
				AccountID:        aws.ToString(assignment.AccountId),
				PermissionSetARN: permissionSetArn,
				PrincipalType:    string(assignment.PrincipalType),
				PrincipalID:      aws.ToString(assignment.PrincipalId),
			})
		}
	}

	return assignments, nil
}

// GetPermissionSetByName finds a permission set by name (v1.2.0)
func (c *IdentityCenterClient) GetPermissionSetByName(name string) *PermissionSet {
	for _, ps := range c.PermissionSets {
		if ps.Name == name {
			return &ps
		}
	}
	return nil
}

// GetAccountAssignmentsForAccount returns all assignments for a specific account (v1.2.0)
func (c *IdentityCenterClient) GetAccountAssignmentsForAccount(accountID string) []AccountAssignment {
	var assignments []AccountAssignment
	for _, assignment := range c.AccountAssignments {
		if assignment.AccountID == accountID {
			assignments = append(assignments, assignment)
		}
	}
	return assignments
}
