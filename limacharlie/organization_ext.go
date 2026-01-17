package limacharlie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// UsageStats is deprecated - GetUsageStats now returns Dict directly
// This struct never matched the actual API response format
// Kept for backwards compatibility with any code that may reference it
type UsageStats = Dict

// OrgError represents an error log entry for an organization
type OrgError struct {
	Component string                 `json:"component,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp int64                  `json:"ts,omitempty"`
	OID       string                 `json:"oid,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// UserOrgInfo contains information about an organization accessible to a user
type UserOrgInfo struct {
	OID         string   `json:"oid,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	IsOwner     bool     `json:"is_owner,omitempty"`
	Region      string   `json:"region,omitempty"`
}

// APIKeyInfo contains information about an API key
type APIKeyInfo struct {
	KeyHash     string   `json:"key_hash,omitempty"`
	Description string   `json:"name,omitempty"` // API returns "name" from Firebase
	Permissions []string `json:"priv,omitempty"` // API returns "priv"
	CreatedAt   int64    `json:"created_at,omitempty"`
	CreatedBy   string   `json:"created_by,omitempty"`
	OID         string   `json:"oid,omitempty"`
}

// APIKeyCreate contains the response when creating a new API key
type APIKeyCreate struct {
	Key     string `json:"api_key,omitempty"` // Only returned on creation
	KeyHash string `json:"key_hash,omitempty"`
}

// MITREReport contains the MITRE ATT&CK coverage report for an organization
// This format is compatible with the MITRE ATT&CK Navigator
type MITREReport struct {
	Name        string                   `json:"name,omitempty"`
	Versions    MITREVersion             `json:"versions,omitempty"`
	Sorting     int                      `json:"sorting,omitempty"`
	Description string                   `json:"description,omitempty"`
	Domain      string                   `json:"domain,omitempty"`
	Techniques  []MITRETechniqueCoverage `json:"techniques,omitempty"`
}

// MITRETechniqueCoverage contains coverage information for a MITRE technique
type MITRETechniqueCoverage struct {
	TechniqueID string `json:"techniqueID,omitempty"`
	Enabled     bool   `json:"enabled,omitempty"`
	Color       string `json:"color,omitempty"`
}

// MITREVersion contains version information for the MITRE layer format
type MITREVersion struct {
	Layer     string `json:"layer,omitempty"`
	Navigator string `json:"navigator,omitempty"`
}

// SensorTimeData contains timestamp information for when a sensor has data
type SensorTimeData struct {
	SID        string  `json:"sid,omitempty"`
	Timestamps []int64 `json:"overview,omitempty"` // API returns "overview" not "timestamps"
	Start      int64   `json:"start,omitempty"`
	End        int64   `json:"end,omitempty"`
}

// GetUsageStats retrieves usage statistics for the organization
// Returns raw API response as Dict to handle dynamic response formats
func (org *Organization) GetUsageStats() (Dict, error) {
	var stats Dict
	url := fmt.Sprintf("usage/%s", org.GetOID())

	request := makeDefaultRequest(&stats)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return stats, nil
}

// GetOrgErrors retrieves error logs for the organization
func (org *Organization) GetOrgErrors() ([]OrgError, error) {
	var response struct {
		Errors []OrgError `json:"errors"`
	}
	url := fmt.Sprintf("errors/%s", org.GetOID())

	request := makeDefaultRequest(&response)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return response.Errors, nil
}

// DismissOrgError dismisses a specific error for the organization
func (org *Organization) DismissOrgError(component string) error {
	url := fmt.Sprintf("errors/%s/%s", org.GetOID(), url.PathEscape(component))

	request := makeDefaultRequest(nil)

	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, url, request); err != nil {
		return err
	}

	return nil
}

// ListUserOrgs retrieves the list of organizations accessible to the current user
// offset: starting index for pagination
// limit: maximum number of results to return
// filter: optional filter string
// sortBy: optional field to sort by
// sortOrder: optional sort order ("asc" or "desc")
// withNames: whether to include organization names
// Returns nil, nil if the endpoint requires user-based authentication
func (org *Organization) ListUserOrgs(offset, limit *int, filter, sortBy, sortOrder *string, withNames bool) ([]UserOrgInfo, error) {
	var response struct {
		Organizations []UserOrgInfo `json:"orgs"`
		Total         int           `json:"total,omitempty"`
	}

	urlPath := "user/orgs"
	values := url.Values{}

	if offset != nil {
		values.Set("offset", fmt.Sprintf("%d", *offset))
	}
	if limit != nil {
		values.Set("limit", fmt.Sprintf("%d", *limit))
	}
	if filter != nil && *filter != "" {
		values.Set("filter", *filter)
	}
	if sortBy != nil && *sortBy != "" {
		values.Set("sort_by", *sortBy)
	}
	if sortOrder != nil && *sortOrder != "" {
		values.Set("sort_order", *sortOrder)
	}
	// Note: with_names is NOT sent to API - it's used client-side in Python SDK
	// The API always returns full org info, we just don't need to filter it

	request := makeDefaultRequest(&response).withQueryData(values)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	return response.Organizations, nil
}

// GetAPIKeys retrieves the list of API keys for the organization
func (org *Organization) GetAPIKeys() ([]APIKeyInfo, error) {
	var response struct {
		APIKeys map[string]APIKeyInfo `json:"api_keys"`
	}
	url := fmt.Sprintf("orgs/%s/keys", org.GetOID())

	request := makeDefaultRequest(&response)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, url, request); err != nil {
		return nil, err
	}

	// Convert map to slice and populate KeyHash from map keys
	keys := make([]APIKeyInfo, 0, len(response.APIKeys))
	for hash, keyInfo := range response.APIKeys {
		keyInfo.KeyHash = hash
		keys = append(keys, keyInfo)
	}

	return keys, nil
}

// CreateAPIKey creates a new API key for the organization
// name: description/name for the API key
// permissions: optional list of permissions for the key
func (org *Organization) CreateAPIKey(name string, permissions []string) (*APIKeyCreate, error) {
	var response APIKeyCreate
	url := fmt.Sprintf("orgs/%s/keys", org.GetOID())

	data := map[string]interface{}{
		"key_name": name,
	}
	if permissions != nil && len(permissions) > 0 {
		// API expects comma-separated string
		data["perms"] = strings.Join(permissions, ",")
	}

	request := makeDefaultRequest(&response).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodPost, url, request); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteAPIKey deletes an API key
// keyHash: the hash of the API key to delete
func (org *Organization) DeleteAPIKey(keyHash string) error {
	url := fmt.Sprintf("orgs/%s/keys", org.GetOID())

	data := map[string]interface{}{
		"key_hash": keyHash,
	}

	request := makeDefaultRequest(nil).withFormData(data)

	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, url, request); err != nil {
		return err
	}

	return nil
}

// GetMITREReport retrieves the MITRE ATT&CK coverage report for the organization
func (org *Organization) GetMITREReport() (*MITREReport, error) {
	var report MITREReport
	url := fmt.Sprintf("mitre/%s", org.GetOID())

	request := makeDefaultRequest(&report)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return &report, nil
}

// GetTimeWhenSensorHasData retrieves timestamps when a sensor has reported data
// sid: sensor ID
// start: start timestamp (unix seconds)
// end: end timestamp (unix seconds)
// Note: The time range must be less than 30 days
// Returns nil, nil if the endpoint is not available or requires different authentication
func (org *Organization) GetTimeWhenSensorHasData(sid string, start, end int64) (*SensorTimeData, error) {
	if end-start > 30*24*3600 {
		return nil, fmt.Errorf("time range must be less than 30 days")
	}

	var response SensorTimeData
	urlPath := fmt.Sprintf("insight/%s/%s/overview", org.GetOID(), sid)

	values := url.Values{}
	values.Set("start", fmt.Sprintf("%d", start))
	values.Set("end", fmt.Sprintf("%d", end))

	request := makeDefaultRequest(&response).withQueryData(values)

	if err := org.client.reliableRequest(context.Background(), http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	response.SID = sid
	response.Start = start
	response.End = end

	return &response, nil
}
