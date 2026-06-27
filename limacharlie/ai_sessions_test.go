package limacharlie

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// assertOrgIdentityHeaders checks the headers the org-scoped ai-sessions
// endpoints require. The mock client is created with an API key (and no UID),
// so the api-key Authorization override must be present and X-LC-OID set.
func assertOrgIdentityHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	require.Equal(t, testOID, r.Header.Get("X-LC-OID"))
	require.True(t, len(r.Header.Get("Authorization")) > len("Bearer "),
		"expected an Authorization bearer header")
}

func TestAISessions_ListSessions(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath, gotStatus string
	ms.CustomHandlers["/v1/org/sessions"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotStatus = r.URL.Query().Get("status")
		assertOrgIdentityHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessions":[{"id":"sess-1"}],"next_cursor":"abc"}`))
	}

	resp, err := org.AI().ListSessions(context.Background(), &ListSessionsOptions{
		Status: "running",
		Limit:  10,
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/org/sessions", gotPath)
	require.Equal(t, "running", gotStatus)
	require.Equal(t, "abc", resp["next_cursor"])
	sessions, ok := resp["sessions"].([]interface{})
	require.True(t, ok)
	require.Len(t, sessions, 1)
}

func TestAISessions_GetSession(t *testing.T) {
	ms, org := setupMock(t)

	sessionID := "sess-42"
	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/org/sessions/"+sessionID] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		assertOrgIdentityHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session":{"id":"sess-42","status":"running"}}`))
	}

	resp, err := org.AI().GetSession(context.Background(), sessionID)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/org/sessions/"+sessionID, gotPath)
	session, ok := resp["session"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "sess-42", session["id"])
}

func TestAISessions_GetSessionHistory(t *testing.T) {
	ms, org := setupMock(t)

	sessionID := "sess-7"
	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/org/sessions/"+sessionID+"/history"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		assertOrgIdentityHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"type":"user"},{"type":"assistant"}]}`))
	}

	resp, err := org.AI().GetSessionHistory(context.Background(), sessionID)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/org/sessions/"+sessionID+"/history", gotPath)
	messages, ok := resp["messages"].([]interface{})
	require.True(t, ok)
	require.Len(t, messages, 2)
}

func TestAISessions_TerminateSession(t *testing.T) {
	ms, org := setupMock(t)

	sessionID := "sess-kill"
	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/org/sessions/"+sessionID] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		assertOrgIdentityHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"terminated":true}`))
	}

	resp, err := org.AI().TerminateSession(context.Background(), sessionID)
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, gotMethod)
	require.Equal(t, "/v1/org/sessions/"+sessionID, gotPath)
	require.Equal(t, true, resp["terminated"])
}

func TestAISessions_StartSession(t *testing.T) {
	ms, org := setupMock(t)

	// Seed the ai_agent definition and the referenced secret in the hive store
	// so start_session can read the template and resolve the secret reference.
	ms.HiveStore["ai_agent/"+testOID] = map[string]HiveData{
		"my-agent": {
			Data: map[string]interface{}{
				"prompt":           "investigate",
				"name":             "tmpl-name",
				"model":            "claude-sonnet-4-6",
				"max_turns":        float64(5),
				"anthropic_secret": "hive://secret/anthropic",
				"environment": map[string]interface{}{
					"FOO": "bar",
				},
			},
		},
	}
	ms.HiveStore["secret/"+testOID] = map[string]HiveData{
		"anthropic": {
			Data: map[string]interface{}{
				"secret": "sk-resolved-key",
			},
		},
	}

	var gotMethod, gotPath string
	var gotBody Dict
	ms.CustomHandlers["/v1/api/sessions"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		assertOrgIdentityHeaders(t, r)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"new-sess","status":"starting"}`))
	}

	overridePrompt := "investigate this alert"
	resp, err := org.AI().StartSession(context.Background(), "hive://ai_agent/my-agent", &StartSessionOptions{
		Prompt: &overridePrompt,
		Data:   Dict{"alert_id": "abc"},
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/v1/api/sessions", gotPath)
	require.Equal(t, "starting", resp["status"])

	// Body assertions: prompt override applied + yaml data appended, secret
	// resolved, trigger_source set, profile carried over from the template.
	prompt, _ := gotBody["prompt"].(string)
	require.Contains(t, prompt, "investigate this alert")
	require.Contains(t, prompt, "Event data:")
	require.Contains(t, prompt, "alert_id: abc")
	require.Equal(t, "sk-resolved-key", gotBody["anthropic_key"])
	require.Equal(t, "cli", gotBody["trigger_source"])
	require.Equal(t, "tmpl-name", gotBody["name"])

	profile, ok := gotBody["profile"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4-6", profile["model"])
	env, ok := profile["environment"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "bar", env["FOO"])
}

func TestAISessions_ListUsageIdentities(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/org/usage/identities"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		assertOrgIdentityHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identities":["key-a","key-b"]}`))
	}

	resp, err := org.AI().ListUsageIdentities(context.Background())
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/org/usage/identities", gotPath)
	identities, ok := resp["identities"].([]interface{})
	require.True(t, ok)
	require.Len(t, identities, 2)
}

