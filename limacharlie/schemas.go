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
	if err := o.client.reliableRequest(http.MethodGet, fmt.Sprintf("orgs/%s/schema", o.client.options.OID), makeDefaultRequest(&resp)); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSchemasForPlatform retrieves event type schemas filtered by platform
// platform: platform name (e.g., "windows", "linux", "macos", "chrome")
func (o *Organization) GetSchemasForPlatform(platform string) (*Schemas, error) {
	resp := Schemas{}
	values := url.Values{}
	values.Set("platform", platform)

	request := makeDefaultRequest(&resp).withURLValues(values)

	if err := o.client.reliableRequest(http.MethodGet, fmt.Sprintf("orgs/%s/schema", o.client.options.OID), request); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPlatformNames retrieves the list of platform names available in LimaCharlie
func (o *Organization) GetPlatformNames() ([]string, error) {
	var resp struct {
		Platforms []string `json:"platforms"`
	}

	// This endpoint returns the ontology of platform names
	if err := o.client.reliableRequest(http.MethodGet, fmt.Sprintf("orgs/%s/platforms", o.client.options.OID), makeDefaultRequest(&resp)); err != nil {
		return nil, err
	}
	return resp.Platforms, nil
}

// GetSchema retrieves a specific schema definition based on the provided schema name.
func (o *Organization) GetSchema(name string) (*SchemaResponse, error) {
	resp := SchemaResponse{}
	if err := o.client.reliableRequest(http.MethodGet, fmt.Sprintf("orgs/%s/schema/%s", o.client.options.OID, name), makeDefaultRequest(&resp)); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResetSchemas resets the schema definition for all schemas in the organization.
func (o *Organization) ResetSchemas() (bool, error) {
	resp := map[string]bool{}
	if err := o.client.reliableRequest(http.MethodDelete, fmt.Sprintf("orgs/%s/schema", o.client.options.OID), makeDefaultRequest(&resp)); err != nil {
		return false, err
	}
	if val, ok := resp["success"]; ok {
		return val, nil
	}
	return false, nil
}
