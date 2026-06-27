package limacharlie

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

const casesTestOID = "00000000-0000-0000-0000-000000000001"

// capturedRequest records what the router saw for a single HTTP call.
type capturedRequest struct {
	method string
	path   string
	query  url.Values
	body   []byte
}

// casesRouter is a single CustomHandler registered on the cases base path
// ("/api/v1/") that records every request and replies with a canned body keyed
// by exact "METHOD PATH". Using one handler with exact-match routing avoids the
// prefix-collision + random-map-iteration problem that overlapping
// CustomHandlers suffer from.
type casesRouter struct {
	responses map[string]string // "METHOD /path" -> JSON body
	calls     []capturedRequest
}

func newCasesRouter() *casesRouter {
	return &casesRouter{responses: map[string]string{}}
}

// on registers the canned response body for an exact method+path.
func (cr *casesRouter) on(method, path, body string) *casesRouter {
	cr.responses[method+" "+path] = body
	return cr
}

// install wires the router onto the mock server for both the cases REST base
// path and the extension request path (used by CreateCase).
func (cr *casesRouter) install(ms *MockServer) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		cr.calls = append(cr.calls, capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			body:   b,
		})
		w.Header().Set("Content-Type", "application/json")
		if body, ok := cr.responses[r.Method+" "+r.URL.Path]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte("{}"))
	}
	ms.CustomHandlers["/api/v1/"] = handler
	ms.CustomHandlers["/v1/extension/request/"] = handler
}

// last returns the most recent captured request. Fails the test if none.
func (cr *casesRouter) last(t *testing.T) capturedRequest {
	t.Helper()
	require.NotEmpty(t, cr.calls, "expected at least one captured request")
	return cr.calls[len(cr.calls)-1]
}

// find returns the single captured request matching method+path.
func (cr *casesRouter) find(t *testing.T, method, path string) capturedRequest {
	t.Helper()
	for _, c := range cr.calls {
		if c.method == method && c.path == path {
			return c
		}
	}
	t.Fatalf("no captured request for %s %s; got %v", method, path, cr.calls)
	return capturedRequest{}
}

func newCasesTestOrg(t *testing.T) (*MockServer, *Organization, *casesRouter) {
	t.Helper()
	ms := NewMockServer(casesTestOID)
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	cr := newCasesRouter()
	cr.install(ms)
	return ms, org, cr
}

func TestCasesListCases(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/cases", `{"cases":[],"next_page_token":""}`)

	resp, err := org.Cases().ListCases(CaseListFilters{
		Status:    []string{"new", "in_progress"},
		Severity:  []string{"critical"},
		Assignee:  "alice@example.com",
		Search:    "mimikatz",
		SensorID:  "sensor-1",
		Tag:       []string{"phishing", "urgent"},
		Sort:      "severity",
		Order:     "desc",
		PageSize:  20,
		PageToken: "tok",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Contains(t, resp, "cases")

	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, "/api/v1/cases", call.path)
	require.Equal(t, casesTestOID, call.query.Get("oids"))
	require.Equal(t, "new,in_progress", call.query.Get("status"))
	require.Equal(t, "critical", call.query.Get("severity"))
	require.Equal(t, "alice@example.com", call.query.Get("assignee"))
	require.Equal(t, "mimikatz", call.query.Get("search"))
	require.Equal(t, "sensor-1", call.query.Get("sid"))
	require.Equal(t, "phishing,urgent", call.query.Get("tag"))
	require.Equal(t, "severity", call.query.Get("sort"))
	require.Equal(t, "desc", call.query.Get("order"))
	require.Equal(t, "20", call.query.Get("page_size"))
	require.Equal(t, "tok", call.query.Get("page_token"))
	require.Empty(t, call.body)
}

func TestCasesListCasesNoFilters(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/cases", `{"cases":[]}`)

	_, err := org.Cases().ListCases(CaseListFilters{})
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, casesTestOID, call.query.Get("oids"))
	require.Empty(t, call.query.Get("status"))
	require.Empty(t, call.query.Get("page_size"))
}

func TestCasesGetCase(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/cases/42", `{"case":{"case_number":42}}`)

	resp, err := org.Cases().GetCase(42)
	require.NoError(t, err)
	require.Contains(t, resp, "case")

	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, "/api/v1/cases/42", call.path)
	require.Equal(t, casesTestOID, call.query.Get("oid"))
}

