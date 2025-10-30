package limacharlie

import (
	"fmt"
	"net/http"
	"net/url"
)

// UsageStats contains usage statistics for an organization
type UsageStats struct {
	OID                string                 `json:"oid,omitempty"`
	DataRetention      int64                  `json:"data_retention_days,omitempty"`
	TotalSensors       int64                  `json:"total_sensors,omitempty"`
	OnlineSensors      int64                  `json:"online_sensors,omitempty"`
	EventsIngested     int64                  `json:"events_ingested,omitempty"`
	DetectionsGenerated int64                 `json:"detections_generated,omitempty"`
	StorageUsedGB      float64                `json:"storage_used_gb,omitempty"`
	Period             string                 `json:"period,omitempty"`
	Extra              map[string]interface{} `json:"extra,omitempty"`
}

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
	Permissions []string `json:"permissions,omitempty"`
	IsOwner     bool     `json:"is_owner,omitempty"`
	Region      string   `json:"region,omitempty"`
}

// APIKeyInfo contains information about an API key
type APIKeyInfo struct {
	KeyHash     string   `json:"key_hash,omitempty"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	CreatedAt   int64    `json:"created_at,omitempty"`
	CreatedBy   string   `json:"created_by,omitempty"`
	OID         string   `json:"oid,omitempty"`
}

// APIKeyCreate contains the response when creating a new API key
type APIKeyCreate struct {
	Key         string   `json:"key,omitempty"` // Only returned on creation
	KeyHash     string   `json:"key_hash,omitempty"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// MITREReport contains the MITRE ATT&CK coverage report for an organization
type MITREReport struct {
	OID        string                            `json:"oid,omitempty"`
	Techniques map[string]MITRETechniqueCoverage `json:"techniques,omitempty"`
	Tactics    map[string]MITRETacticCoverage    `json:"tactics,omitempty"`
	Coverage   float64                           `json:"coverage_percentage,omitempty"`
	GeneratedAt int64                            `json:"generated_at,omitempty"`
}

// MITRETechniqueCoverage contains coverage information for a MITRE technique
type MITRETechniqueCoverage struct {
	TechniqueID  string   `json:"technique_id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Covered      bool     `json:"covered,omitempty"`
	DetectionRules []string `json:"detection_rules,omitempty"`
	Tactic       string   `json:"tactic,omitempty"`
}

// MITRETacticCoverage contains coverage information for a MITRE tactic
type MITRETacticCoverage struct {
	TacticName       string  `json:"tactic_name,omitempty"`
	TechniquesTotal  int     `json:"techniques_total,omitempty"`
	TechniquesCovered int    `json:"techniques_covered,omitempty"`
	Coverage         float64 `json:"coverage_percentage,omitempty"`
}

// SensorTimeData contains timestamp information for when a sensor has data
type SensorTimeData struct {
	SID        string  `json:"sid,omitempty"`
	Timestamps []int64 `json:"timestamps,omitempty"`
	Start      int64   `json:"start,omitempty"`
	End        int64   `json:"end,omitempty"`
}

// GetUsageStats retrieves usage statistics for the organization
func (org *Organization) GetUsageStats() (*UsageStats, error) {
	var stats UsageStats
	url := fmt.Sprintf("usage/%s", org.GetOID())

	request := makeDefaultRequest(&stats)

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return &stats, nil
}

// GetOrgErrors retrieves error logs for the organization
func (org *Organization) GetOrgErrors() ([]OrgError, error) {
	var response struct {
		Errors []OrgError `json:"errors"`
	}
	url := fmt.Sprintf("orgs/%s/errors", org.GetOID())

	request := makeDefaultRequest(&response)

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return response.Errors, nil
}

// DismissOrgError dismisses a specific error for the organization
func (org *Organization) DismissOrgError(component string) error {
	url := fmt.Sprintf("orgs/%s/errors/%s", org.GetOID(), component)

	request := makeDefaultRequest(nil)

	if err := org.client.reliableRequest(http.MethodDelete, url, request); err != nil {
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
func (org *Organization) ListUserOrgs(offset, limit *int, filter, sortBy, sortOrder *string, withNames bool) ([]UserOrgInfo, error) {
	var response struct {
		Organizations []UserOrgInfo `json:"orgs"`
		Total         int           `json:"total,omitempty"`
	}

	urlPath := "orgs"
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
	if withNames {
		values.Set("with_names", "true")
	}

	request := makeDefaultRequest(&response).withURLValues(values)

	if err := org.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	return response.Organizations, nil
}

// GetAPIKeys retrieves the list of API keys for the organization
func (org *Organization) GetAPIKeys() ([]APIKeyInfo, error) {
	var response struct {
		Keys []APIKeyInfo `json:"keys"`
	}
	url := fmt.Sprintf("orgs/%s/apikeys", org.GetOID())

	request := makeDefaultRequest(&response)

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return response.Keys, nil
}

// CreateAPIKey creates a new API key for the organization
// name: description/name for the API key
// permissions: optional list of permissions for the key
func (org *Organization) CreateAPIKey(name string, permissions []string) (*APIKeyCreate, error) {
	var response APIKeyCreate
	url := fmt.Sprintf("orgs/%s/apikeys", org.GetOID())

	data := map[string]interface{}{
		"description": name,
	}
	if permissions != nil && len(permissions) > 0 {
		data["permissions"] = permissions
	}

	request := makeDefaultRequest(&response).withFormData(data)

	if err := org.client.reliableRequest(http.MethodPost, url, request); err != nil {
		return nil, err
	}

	return &response, nil
}

// DeleteAPIKey deletes an API key
// keyHash: the hash of the API key to delete
func (org *Organization) DeleteAPIKey(keyHash string) error {
	url := fmt.Sprintf("orgs/%s/apikeys/%s", org.GetOID(), keyHash)

	request := makeDefaultRequest(nil)

	if err := org.client.reliableRequest(http.MethodDelete, url, request); err != nil {
		return err
	}

	return nil
}

// GetMITREReport retrieves the MITRE ATT&CK coverage report for the organization
func (org *Organization) GetMITREReport() (*MITREReport, error) {
	var report MITREReport
	url := fmt.Sprintf("mitre/%s", org.GetOID())

	request := makeDefaultRequest(&report)

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return &report, nil
}

// GetTimeWhenSensorHasData retrieves timestamps when a sensor has reported data
// sid: sensor ID
// start: start timestamp (unix seconds)
// end: end timestamp (unix seconds)
// Note: The time range must be less than 30 days
func (org *Organization) GetTimeWhenSensorHasData(sid string, start, end int64) (*SensorTimeData, error) {
	if end-start > 30*24*3600 {
		return nil, fmt.Errorf("time range must be less than 30 days")
	}

	var response SensorTimeData
	urlPath := fmt.Sprintf("insight/%s/timeline/%s", org.GetOID(), sid)

	values := url.Values{}
	values.Set("start", fmt.Sprintf("%d", start))
	values.Set("end", fmt.Sprintf("%d", end))

	request := makeDefaultRequest(&response).withURLValues(values)

	if err := org.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	response.SID = sid
	response.Start = start
	response.End = end

	return &response, nil
}
