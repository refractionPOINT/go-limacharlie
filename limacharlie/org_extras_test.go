package limacharlie

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAuditLogsPaginates(t *testing.T) {
	ms, org := setupMock(t)

	var calls int
	var cursors []string
	var lastStart, lastEnd, lastEventType, lastSID string
	ms.CustomHandlers[fmt.Sprintf("/v1/insight/%s/audit", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		lastStart = r.URL.Query().Get("start")
		lastEnd = r.URL.Query().Get("end")
		lastEventType = r.URL.Query().Get("event_type")
		lastSID = r.URL.Query().Get("sid")
		w.Header().Set("Content-Type", "application/json")
		// First page returns a cursor, second page returns an empty cursor
		// to prove the pagination loop terminates.
		if calls == 1 {
			_, _ = w.Write([]byte(`{"events":[{"event_type":"login"},{"event_type":"logout"}],"next_cursor":"page2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"events":[{"event_type":"create"}],"next_cursor":""}`))
	}

	logs, err := org.GetAuditLogs(GetAuditLogsOptions{
		Start:     100,
		End:       200,
		EventType: "all",
		SID:       "sensor-x",
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, []string{"-", "page2"}, cursors)
	require.Len(t, logs, 3)
	require.Equal(t, "login", logs[0]["event_type"])
	require.Equal(t, "create", logs[2]["event_type"])
	require.Equal(t, "100", lastStart)
	require.Equal(t, "200", lastEnd)
	require.Equal(t, "all", lastEventType)
	require.Equal(t, "sensor-x", lastSID)
}

func TestGetAuditLogsRespectsLimit(t *testing.T) {
	ms, org := setupMock(t)

	var calls int
	ms.CustomHandlers[fmt.Sprintf("/v1/insight/%s/audit", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		// Always returns a non-empty cursor; the Limit must stop iteration.
		_, _ = w.Write([]byte(`{"events":[{"event_type":"a"},{"event_type":"b"}],"next_cursor":"more"}`))
	}

	logs, err := org.GetAuditLogs(GetAuditLogsOptions{Start: 1, End: 2, Limit: 3})
	require.NoError(t, err)
	// Limit=3: first page gives 2, second page gives 2 more but we cap at 3.
	require.Len(t, logs, 3)
	require.Equal(t, 2, calls)
}

func TestGetQuotaUsage(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath string
	ms.CustomHandlers[fmt.Sprintf("/v1/quota_usage/%s", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":12,"quota":100,"breakdown":{"epp":{"n":50,"quota":0}}}`))
	}

	usage, err := org.GetQuotaUsage()
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, fmt.Sprintf("/v1/quota_usage/%s", testOID), gotPath)
	require.EqualValues(t, 12, usage["usage"])
	require.EqualValues(t, 100, usage["quota"])
}

func TestGetGroupLogs(t *testing.T) {
	ms, org := setupMock(t)

	groupID := "group-7"
	var gotMethod, gotPath string
	ms.CustomHandlers[fmt.Sprintf("/v1/groups/%s/logs", groupID)] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"logs":[{"action":"add_member"}]}`))
	}

	logs, err := org.GetGroupLogs(groupID)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, fmt.Sprintf("/v1/groups/%s/logs", groupID), gotPath)
	entries, ok := logs["logs"].([]interface{})
	require.True(t, ok)
	require.Len(t, entries, 1)
}

func TestResolveARL(t *testing.T) {
	ms, org := setupMock(t)

	arl := "[https,storage.googleapis.com/lc-lookups-bucket/tor-ips.json]"
	var gotMethod, gotPath, gotARL string
	ms.CustomHandlers[fmt.Sprintf("/v1/arl/%s", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotARL = r.URL.Query().Get("arl")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"resolved-content"}`))
	}

	resp, err := org.ResolveARL(arl)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, fmt.Sprintf("/v1/arl/%s", testOID), gotPath)
	require.Equal(t, arl, gotARL)
	require.Equal(t, "resolved-content", resp["data"])
}

func TestRenameOrg(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath, gotName string
	ms.CustomHandlers[fmt.Sprintf("/v1/orgs/%s/name", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = r.ParseForm()
		// name is sent as a form/query param; check both for robustness.
		gotName = r.FormValue("name")
		if gotName == "" {
			gotName = r.URL.Query().Get("name")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}

	resp, err := org.RenameOrg("new-org-name")
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, fmt.Sprintf("/v1/orgs/%s/name", testOID), gotPath)
	require.Equal(t, "new-org-name", gotName)
	require.Equal(t, true, resp["success"])
}

func TestExportSensors(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath string
	ms.CustomHandlers[fmt.Sprintf("/v1/export/%s/sensors", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sensors":[{"sid":"s1","hostname":"web-01"}]}`))
	}

	resp, err := org.ExportSensors()
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, fmt.Sprintf("/v1/export/%s/sensors", testOID), gotPath)
	sensors, ok := resp["sensors"].([]interface{})
	require.True(t, ok)
	require.Len(t, sensors, 1)
}

func TestListAvailableExtensions(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/extension/definition"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"extensions":[{"name":"ext-yara"},{"name":"ext-zeek"}]}`))
	}

	resp, err := org.ListAvailableExtensions()
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/extension/definition", gotPath)
	exts, ok := resp["extensions"].([]interface{})
	require.True(t, ok)
	require.Len(t, exts, 2)
}

func TestMassTag(t *testing.T) {
	ms, org := setupMock(t)

	sid1 := "11111111-1111-1111-1111-111111111111"
	sid2 := "22222222-2222-2222-2222-222222222222"
	ms.SensorStore[sid1] = &Sensor{OID: testOID, SID: sid1, Hostname: "web-01", Platform: Platforms.Linux}
	ms.SensorStore[sid2] = &Sensor{OID: testOID, SID: sid2, Hostname: "web-02", Platform: Platforms.Linux}
	ms.SensorOnline[sid1] = true
	ms.SensorOnline[sid2] = true

	result, err := org.MassTag("plat == linux", "investigate", 3600)
	require.NoError(t, err)
	require.Equal(t, "plat == linux", result.Selector)
	require.Equal(t, "investigate", result.Tag)
	require.Equal(t, 2, result.Matched)
	require.Equal(t, 2, result.Succeeded)
	require.Empty(t, result.Errors)

	// The tag should now be present on both sensors in the mock state.
	require.Contains(t, ms.SensorTags[sid1], "investigate")
	require.Contains(t, ms.SensorTags[sid2], "investigate")
}

func TestMassUntag(t *testing.T) {
	ms, org := setupMock(t)

	sid := "33333333-3333-3333-3333-333333333333"
	ms.SensorStore[sid] = &Sensor{OID: testOID, SID: sid, Hostname: "db-01", Platform: Platforms.Windows}
	ms.SensorOnline[sid] = true
	ms.SensorTags[sid] = map[string]TagInfo{
		"investigate": {Tag: "investigate", By: "admin"},
	}

	result, err := org.MassUntag("plat == windows", "investigate")
	require.NoError(t, err)
	require.Equal(t, 1, result.Matched)
	require.Equal(t, 1, result.Succeeded)
	require.Empty(t, result.Errors)
	require.NotContains(t, ms.SensorTags[sid], "investigate")
}