func TestAISessions_GetUsage(t *testing.T) {
	ms, org := setupMock(t)

	identity := "my-api-key"
	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/org/usage/identities/"+identity] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		assertOrgIdentityHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identity":"my-api-key","usage":[{"tokens":100}]}`))
	}

	resp, err := org.AI().GetUsage(context.Background(), identity)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/org/usage/identities/"+identity, gotPath)
	require.Equal(t, "my-api-key", resp["identity"])
}

func TestAISessions_ListUserSessions(t *testing.T) {
	ms, org := setupMock(t)

	var gotMethod, gotPath, gotStatus string
	ms.CustomHandlers["/v1/sessions"] = func(w http.ResponseWriter, r *http.Request) {
		// Guard against also matching /v1/sessions/{id}/... prefix routes.
		if r.URL.Path != "/v1/sessions" {
			http.NotFound(w, r)
			return
		}
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotStatus = r.URL.Query().Get("status")
		// User-scoped routes carry the JWT only; X-LC-OID is not sent.
		require.Empty(t, r.Header.Get("X-LC-OID"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessions":[{"id":"chat-1"}],"next_cursor":""}`))
	}

	resp, err := org.AI().ListUserSessions(context.Background(), &ListSessionsOptions{Status: "ended"})
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/sessions", gotPath)
	require.Equal(t, "ended", gotStatus)
	sessions, ok := resp["sessions"].([]interface{})
	require.True(t, ok)
	require.Len(t, sessions, 1)
}

func TestAISessions_GetUserSession(t *testing.T) {
	ms, org := setupMock(t)

	sessionID := "chat-42"
	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/sessions/"+sessionID] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		require.Empty(t, r.Header.Get("X-LC-OID"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session":{"id":"chat-42"}}`))
	}

	resp, err := org.AI().GetUserSession(context.Background(), sessionID)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/sessions/"+sessionID, gotPath)
	session, ok := resp["session"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "chat-42", session["id"])
}

func TestAISessions_GetUserSessionHistory(t *testing.T) {
	ms, org := setupMock(t)

	sessionID := "chat-7"
	var gotMethod, gotPath string
	ms.CustomHandlers["/v1/sessions/"+sessionID+"/history"] = func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		require.Empty(t, r.Header.Get("X-LC-OID"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"type":"assistant"}]}`))
	}

	resp, err := org.AI().GetUserSessionHistory(context.Background(), sessionID)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/v1/sessions/"+sessionID+"/history", gotPath)
	messages, ok := resp["messages"].([]interface{})
	require.True(t, ok)
	require.Len(t, messages, 1)
}

// TestAISessions_UIDHeader verifies that when a UID is configured on the client
// the X-LC-UID identity header is forwarded on org-scoped requests (mirroring
// ai.py's _org_auth_headers).
func TestAISessions_UIDHeader(t *testing.T) {
	ms, org := setupMock(t)
	org.client.options.UID = "user-123"

	var gotUID string
	ms.CustomHandlers["/v1/org/sessions"] = func(w http.ResponseWriter, r *http.Request) {
		gotUID = r.Header.Get("X-LC-UID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessions":[]}`))
	}

	_, err := org.AI().ListSessions(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, "user-123", gotUID)
}
