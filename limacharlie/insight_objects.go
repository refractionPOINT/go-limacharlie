package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// InsightObjectType represents the type of object to search for in Insight
type InsightObjectType string

// InsightObjectTypes contains all available object types for Insight searches
var InsightObjectTypes = struct {
	Domain      InsightObjectType
	Username    InsightObjectType
	IP          InsightObjectType
	FileHash    InsightObjectType
	FilePath    InsightObjectType
	FileName    InsightObjectType
	ServiceName InsightObjectType
	PackageName InsightObjectType
	Hostname    InsightObjectType
}{
	Domain:      "domain",
	Username:    "user",
	IP:          "ip",
	FileHash:    "file_hash",
	FilePath:    "file_path",
	FileName:    "file_name",
	ServiceName: "service_name",
	PackageName: "package_name",
	Hostname:    "hostname",
}

// IOCSearchParams contains parameters for searching IOCs in Insight
type IOCSearchParams struct {
	SearchTerm    string            // The IOC value to search for (e.g., "svchost.exe" or "%svchost.exe")
	ObjectType    InsightObjectType // The type of object (e.g., file_name, domain, ip)
	CaseSensitive bool              // Whether the search should be case-sensitive (forced to false for locations)
}

// TimeRangeCounts represents counts that can be either a single number (exact match) or a map (wildcards)
type TimeRangeCounts struct {
	rawValue interface{}
}

// UnmarshalJSON handles parsing of either int64 or map[string]int64
func (t *TimeRangeCounts) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int64 first
	var numValue int64
	if err := json.Unmarshal(data, &numValue); err == nil {
		t.rawValue = numValue
		return nil
	}

	// If that fails, try as map
	var mapValue map[string]int64
	if err := json.Unmarshal(data, &mapValue); err == nil {
		t.rawValue = mapValue
		return nil
	}

	return fmt.Errorf("TimeRangeCounts must be either int64 or map[string]int64")
}

// AsNumber returns the value as an int64 if it's an exact match, 0 otherwise
func (t *TimeRangeCounts) AsNumber() int64 {
	if num, ok := t.rawValue.(int64); ok {
		return num
	}
	return 0
}

// AsMap returns the value as a map if it's a wildcard result, nil otherwise
func (t *TimeRangeCounts) AsMap() map[string]int64 {
	if m, ok := t.rawValue.(map[string]int64); ok {
		return m
	}
	return nil
}

// IsWildcard returns true if this contains wildcard results (map), false for exact match (number)
func (t *TimeRangeCounts) IsWildcard() bool {
	_, ok := t.rawValue.(map[string]int64)
	return ok
}

// IOCSummaryResponse represents the response from an IOC summary search
type IOCSummaryResponse struct {
	FromCache   bool              `json:"from_cache"`
	Type        InsightObjectType `json:"type"`
	Name        string            `json:"name"`
	Last1Days   *TimeRangeCounts  `json:"last_1_days"`
	Last7Days   *TimeRangeCounts  `json:"last_7_days"`
	Last30Days  *TimeRangeCounts  `json:"last_30_days"`
	Last365Days *TimeRangeCounts  `json:"last_365_days"`
}

// IOCLocation represents a single location where an IOC was found
type IOCLocation struct {
	SID      string `json:"sid"`
	Hostname string `json:"hostname"`
	FirstTS  int64  `json:"first_ts"`
	LastTS   int64  `json:"last_ts"`
}

// IOCLocationsResponse represents the response from an IOC locations search
type IOCLocationsResponse struct {
	FromCache bool                   `json:"from_cache"`
	Type      InsightObjectType      `json:"type"`
	Name      string                 `json:"name"`
	Locations map[string]IOCLocation `json:"-"` // Dynamic keys (sensor IDs), populated in UnmarshalJSON
}

// Temporary type to bypass custom UnmarshalJSON when unmarshaling known fields
type tempIOCLocationsResponse IOCLocationsResponse

