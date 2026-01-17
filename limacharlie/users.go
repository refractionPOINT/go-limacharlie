package limacharlie

import (
	"context"
	"fmt"
	"net/http"
)

// OrgUserInfo represents detailed information about a direct user in an organization
type OrgUserInfo struct {
	Email       string   `json:"email,omitempty"`
	UID         string   `json:"uid,omitempty"`
	Permissions []string `json:"perms,omitempty"`
	MFAEnabled  bool     `json:"MFA_enabled,omitempty"`
	MFATypes    []string `json:"MFA_types,omitempty"`
}

// OrgGroupUserInfo represents a user who has access via groups
type OrgGroupUserInfo struct {
	Email           string          `json:"email,omitempty"`
	Groups          map[string]bool `json:"groups,omitempty"`
	MFAEnabled      bool            `json:"MFA_enabled,omitempty"`
	MFATypes        []string        `json:"MFA_types,omitempty"`
	HasDirectAccess bool            `json:"hasDirectAccess,omitempty"`
}

// OrgGroupBrief contains brief group info within permissions response
type OrgGroupBrief struct {
	Name        string   `json:"name,omitempty"`
	Permissions []string `json:"perms,omitempty"`
	Owners      []string `json:"owners,omitempty"`
}

// OrgUsersPermissions contains the full permissions response for an organization
type OrgUsersPermissions struct {
	UserPermissions map[string][]string         `json:"user_permissions,omitempty"`
	DirectUsers     []OrgUserInfo               `json:"direct_users,omitempty"`
	FromGroups      map[string]OrgGroupUserInfo `json:"from_groups,omitempty"`
	GroupInfo       map[string]OrgGroupBrief    `json:"group_info,omitempty"`
}

// AddUserResponse contains the response from adding a user
type AddUserResponse struct {
	Success    bool   `json:"success,omitempty"`
	Role       string `json:"role,omitempty"`
	InviteSent bool   `json:"invite_sent,omitempty"`
}

// SetUserRoleResponse contains the response from setting user role
type SetUserRoleResponse struct {
	Success     bool     `json:"success,omitempty"`
	Role        string   `json:"role,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// Valid user roles in LimaCharlie
const (
	UserRoleOwner         = "Owner"
	UserRoleAdministrator = "Administrator"
	UserRoleOperator      = "Operator"
	UserRoleViewer        = "Viewer"
	UserRoleBasic         = "Basic"
)

// GetUsers retrieves the list of user emails with access to this organization
func (org *Organization) GetUsers() ([]string, error) {
	var response struct {
		Users []string `json:"users"`
	}
	urlPath := fmt.Sprintf("orgs/%s/users", org.GetOID())

	request := makeDefaultRequest(&response)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	return response.Users, nil
}

// AddUser adds a user to the organization
// email: the user's email address
// inviteMissing: if true, send an invite to users who don't have a LimaCharlie account
// role: the role to assign (Owner, Administrator, Operator, Viewer, Basic)
func (org *Organization) AddUser(email string, inviteMissing bool, role string) (*AddUserResponse, error) {
	var response AddUserResponse
	urlPath := fmt.Sprintf("orgs/%s/users", org.GetOID())

	data := map[string]interface{}{
		"email": email,
		"role":  role,
	}
	if inviteMissing {
		data["invite_missing"] = "true"
	}

	request := makeDefaultRequest(&response).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodPost, urlPath, request); err != nil {
		return nil, err
	}

	return &response, nil
}

// RemoveUser removes a user from the organization
// email: the user's email address to remove
func (org *Organization) RemoveUser(email string) error {
	urlPath := fmt.Sprintf("orgs/%s/users", org.GetOID())

	data := map[string]interface{}{
		"email": email,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, urlPath, request); err != nil {
		return err
	}

	return nil
}

// GetUsersPermissions retrieves detailed permission information for all users
// This includes direct users, users from groups, and group info
func (org *Organization) GetUsersPermissions() (*OrgUsersPermissions, error) {
	var response OrgUsersPermissions
	urlPath := fmt.Sprintf("orgs/%s/users/permissions", org.GetOID())

	request := makeDefaultRequest(&response)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	return &response, nil
}

// AddUserPermission adds a specific permission to a user
// email: the user's email address
// perm: the permission to add
func (org *Organization) AddUserPermission(email, perm string) error {
	urlPath := fmt.Sprintf("orgs/%s/users/permissions", org.GetOID())

	data := map[string]interface{}{
		"email": email,
		"perm":  perm,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodPost, urlPath, request); err != nil {
		return err
	}

	return nil
}

// RemoveUserPermission removes a specific permission from a user
// email: the user's email address
// perm: the permission to remove
func (org *Organization) RemoveUserPermission(email, perm string) error {
	urlPath := fmt.Sprintf("orgs/%s/users/permissions", org.GetOID())

	data := map[string]interface{}{
		"email": email,
		"perm":  perm,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, urlPath, request); err != nil {
		return err
	}

	return nil
}

// SetUserRole sets the role for a user in the organization
// email: the user's email address
// role: the role to set (Owner, Administrator, Operator, Viewer, Basic)
func (org *Organization) SetUserRole(email, role string) (*SetUserRoleResponse, error) {
	var response SetUserRoleResponse
	urlPath := fmt.Sprintf("orgs/%s/users/role", org.GetOID())

	data := map[string]interface{}{
		"email": email,
		"role":  role,
	}

	request := makeDefaultRequest(&response).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodPut, urlPath, request); err != nil {
		return nil, err
	}

	return &response, nil
}
