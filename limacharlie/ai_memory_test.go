package limacharlie

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetAIMemory(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)

	var gotPath, gotData string
	var gotMethod string
	ms.CustomHandlers["/v1/hive/ai_memory/"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = r.ParseForm()
		gotData = r.Form.Get("data")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}

	require.NoError(t, org.SetAIMemory("my-agent", "fact1", "the sky is blue"))
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/v1/hive/ai_memory/"+org.GetOID()+"/my-agent/data", gotPath)

	var payload map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(gotData), &payload))
	require.Equal(t, "the sky is blue", payload["memories"]["fact1"])
}

func TestDeleteAIMemory(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)

	var gotData string
	ms.CustomHandlers["/v1/hive/ai_memory/"] = func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotData = r.Form.Get("data")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}

	require.NoError(t, org.DeleteAIMemory("my-agent", "fact1"))
	// A delete sends a JSON null value for the entry.
	var raw map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(gotData), &raw))
	require.Equal(t, "null", string(raw["memories"]["fact1"]))
}

func TestAIMemoryAgentEscaped(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)

	var gotPath string
	ms.CustomHandlers["/v1/hive/ai_memory/"] = func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath preserves the on-the-wire percent-encoding (r.URL.Path is decoded).
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}
	require.NoError(t, org.SetAIMemory("agent with space", "k", "v"))
	require.Contains(t, gotPath, url.PathEscape("agent with space"))
}

func TestSensorSealUnseal(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	sid := "11111111-1111-1111-1111-111111111111"
	ms.SensorStore[sid] = &Sensor{OID: ms.OID, SID: sid}

	var sealMethod, unsealMethod string
	ms.CustomHandlers["/v1/"+sid+"/seal"] = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sealMethod = r.Method
		} else if r.Method == http.MethodDelete {
			unsealMethod = r.Method
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}

	sensor := org.GetSensor(sid)
	require.NoError(t, sensor.Seal())
	require.Equal(t, http.MethodPost, sealMethod)
	require.NoError(t, sensor.Unseal())
	require.Equal(t, http.MethodDelete, unsealMethod)
}
