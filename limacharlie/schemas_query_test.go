package limacharlie

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSchemaQuerySelector(t *testing.T) {
	const name = "det:group/rule+name%?#&"
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodGet || r.URL.Path != "/v1/orgs/"+testOID+"/schema" || r.URL.Query().Get("name") != name {
			t.Errorf("unexpected schema request: %s %s", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"schema": map[string]any{"event_type": name, "elements": []string{"s:event/value"}}})
	}))
	defer server.Close()
	org, err := NewOrganizationFromClientOptions(ClientOptions{OID: testOID, JWT: "test", URL: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := org.GetSchema(name)
	if err != nil {
		t.Fatal(err)
	}
	if !called || response.Schema.EventType != name {
		t.Fatalf("unexpected response: %+v", response)
	}
}