// UnmarshalJSON custom unmarshaling to handle dynamic sensor ID keys
func (r *IOCLocationsResponse) UnmarshalJSON(data []byte) error {
	// First unmarshal into a map to separate known fields from dynamic location keys
	raw := map[string]interface{}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract and store location data (all keys except the known metadata fields)
	locations := make(map[string]IOCLocation)
	for key := range raw {
		// Skip metadata fields - we'll handle them via standard unmarshaling
		if key == "from_cache" || key == "type" || key == "name" {
			continue
		}

		// Each location is a nested map, convert it to IOCLocation
		if locData, ok := raw[key].(map[string]interface{}); ok {
			locBytes, err := json.Marshal(locData)
			if err != nil {
				continue
			}
			var loc IOCLocation
			if err := json.Unmarshal(locBytes, &loc); err == nil {
				locations[key] = loc
			}
		}

		// Remove location keys from the map so they don't interfere with standard unmarshaling
		delete(raw, key)
	}

	// Re-marshal the cleaned map (only known fields remain)
	cleanedData, err := json.Marshal(raw)
	if err != nil {
		return err
	}

	// Unmarshal into temporary type (bypasses this custom unmarshaler) to properly set known fields
	temp := tempIOCLocationsResponse{}
	if err := json.Unmarshal(cleanedData, &temp); err != nil {
		return err
	}

	// Copy the properly unmarshaled known fields
	*r = IOCLocationsResponse(temp)

	// Add the locations we extracted earlier
	r.Locations = locations

	return nil
}

// HostnameSearchResult represents a result from hostname search
type HostnameSearchResult struct {
	SID      string `json:"sid"`
	Hostname string `json:"hostname"`
}

// HostnameSearchResponse represents the response from a hostname search
type HostnameSearchResponse struct {
	Results []HostnameSearchResult `json:"sid"`
}

