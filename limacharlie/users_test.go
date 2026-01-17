package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetUsers tests retrieving the list of users in an organization
func TestGetUsers(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	users, err := org.GetUsers()
	a.NoError(err)
	a.NotNil(users)

	t.Logf("Retrieved %d users from organization", len(users))
	for i, email := range users {
		if i < 5 { // Log first 5 users
			t.Logf("User %d: %s", i+1, email)
		}
	}
}

// TestGetUsersPermissions tests retrieving detailed user permissions
func TestGetUsersPermissions(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	perms, err := org.GetUsersPermissions()
	a.NoError(err)
	a.NotNil(perms)

	t.Logf("User permissions summary:")
	t.Logf("  Direct users: %d", len(perms.DirectUsers))
	t.Logf("  Users from groups: %d", len(perms.FromGroups))
	t.Logf("  Groups: %d", len(perms.GroupInfo))

	// Log some direct user details
	for i, user := range perms.DirectUsers {
		if i < 3 {
			t.Logf("  Direct user %d: %s (MFA: %v, perms: %d)",
				i+1, user.Email, user.MFAEnabled, len(user.Permissions))
		}
	}

	// Log some group info
	for gid, group := range perms.GroupInfo {
		t.Logf("  Group %s: %s (owners: %d, perms: %d)",
			gid, group.Name, len(group.Owners), len(group.Permissions))
		break // Just log first one
	}
}

// TestUserRoleConstants tests that user role constants are defined
func TestUserRoleConstants(t *testing.T) {
	a := assert.New(t)

	a.Equal("Owner", UserRoleOwner)
	a.Equal("Administrator", UserRoleAdministrator)
	a.Equal("Operator", UserRoleOperator)
	a.Equal("Viewer", UserRoleViewer)
	a.Equal("Basic", UserRoleBasic)

	t.Log("User role constants are correctly defined")
}