func TestCasesUpdateCase(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPatch, "/api/v1/cases/7", `{"success":true}`)

	_, err := org.Cases().UpdateCase(7, Dict{
		"status":   "in_progress",
		"severity": "high",
		"dropped":  nil, // nil values must be filtered out
	})
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodPatch, call.method)
	require.Equal(t, "/api/v1/cases/7", call.path)
	require.Equal(t, casesTestOID, call.query.Get("oid"))

	var body Dict
	require.NoError(t, json.Unmarshal(call.body, &body))
	require.Equal(t, "in_progress", body["status"])
	require.Equal(t, "high", body["severity"])
	require.NotContains(t, body, "dropped")
}

func TestCasesCreateCase(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	// create_case goes through the extension mechanism, not the cases host.
	cr.on(http.MethodPost, "/v1/extension/request/ext-cases", `{"data":{"case_number":1}}`)

	detection := Dict{"detect_id": "abc", "cat": "test"}
	resp, err := org.Cases().CreateCase(CreateCaseOptions{
		Detection: detection,
		Severity:  "high",
		Summary:   "investigating",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	call := cr.last(t)
	require.Equal(t, http.MethodPost, call.method)
	require.Equal(t, "/v1/extension/request/ext-cases", call.path)

	// The extension request is form-encoded: oid, action, and data (a JSON
	// string holding the create_case payload).
	form, err := url.ParseQuery(string(call.body))
	require.NoError(t, err)
	require.Equal(t, "create_case", form.Get("action"))
	require.Equal(t, casesTestOID, form.Get("oid"))

	var data Dict
	require.NoError(t, json.Unmarshal([]byte(form.Get("data")), &data))
	require.Equal(t, "high", data["severity"])
	require.Equal(t, "investigating", data["summary"])
	// detection must be passed through as an object, not a double-encoded string.
	det, ok := data["detection"].(map[string]interface{})
	require.True(t, ok, "detection should be a nested object")
	require.Equal(t, "abc", det["detect_id"])
}

func TestCasesBulkUpdate(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPost, "/api/v1/cases/bulk-update", `{"updated":3}`)

	_, err := org.Cases().BulkUpdate([]int{1, 2, 3}, Dict{"status": "closed"})
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodPost, call.method)
	require.Equal(t, "/api/v1/cases/bulk-update", call.path)
	require.JSONEq(t,
		`{"oid":"`+casesTestOID+`","case_numbers":[1,2,3],"update":{"status":"closed"}}`,
		string(call.body),
	)
}

func TestCasesMerge(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPost, "/api/v1/cases/merge", `{"success":true}`)

	_, err := org.Cases().Merge(10, []int{11, 12})
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodPost, call.method)
	require.Equal(t, "/api/v1/cases/merge", call.path)
	require.JSONEq(t,
		`{"oid":"`+casesTestOID+`","target_case_number":10,"source_case_numbers":[11,12]}`,
		string(call.body),
	)
}

func TestCasesAddNote(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPost, "/api/v1/cases/5/notes", `{"event_id":"e1"}`)

	isPublic := true
	_, err := org.Cases().AddNote(5, "triage done", AddNoteOptions{
		NoteType: "analysis",
		IsPublic: &isPublic,
	})
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodPost, call.method)
	require.Equal(t, "/api/v1/cases/5/notes", call.path)
	require.Equal(t, casesTestOID, call.query.Get("oid"))

	var body Dict
	require.NoError(t, json.Unmarshal(call.body, &body))
	require.Equal(t, "triage done", body["content"])
	require.Equal(t, "analysis", body["note_type"])
	require.Equal(t, true, body["is_public"])
}

func TestCasesUpdateNoteVisibility(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPatch, "/api/v1/cases/5/notes/eid-1", `{"success":true}`)

	_, err := org.Cases().UpdateNoteVisibility(5, "eid-1", false)
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodPatch, call.method)
	require.Equal(t, "/api/v1/cases/5/notes/eid-1", call.path)
	var body Dict
	require.NoError(t, json.Unmarshal(call.body, &body))
	require.Equal(t, false, body["is_public"])
}