// SearchIOCSummary searches for an IOC and returns summary statistics
// This matches the web app's fetchIndicatorSummaryResult function
func (org Organization) SearchIOCSummary(params IOCSearchParams) (*IOCSummaryResponse, error) {
	hasWildcards := strings.Contains(params.SearchTerm, "%")

	queryParams := Dict{
		"name":           params.SearchTerm,
		"case_sensitive": params.CaseSensitive,
		"with_wildcards": hasWildcards,
		"info":           "summary",
		"per_object":     hasWildcards, // per_object is true only for wildcard searches
	}

	var resp IOCSummaryResponse
	request := makeDefaultRequest(&resp).withQueryData(queryParams)

	endpoint := fmt.Sprintf("insight/%s/objects/%s", org.client.options.OID, params.ObjectType)
	if err := org.client.reliableRequest(http.MethodGet, endpoint, request); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SearchIOCLocations searches for locations where an IOC was found
// This matches the web app's fetchIndicatorLocationsResult function
func (org Organization) SearchIOCLocations(params IOCSearchParams) (*IOCLocationsResponse, error) {
	hasWildcards := strings.Contains(params.SearchTerm, "%")

	queryParams := Dict{
		"name":           params.SearchTerm,
		"case_sensitive": false, // ALWAYS false for location searches (per web app)
		"with_wildcards": hasWildcards,
		"info":           "locations",
		"per_object":     false, // ALWAYS false for location searches (per web app)
	}

	var resp IOCLocationsResponse
	request := makeDefaultRequest(&resp).withQueryData(queryParams)

	endpoint := fmt.Sprintf("insight/%s/objects/%s", org.client.options.OID, params.ObjectType)
	if err := org.client.reliableRequest(http.MethodGet, endpoint, request); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SearchHostname searches for sensors by hostname
// This matches the web app's fetchHostnameSearchResults function
func (org Organization) SearchHostname(hostname string) ([]HostnameSearchResult, error) {
	queryParams := Dict{
		"hostname": hostname,
	}

	var resp HostnameSearchResponse
	request := makeDefaultRequest(&resp).withQueryData(queryParams)

	endpoint := fmt.Sprintf("hostnames/%s", org.client.options.OID)
	if err := org.client.reliableRequest(http.MethodGet, endpoint, request); err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// Legacy types and methods below - kept for backward compatibility
// These are deprecated and should not be used in new code

type InsightObjectTypeInfoType string

var InsightObjectTypeInfoTypes = struct {
	Summary  InsightObjectTypeInfoType
	Location InsightObjectTypeInfoType
}{
	Summary:  "summary",
	Location: "locations",
}

type InsightObjectsRequest struct {
	IndicatorName   string
	ObjectType      InsightObjectType
	ObjectTypeInfo  InsightObjectTypeInfoType
	IsCaseSensitive bool
	AllowWildcards  bool
	SearchInLogs    bool
}

type InsightObjectsResponse struct {
	ObjectType    InsightObjectType `json:"type"`
	IndicatorName string            `json:"name"`
	FromCache     bool              `json:"from_cache"`
	Last1Day      int64             `json:"last_1_days"`
	Last7Days     int64             `json:"last_7_days"`
	Last30Days    int64             `json:"last_30_days"`
	Last365Days   int64             `json:"last_365_days"`
}

// Deprecated: Use SearchIOCSummary instead
func (org Organization) InsightObjects(insightReq InsightObjectsRequest) (InsightObjectsResponse, error) {
	var resp InsightObjectsResponse
	if err := org.insightObjects(insightReq, false, &resp); err != nil {
		return InsightObjectsResponse{}, err
	}
	return resp, nil
}

type InsightObjectsPerObjectResponse struct {
	ObjectType    InsightObjectType `json:"type"`
	IndicatorName string            `json:"name"`
	FromCache     bool              `json:"from_cache"`
	Last1Day      Dict              `json:"last_1_days"`
	Last7Days     Dict              `json:"last_7_days"`
	Last30Days    Dict              `json:"last_30_days"`
	Last365Days   Dict              `json:"last_365_days"`
}

// Deprecated: Use SearchIOCSummary instead
func (org Organization) InsightObjectsPerObject(insightReq InsightObjectsRequest) (InsightObjectsPerObjectResponse, error) {
	var resp InsightObjectsPerObjectResponse
	if err := org.insightObjects(insightReq, true, &resp); err != nil {
		return InsightObjectsPerObjectResponse{}, err
	}
	return resp, nil
}

type InsightObjectsBatchRequest struct {
	Objects         map[InsightObjectType][]string
	IsCaseSensitive bool
}

type InsightObjectBatchResponse struct {
	FromCache   bool `json:"from_cache"`
	Last1Day    Dict `json:"last_1_days"`
	Last7Days   Dict `json:"last_7_days"`
	Last30Days  Dict `json:"last_30_days"`
	Last365Days Dict `json:"last_365_days"`
}

// Deprecated: Use SearchIOCSummary for individual searches
func (org Organization) InsightObjectsBatch(insightReq InsightObjectsBatchRequest) (InsightObjectBatchResponse, error) {
	req := Dict{
		"objects":        insightReq.Objects,
		"case_sensitive": insightReq.IsCaseSensitive,
	}
	var resp InsightObjectBatchResponse
	request := makeDefaultRequest(&resp).withFormData(req)
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("insight/%s/objects", org.client.options.OID), request); err != nil {
		return InsightObjectBatchResponse{}, err
	}
	return resp, nil
}

// Deprecated: Internal method for legacy API
func (org Organization) insightObjects(insightReq InsightObjectsRequest, perObject bool, resp interface{}) error {
	req := Dict{
		"name":           insightReq.IndicatorName,
		"info":           insightReq.ObjectTypeInfo,
		"case_sensitive": insightReq.IsCaseSensitive,
		"with_wildcards": insightReq.AllowWildcards,
		"per_object":     perObject,
	}

	// NOTE: origin_type removed - web app doesn't send it
	// Keeping this here for reference in case we need to add it back:
	// if insightReq.SearchInLogs {
	// 	req["origin_type"] = "lsid"
	// } else {
	// 	req["origin_type"] = "sid"
	// }

	request := makeDefaultRequest(resp).withQueryData(req)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/objects/%s", org.client.options.OID, insightReq.ObjectType), request); err != nil {
		return err
	}
	return nil
}
