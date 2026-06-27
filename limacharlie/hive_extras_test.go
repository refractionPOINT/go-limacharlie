package limacharlie

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHiveSchema(t *testing.T) {
	ms := NewMockServer("test-oid")
	defer ms.Close()

	org, err := ms.NewOrganization()
	require.NoError(t, err)

	var gotMethod, gotPath string
	// The hive name is part of the path: hive/{hive}/schema -> /v1/hive/{hive}/schema.
	ms.CustomHandlers["/v1/hive/dr-general/schema"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Dict{
			"schema": Dict{
				"type": "object",
				"properties": Dict{
					"detect": Dict{"type": "object"},
				},
			},
		})
	}

	resp, err := org.GetHiveSchema("dr-general")
	require.NoError(t, err)

	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/hive/dr-general/schema", gotPath)

	schema, ok := resp["schema"].(map[string]interface{})
	require.True(t, ok, "expected a schema object in the response, got: %#v", resp)
	require.Equal(t, "object", schema["type"])
}

func TestValidateHiveRecord(t *testing.T) {
	ms := NewMockServer("test-oid")
	defer ms.Close()

	org, err := ms.NewOrganization()
	require.NoError(t, err)

	var gotMethod, gotPath, gotBody string
	path := "/v1/hive/dr-general/test-oid/my-rule/validate"
	ms.CustomHandlers[path] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = readBody(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Dict{"success": true})
	}

	data := Dict{
		"detect":  Dict{"event": "NEW_PROCESS"},
		"respond": []interface{}{Dict{"action": "report", "name": "test"}},
	}
	resp, err := org.ValidateHiveRecord("dr-general", "test-oid", "my-rule", data)
	require.NoError(t, err)

	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, path, gotPath)
	require.Equal(t, true, resp["success"])

	// The record data must be sent (JSON-encoded) under the "data" form field,
	// mirroring the Python SDK's Hive.validate.
	form, err := url.ParseQuery(gotBody)
	require.NoError(t, err)
	rawData := form.Get("data")
	require.NotEmpty(t, rawData, "expected a 'data' form field, body was: %s", gotBody)

	var decoded Dict
	require.NoError(t, json.Unmarshal([]byte(rawData), &decoded))
	detect, ok := decoded["detect"].(map[string]interface{})
	require.True(t, ok, "expected 'detect' object in submitted data, got: %#v", decoded)
	require.Equal(t, "NEW_PROCESS", detect["event"])
}
