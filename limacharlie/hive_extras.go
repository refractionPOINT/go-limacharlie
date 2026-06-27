package limacharlie

import (
	"fmt"
)

// GetHiveSchema fetches the JSON Schema describing the record type of a given
// hive. The returned Dict mirrors the API response, which contains a "schema"
// field holding the JSON Schema document.
//
// This mirrors the Python SDK's Hive.get_schema (sdk/hive.py), which issues a
// GET to hive/{hive}/schema.
func (org *Organization) GetHiveSchema(hiveName string) (Dict, error) {
	resp := Dict{}
	if err := org.GenericGETRequest(fmt.Sprintf("hive/%s/schema", hiveName), Dict{}, &resp); err != nil {
		return nil, fmt.Errorf("failed to get hive schema for %q: %w", hiveName, err)
	}
	return resp, nil
}

// ValidateHiveRecord validates a record against a hive's schema without
// persisting it. The data argument is the record payload that would be stored
// under the record's "data" field. The returned Dict mirrors the API's
// validation result.
//
// This mirrors the Python SDK's Hive.validate (sdk/hive.py), which issues a
// POST to hive/{hive}/{partition}/{key}/validate with the record data sent
// (JSON-encoded) under the "data" form parameter. The partitionKey is the
// hive partition (typically the organization OID).
func (org *Organization) ValidateHiveRecord(hiveName string, partitionKey string, key string, data Dict) (Dict, error) {
	if data == nil {
		data = Dict{}
	}
	req := Dict{
		"data": data,
	}
	resp := Dict{}
	path := fmt.Sprintf("hive/%s/%s/%s/validate", hiveName, partitionKey, key)
	if err := org.GenericPOSTRequest(path, req, &resp); err != nil {
		return nil, fmt.Errorf("failed to validate hive record %q in hive %q: %w", key, hiveName, err)
	}
	return resp, nil
}
