package limacharlie

import (
	"fmt"
	"net/http"
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
