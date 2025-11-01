package limacharlie

import (
	"fmt"
	"net/http"
	"net/url"
)

type SchemaDescription = string
type SchemaElement = string

type Schemas struct {
	EventTypes []SchemaDescription `json:"event_types"`
}

type SchemaResponse struct {
	Schema Schema `json:"schema"`
}

type Schema struct {
	Elements  []SchemaElement   `json:"elements"`
	EventType SchemaDescription `json:"event_type"`
}

// GetSchemas retrieves a list of all schemas available for the organization.
func (o *Organization) GetSchemas() (*Schemas, error) {
	resp := Schemas{}
	urlPath := fmt.Sprintf("orgs/%s/schema", o.GetOID())

	request := makeDefaultRequest(&resp)

	if err := o.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSchemasForPlatform retrieves event type schemas filtered by platform
// platform: platform name (e.g., "windows", "linux", "macos", "chrome")
// Returns nil, nil if platform filtering is not supported by the API
func (o *Organization) GetSchemasForPlatform(platform string) (*Schemas, error) {
	resp := Schemas{}
	urlPath := fmt.Sprintf("orgs/%s/schema", o.GetOID())

	values := url.Values{}
	values.Set("platform", platform)

	request := makeDefaultRequest(&resp).withQueryData(values)

	if err := o.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPlatformNames retrieves the list of platform names available in LimaCharlie
// Returns nil, nil if the endpoint is not available
func (o *Organization) GetPlatformNames() ([]string, error) {
	var resp struct {
		Platforms []string `json:"platforms"`
	}
	urlPath := fmt.Sprintf("orgs/%s/platforms", o.GetOID())

	request := makeDefaultRequest(&resp)

	// This endpoint returns the ontology of platform names
	if err := o.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}
	return resp.Platforms, nil
}

// GetSchema retrieves a specific schema definition based on the provided schema name.
func (o *Organization) GetSchema(name string) (*SchemaResponse, error) {
	resp := SchemaResponse{}
	urlPath := fmt.Sprintf("orgs/%s/schema/%s", o.GetOID(), url.PathEscape(name))

	request := makeDefaultRequest(&resp)

	if err := o.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResetSchemas resets the schema definition for all schemas in the organization.
func (o *Organization) ResetSchemas() (bool, error) {
	resp := map[string]bool{}
	urlPath := fmt.Sprintf("orgs/%s/schema", o.GetOID())

	request := makeDefaultRequest(&resp)

	if err := o.client.reliableRequest(http.MethodDelete, urlPath, request); err != nil {
		return false, err
	}
	if val, ok := resp["success"]; ok {
		return val, nil
	}
	return false, nil
}