func TestCasesDetections(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/cases/9/detections", `{"detections":[]}`)
	cr.on(http.MethodPost, "/api/v1/cases/9/detections", `{"success":true}`)
	cr.on(http.MethodDelete, "/api/v1/cases/9/detections/d1", `{"success":true}`)

	_, err := org.Cases().ListDetections(9)
	require.NoError(t, err)
	list := cr.find(t, http.MethodGet, "/api/v1/cases/9/detections")
	require.Equal(t, casesTestOID, list.query.Get("oid"))

	_, err = org.Cases().AddDetection(9, Dict{"detect_id": "d1"})
	require.NoError(t, err)
	add := cr.find(t, http.MethodPost, "/api/v1/cases/9/detections")
	var body Dict
	require.NoError(t, json.Unmarshal(add.body, &body))
	det, ok := body["detection"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "d1", det["detect_id"])

	_, err = org.Cases().RemoveDetection(9, "d1")
	require.NoError(t, err)
	rm := cr.find(t, http.MethodDelete, "/api/v1/cases/9/detections/d1")
	require.Equal(t, casesTestOID, rm.query.Get("oid"))
}

func TestCasesEntities(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPost, "/api/v1/cases/3/entities", `{"entity_id":"x"}`)
	cr.on(http.MethodPatch, "/api/v1/cases/3/entities/eid", `{"success":true}`)
	cr.on(http.MethodDelete, "/api/v1/cases/3/entities/eid", `{"success":true}`)

	_, err := org.Cases().AddEntity(3, "ip", "10.0.0.1", EntityOptions{
		Note:    "seen in logs",
		Verdict: "malicious",
	})
	require.NoError(t, err)
	add := cr.find(t, http.MethodPost, "/api/v1/cases/3/entities")
	var body Dict
	require.NoError(t, json.Unmarshal(add.body, &body))
	require.Equal(t, "ip", body["entity_type"])
	require.Equal(t, "10.0.0.1", body["entity_value"])
	require.Equal(t, "seen in logs", body["note"])
	require.Equal(t, "malicious", body["verdict"])

	_, err = org.Cases().UpdateEntity(3, "eid", EntityOptions{Verdict: "benign"})
	require.NoError(t, err)
	cr.find(t, http.MethodPatch, "/api/v1/cases/3/entities/eid")

	_, err = org.Cases().RemoveEntity(3, "eid")
	require.NoError(t, err)
	cr.find(t, http.MethodDelete, "/api/v1/cases/3/entities/eid")
}

func TestCasesSearchEntities(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/entities/search", `{"cases":[]}`)
	_, err := org.Cases().SearchEntities("domain", "evil.com")
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, "/api/v1/entities/search", call.path)
	require.Equal(t, casesTestOID, call.query.Get("oids"))
	require.Equal(t, "domain", call.query.Get("entity_type"))
	require.Equal(t, "evil.com", call.query.Get("entity_value"))
}

func TestCasesTelemetry(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPost, "/api/v1/cases/4/telemetry", `{"telemetry_id":"t"}`)
	cr.on(http.MethodPatch, "/api/v1/cases/4/telemetry/tid", `{"success":true}`)
	cr.on(http.MethodDelete, "/api/v1/cases/4/telemetry/tid", `{"success":true}`)

	_, err := org.Cases().AddTelemetry(4, Dict{"routing": Dict{"sid": "s1"}}, TelemetryOptions{Verdict: "suspicious"})
	require.NoError(t, err)
	add := cr.find(t, http.MethodPost, "/api/v1/cases/4/telemetry")
	var body Dict
	require.NoError(t, json.Unmarshal(add.body, &body))
	require.Contains(t, body, "event")
	require.Equal(t, "suspicious", body["verdict"])

	_, err = org.Cases().UpdateTelemetry(4, "tid", TelemetryOptions{Note: "n"})
	require.NoError(t, err)
	cr.find(t, http.MethodPatch, "/api/v1/cases/4/telemetry/tid")

	_, err = org.Cases().RemoveTelemetry(4, "tid")
	require.NoError(t, err)
	cr.find(t, http.MethodDelete, "/api/v1/cases/4/telemetry/tid")
}

