package limacharlie

import (
	"context"
	"fmt"
	"net/http"
)

// Group represents a LimaCharlie organization group
type Group struct {
	GID    string
	client *Client
}

// GroupListItem contains brief group info for listing
type GroupListItem struct {
	GID  string `json:"gid,omitempty"`
	Name string `json:"name,omitempty"`
}

// GroupOrg contains org info within a group
type GroupOrg struct {
	OrgName string `json:"org_name,omitempty"`
	OrgID   string `json:"org_id,omitempty"`
}

// GroupInfo contains full group information
type GroupInfo struct {
	GroupID     string     `json:"group_id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Owners      []string   `json:"owners,omitempty"`
	Members     []string   `json:"members,omitempty"`
	Orgs        []GroupOrg `json:"orgs,omitempty"`
	Permissions []string   `json:"perms,omitempty"`
}

// GroupCreateResponse contains response from creating a group
type GroupCreateResponse struct {
	Data    GroupCreateData `json:"data,omitempty"`
	Success bool            `json:"success,omitempty"`
}

// GroupCreateData contains the data portion of the create group response
type GroupCreateData struct {
	GID string `json:"gid,omitempty"`
}

// GetGroups retrieves the list of groups accessible to the current user
// Note: This requires user-level authentication
func (c *Client) GetGroups() ([]GroupListItem, error) {
	var response struct {
		Groups []GroupListItem `json:"groups"`
	}

	request := makeDefaultRequest(&response)

	if err := c.reliableRequest(context.Background(), http.MethodGet, "groups", request); err != nil {
		return nil, err
	}

	return response.Groups, nil
}

// GetGroupsConcurrent retrieves all groups with full info concurrently
// Note: This requires user-level authentication
func (c *Client) GetGroupsConcurrent() ([]GroupInfo, error) {
	var response struct {
		Groups []GroupInfo `json:"groups"`
	}

	request := makeDefaultRequest(&response)

	if err := c.reliableRequest(context.Background(), http.MethodGet, "groups/concurrent", request); err != nil {
		return nil, err
	}

	return response.Groups, nil
}

// CreateGroup creates a new group
// Note: This requires user-level authentication
func (c *Client) CreateGroup(name string) (*GroupCreateResponse, error) {
	var response GroupCreateResponse

	data := map[string]any{
		"name": name,
	}

	request := makeDefaultRequest(&response).withFormData(data)

	if err := c.reliableRequest(context.Background(), http.MethodPost, "groups", request); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetGroup returns a Group struct for interacting with a specific group
func (c *Client) GetGroup(gid string) *Group {
	return &Group{
		GID:    gid,
		client: c,
	}
}

// GetInfo retrieves detailed information about the group
// Note: User must be group owner
func (g *Group) GetInfo() (*GroupInfo, error) {
	var response struct {
		Group GroupInfo `json:"group"`
	}
	urlPath := fmt.Sprintf("groups/%s", g.GID)

	request := makeDefaultRequest(&response)

	if err := g.client.reliableRequest(context.Background(), http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	return &response.Group, nil
}

// Delete deletes the group
// Note: User must be group owner
func (g *Group) Delete() error {
	urlPath := fmt.Sprintf("groups/%s", g.GID)

	request := makeDefaultRequest(nil)

	if err := g.client.reliableRequest(context.Background(), http.MethodDelete, urlPath, request); err != nil {
		return err
	}

	return nil
}

// AddMember adds a user as a member of the group
// email: the user's email address
// inviteMissing: if true, send an invite to users who don't have a LimaCharlie account
// Note: User must be group owner
func (g *Group) AddMember(email string, inviteMissing bool) error {
	urlPath := fmt.Sprintf("groups/%s/users", g.GID)

	data := map[string]any{
		"member_email": email,
	}
	if inviteMissing {
		data["invite_missing"] = "true"
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodPost, urlPath, request); err != nil {
		return err
	}

	return nil
}

// RemoveMember removes a user from the group's members
// email: the user's email address
// Note: User must be group owner
func (g *Group) RemoveMember(email string) error {
	urlPath := fmt.Sprintf("groups/%s/users", g.GID)

	data := map[string]any{
		"member_email": email,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodDelete, urlPath, request); err != nil {
		return err
	}

	return nil
}

// AddOwner adds a user as an owner of the group
// email: the user's email address
// inviteMissing: if true, send an invite to users who don't have a LimaCharlie account
// Note: User must be group owner
func (g *Group) AddOwner(email string, inviteMissing bool) error {
	urlPath := fmt.Sprintf("groups/%s/owners", g.GID)

	data := map[string]any{
		"member_email": email,
	}
	if inviteMissing {
		data["invite_missing"] = "true"
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodPost, urlPath, request); err != nil {
		return err
	}

	return nil
}

// RemoveOwner removes a user from the group's owners
// email: the user's email address
// Note: User must be group owner
func (g *Group) RemoveOwner(email string) error {
	urlPath := fmt.Sprintf("groups/%s/owners", g.GID)

	data := map[string]any{
		"member_email": email,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodDelete, urlPath, request); err != nil {
		return err
	}

	return nil
}

// SetPermissions sets the permissions for the group
// perms: slice of permission names
// Note: User must be group owner
func (g *Group) SetPermissions(perms []string) error {
	urlPath := fmt.Sprintf("groups/%s/permissions", g.GID)

	// API expects repeated "perm" parameter, not "perms"
	data := map[string]any{
		"perm": perms,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodPost, urlPath, request); err != nil {
		return err
	}

	return nil
}

// AddOrg adds an organization to the group
// oid: the organization ID to add
// Note: User must be group owner
func (g *Group) AddOrg(oid string) error {
	urlPath := fmt.Sprintf("groups/%s/orgs", g.GID)

	data := map[string]any{
		"oid": oid,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodPost, urlPath, request); err != nil {
		return err
	}

	return nil
}

// RemoveOrg removes an organization from the group
// oid: the organization ID to remove
// Note: User must be group owner
func (g *Group) RemoveOrg(oid string) error {
	urlPath := fmt.Sprintf("groups/%s/orgs", g.GID)

	data := map[string]any{
		"oid": oid,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := g.client.reliableRequest(context.Background(), http.MethodDelete, urlPath, request); err != nil {
		return err
	}

	return nil
}
