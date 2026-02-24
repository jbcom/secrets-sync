package identitycenter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentityCenterClient_EnhancedConfig(t *testing.T) {
	t.Run("permission set discovery enabled", func(t *testing.T) {
		client := &IdentityCenterClient{
			DiscoverPermissionSets: true,
			CacheAssignments:       true,
		}

		assert.True(t, client.DiscoverPermissionSets)
		assert.True(t, client.CacheAssignments)
	})

	t.Run("instance ARN configuration", func(t *testing.T) {
		client := &IdentityCenterClient{
			InstanceARN: "arn:aws:sso:::instance/ssoins-1234567890abcdef",
		}

		assert.Equal(t, "arn:aws:sso:::instance/ssoins-1234567890abcdef", client.InstanceARN)
	})
}

func TestPermissionSet_Structure(t *testing.T) {
	ps := PermissionSet{
		ARN:         "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-1234567890abcdef",
		Name:        "AdministratorAccess",
		Description: "Full administrative access",
	}

	assert.Equal(t, "AdministratorAccess", ps.Name)
	assert.Contains(t, ps.ARN, "permissionSet")
	assert.NotEmpty(t, ps.Description)
}

func TestAccountAssignment_Structure(t *testing.T) {
	assignment := AccountAssignment{
		AccountID:        "123456789012",
		PermissionSetARN: "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-1234567890abcdef",
		PrincipalType:    "GROUP",
	}

	assert.Equal(t, "123456789012", assignment.AccountID)
	assert.Equal(t, "GROUP", assignment.PrincipalType)
	assert.Contains(t, assignment.PermissionSetARN, "permissionSet")
}

func TestIdentityCenterClient_GetPermissionSetByName(t *testing.T) {
	client := &IdentityCenterClient{
		PermissionSets: []PermissionSet{
			{
				ARN:         "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-admin",
				Name:        "AdministratorAccess",
				Description: "Full admin access",
			},
			{
				ARN:         "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-readonly",
				Name:        "ReadOnlyAccess",
				Description: "Read-only access",
			},
		},
	}

	t.Run("find existing permission set", func(t *testing.T) {
		ps := client.GetPermissionSetByName("AdministratorAccess")
		assert.NotNil(t, ps)
		assert.Equal(t, "AdministratorAccess", ps.Name)
		assert.Contains(t, ps.ARN, "ps-admin")
	})

	t.Run("permission set not found", func(t *testing.T) {
		ps := client.GetPermissionSetByName("NonExistent")
		assert.Nil(t, ps)
	})
}

func TestIdentityCenterClient_GetAccountAssignmentsForAccount(t *testing.T) {
	client := &IdentityCenterClient{
		AccountAssignments: []AccountAssignment{
			{
				AccountID:        "123456789012",
				PermissionSetARN: "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-admin",
				PrincipalType:    "GROUP",
				PrincipalID:      "group-admins",
			},
			{
				AccountID:        "123456789012",
				PermissionSetARN: "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-readonly",
				PrincipalType:    "USER",
				PrincipalID:      "user-123",
			},
			{
				AccountID:        "987654321098",
				PermissionSetARN: "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-admin",
				PrincipalType:    "GROUP",
				PrincipalID:      "group-admins",
			},
		},
	}

	t.Run("get assignments for account with multiple assignments", func(t *testing.T) {
		assignments := client.GetAccountAssignmentsForAccount("123456789012")
		assert.Len(t, assignments, 2)

		// Check that both assignments are for the correct account
		for _, assignment := range assignments {
			assert.Equal(t, "123456789012", assignment.AccountID)
		}

		// Check that we have both permission sets
		psArns := make([]string, len(assignments))
		for i, assignment := range assignments {
			psArns[i] = assignment.PermissionSetARN
		}
		assert.Contains(t, psArns, "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-admin")
		assert.Contains(t, psArns, "arn:aws:sso:::permissionSet/ssoins-1234567890abcdef/ps-readonly")
	})

	t.Run("get assignments for account with single assignment", func(t *testing.T) {
		assignments := client.GetAccountAssignmentsForAccount("987654321098")
		assert.Len(t, assignments, 1)
		assert.Equal(t, "987654321098", assignments[0].AccountID)
	})

	t.Run("get assignments for account with no assignments", func(t *testing.T) {
		assignments := client.GetAccountAssignmentsForAccount("111111111111")
		assert.Len(t, assignments, 0)
	})
}

func TestIdentityCenterClient_CacheInitialization(t *testing.T) {
	client := &IdentityCenterClient{}

	// Simulate cache initialization (normally done in Init)
	client.assignmentCache = make(map[string][]AccountAssignment)
	client.permissionSetCache = make(map[string]PermissionSet)

	assert.NotNil(t, client.assignmentCache)
	assert.NotNil(t, client.permissionSetCache)
	assert.Len(t, client.assignmentCache, 0)
	assert.Len(t, client.permissionSetCache, 0)

	// Test cache operations
	testPS := PermissionSet{
		ARN:  "test-arn",
		Name: "TestPS",
	}

	client.permissionSetCache["test-arn"] = testPS

	cached, exists := client.permissionSetCache["test-arn"]
	assert.True(t, exists)
	assert.Equal(t, "TestPS", cached.Name)
}