func TestCasesArtifacts(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodPost, "/api/v1/cases/6/artifacts", `{"artifact_id":"a"}`)
	cr.on(http.MethodDelete, "/api/v1/cases/6/artifacts/aid", `{"success":true}`)

	_, err := org.Cases().AddArtifact(6, "/cap.pcap", "sensor-1", ArtifactOptions{
		ArtifactType: "pcap",
		Note:         "capture",
		Verdict:      "suspicious",
	})
	require.NoError(t, err)
	add := cr.find(t, http.MethodPost, "/api/v1/cases/6/artifacts")
	var body Dict
	require.NoError(t, json.Unmarshal(add.body, &body))
	require.Equal(t, "/cap.pcap", body["path"])
	require.Equal(t, "sensor-1", body["source"])
	require.Equal(t, "pcap", body["artifact_type"])

	_, err = org.Cases().RemoveArtifact(6, "aid")
	require.NoError(t, err)
	cr.find(t, http.MethodDelete, "/api/v1/cases/6/artifacts/aid")
}

func TestCasesExportCase(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/cases/8", `{"case":{"case_number":8}}`)
	cr.on(http.MethodGet, "/api/v1/cases/8/detections", `{"detections":[{"detect_id":"d"}]}`)
	cr.on(http.MethodGet, "/api/v1/cases/8/entities", `{"entities":[]}`)
	cr.on(http.MethodGet, "/api/v1/cases/8/telemetry", `{"telemetry":[]}`)
	cr.on(http.MethodGet, "/api/v1/cases/8/artifacts", `{"artifacts":[]}`)

	resp, err := org.Cases().ExportCase(8)
	require.NoError(t, err)
	require.Contains(t, resp, "case")
	require.Contains(t, resp, "detections")
	require.Contains(t, resp, "entities")
	require.Contains(t, resp, "telemetry")
	require.Contains(t, resp, "artifacts")
}

func TestCasesReportSummary(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/reports/summary", `{"mtta":0}`)
	_, err := org.Cases().ReportSummary("2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", "severity")
	require.NoError(t, err)

	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, casesTestOID, call.query.Get("oids"))
	require.Equal(t, "2026-01-01T00:00:00Z", call.query.Get("from"))
	require.Equal(t, "2026-02-01T00:00:00Z", call.query.Get("to"))
	require.Equal(t, "severity", call.query.Get("group_by"))
}

func TestCasesDashboardCounts(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/dashboard/counts", `{"counts":{}}`)
	_, err := org.Cases().DashboardCounts()
	require.NoError(t, err)
	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, "/api/v1/dashboard/counts", call.path)
	require.Equal(t, casesTestOID, call.query.Get("oids"))
}

func TestCasesConfig(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	path := "/api/v1/config/" + casesTestOID
	cr.on(http.MethodGet, path, `{"retention_days":90}`)
	cr.on(http.MethodPut, path, `{"success":true}`)

	_, err := org.Cases().GetConfig()
	require.NoError(t, err)
	get := cr.find(t, http.MethodGet, path)
	require.Equal(t, path, get.path)

	_, err = org.Cases().SetConfig(Dict{"retention_days": 30})
	require.NoError(t, err)
	set := cr.find(t, http.MethodPut, path)
	require.JSONEq(t, `{"retention_days":30}`, string(set.body))
}

func TestCasesAssignees(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/assignees", `{"assignees":[]}`)
	_, err := org.Cases().ListAssignees()
	require.NoError(t, err)
	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, casesTestOID, call.query.Get("oids"))
}

func TestCasesOrgs(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/orgs", `{"orgs":[]}`)
	_, err := org.Cases().ListOrgs()
	require.NoError(t, err)
	call := cr.last(t)
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, "/api/v1/orgs", call.path)
}

func TestCasesListEntitiesAndTelemetry(t *testing.T) {
	ms, org, cr := newCasesTestOrg(t)
	defer ms.Close()

	cr.on(http.MethodGet, "/api/v1/cases/2/entities", `{"entities":[]}`)
	cr.on(http.MethodGet, "/api/v1/cases/2/telemetry", `{"telemetry":[]}`)
	cr.on(http.MethodGet, "/api/v1/cases/2/artifacts", `{"artifacts":[]}`)

	_, err := org.Cases().ListEntities(2)
	require.NoError(t, err)
	e := cr.find(t, http.MethodGet, "/api/v1/cases/2/entities")
	require.Equal(t, casesTestOID, e.query.Get("oid"))

	_, err = org.Cases().ListTelemetry(2)
	require.NoError(t, err)
	cr.find(t, http.MethodGet, "/api/v1/cases/2/telemetry")

	_, err = org.Cases().ListArtifacts(2)
	require.NoError(t, err)
	cr.find(t, http.MethodGet, "/api/v1/cases/2/artifacts")
}
