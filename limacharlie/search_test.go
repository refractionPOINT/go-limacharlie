package limacharlie

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchMockState is a small helper for building /v1/search handlers that
// need to dispatch on method and path segment.
type searchMockState struct {
	QueryID string
	// HandleInit is called for POST /v1/search. Must write the response.
	HandleInit http.HandlerFunc
	// HandlePoll is called for GET /v1/search/{queryId}. Receives the
	// queryId parsed from the URL.
	HandlePoll func(w http.ResponseWriter, r *http.Request, queryID, token string)
	// HandleCancel is called for DELETE /v1/search/{queryId}.
	HandleCancel func(w http.ResponseWriter, r *http.Request, queryID string)
	// HandleValidate is called for POST /v1/search/validate.
	HandleValidate http.HandlerFunc
}

// installSearchHandler wires a searchMockState into ms.CustomHandlers
// under the /v1/search prefix.
func installSearchHandler(ms *MockServer, st *searchMockState) {
	ms.CustomHandlers["/v1/search"] = func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method

		switch {
		case method == http.MethodPost && path == "/v1/search":
			if st.HandleInit != nil {
				st.HandleInit(w, r)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		case method == http.MethodPost && path == "/v1/search/validate":
			if st.HandleValidate != nil {
				st.HandleValidate(w, r)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasPrefix(path, "/v1/search/"):
			id := strings.TrimPrefix(path, "/v1/search/")
			token := r.URL.Query().Get("token")
			switch method {
			case http.MethodGet:
				if st.HandlePoll != nil {
					st.HandlePoll(w, r, id, token)
					return
				}
			case http.MethodDelete:
				if st.HandleCancel != nil {
					st.HandleCancel(w, r, id)
					return
				}
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// sampleEventRow is a minimal VARIST_SCAN_FILE_REP-shaped row that tests
// round-trip through Dict.UnmarshalJSON without exercising the full
// normalization path (that belongs to the ext-varist consumer).
const sampleEventRow = `{
  "data": {
    "routing": {
      "sid": "sensor-1",
      "event_type": "VARIST_SCAN_FILE_REP"
    },
    "event": { "FILE_NAME": "/usr/bin/ls" }
  }
}`

// ---- InitiateSearch ----

func TestInitiateSearch_HappyPath(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			// Assert the request shape matches what the Python SDK sends.
			assert.Equal(t, testOID, body["oid"])
			assert.Equal(t, "* | * | *", body["query"])
			assert.Equal(t, "100", body["startTime"])
			assert.Equal(t, "200", body["endTime"])
			assert.Equal(t, true, body["paginated"])
			assert.Equal(t, "event", body["stream"])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q1"}`))
		},
	})

	queryID, err := org.InitiateSearch(SearchRequest{
		Query:     "* | * | *",
		StartTime: 100,
		EndTime:   200,
	})
	require.NoError(t, err)
	assert.Equal(t, "q1", queryID)
}

func TestInitiateSearch_NonOKStatus(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad request body", http.StatusBadRequest)
		},
	})

	_, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
}

func TestInitiateSearch_ServerErrorBody(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		},
	})

	_, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestInitiateSearch_MissingQueryID(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		},
	})

	_, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no queryId")
}

func TestInitiateSearch_PaginatedOverride(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			assert.Equal(t, false, body["paginated"])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
	})

	req := SearchRequest{Query: "*", StartTime: 1, EndTime: 2}.WithPaginated(false)
	_, err := org.InitiateSearch(req)
	require.NoError(t, err)
}

// ---- PollSearch ----

func TestPollSearch_ForwardsToken(t *testing.T) {
	ms, org := setupMock(t)
	var capturedID, capturedTok string
	installSearchHandler(ms, &searchMockState{
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			capturedID = queryID
			capturedTok = token
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"completed":true,"results":[]}`))
		},
	})

	poll, err := org.PollSearch("q-abc", "tok-xyz")
	require.NoError(t, err)
	assert.True(t, poll.Completed)
	assert.Equal(t, "q-abc", capturedID)
	assert.Equal(t, "tok-xyz", capturedTok)
}

func TestPollSearch_NonOKStatus(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			http.Error(w, "nope", http.StatusInternalServerError)
		},
	})

	_, err := org.PollSearch("q", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

// ---- ExecuteSearch ----

func TestExecuteSearch_SinglePage(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"completed":true,"results":[{"type":"events","rows":[` + sampleEventRow + `]}]}`))
		},
	})

	pages := 0
	var rows int
	err := org.ExecuteSearch(context.Background(),
		SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: 10 * time.Millisecond},
		func(page *SearchPoll) (bool, error) {
			pages++
			for _, r := range page.Results {
				if r.Type == "events" {
					rows += len(r.Rows)
				}
			}
			return true, nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, pages)
	assert.Equal(t, 1, rows)
}

func TestExecuteSearch_MultiPagePagination(t *testing.T) {
	ms, org := setupMock(t)
	var pollCount int32
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			n := atomic.AddInt32(&pollCount, 1)
			nextToken := "more"
			if n >= 3 {
				nextToken = ""
			}
			resp := fmt.Sprintf(`{"completed":true,"results":[{"type":"events","rows":[%s],"nextToken":%q}]}`, sampleEventRow, nextToken)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
		},
	})

	pages := 0
	err := org.ExecuteSearch(context.Background(),
		SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: 10 * time.Millisecond},
		func(page *SearchPoll) (bool, error) {
			pages++
			return true, nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 3, pages)
}

func TestExecuteSearch_HandlerEarlyExit(t *testing.T) {
	ms, org := setupMock(t)
	var pollCount int32
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			atomic.AddInt32(&pollCount, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"completed":true,"results":[{"type":"events","rows":[],"nextToken":"more"}]}`))
		},
	})

	pages := 0
	err := org.ExecuteSearch(context.Background(),
		SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: 10 * time.Millisecond},
		func(page *SearchPoll) (bool, error) {
			pages++
			return false, nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, pages)
	assert.Equal(t, int32(1), atomic.LoadInt32(&pollCount))
}

func TestExecuteSearch_HandlerError(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"completed":true,"results":[]}`))
		},
	})

	sentinel := fmt.Errorf("stop")
	err := org.ExecuteSearch(context.Background(),
		SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: 10 * time.Millisecond},
		func(page *SearchPoll) (bool, error) { return false, sentinel },
	)
	require.Error(t, err)
	assert.Same(t, sentinel, err)
}

func TestExecuteSearch_PollErrorPropagates(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		},
	})

	err := org.ExecuteSearch(context.Background(),
		SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: 10 * time.Millisecond},
		func(page *SearchPoll) (bool, error) { return true, nil },
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestExecuteSearch_ContextCancelCancelsServer(t *testing.T) {
	ms, org := setupMock(t)
	var cancelCalled int32
	var cancelledID string
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q-cancel"}`))
		},
		HandlePoll: func(w http.ResponseWriter, r *http.Request, queryID, token string) {
			// Never complete — force the caller to sit in the wait loop
			// until ctx is cancelled.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"completed":false,"nextPollInMs":50}`))
		},
		HandleCancel: func(w http.ResponseWriter, r *http.Request, queryID string) {
			atomic.AddInt32(&cancelCalled, 1)
			cancelledID = queryID
			w.WriteHeader(http.StatusOK)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := org.ExecuteSearch(ctx,
		SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: 20 * time.Millisecond, MaxPollAttempts: 100},
		func(page *SearchPoll) (bool, error) { return true, nil },
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Best-effort cancel runs on a detached context, so it may complete
	// slightly after ExecuteSearch returns. Give it a short window.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&cancelCalled) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&cancelCalled))
	assert.Equal(t, "q-cancel", cancelledID)
}

// ---- 401 refresh path ----

func TestDoSearchRequest_401RefreshAndRetry(t *testing.T) {
	ms, org := setupMock(t)

	// Distinguish attempts by which JWT the client presented. First call
	// uses the default "mock-jwt-token"; after /jwt refreshes, the next
	// call uses "mock-jwt-token-refreshed" per the mock JWT handler.
	var attempts int32
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			auth := r.Header.Get("Authorization")
			if n == 1 {
				assert.Contains(t, auth, "mock-jwt-token")
				assert.NotContains(t, auth, "refreshed")
				http.Error(w, "stale", http.StatusUnauthorized)
				return
			}
			assert.Contains(t, auth, "mock-jwt-token-refreshed")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"queryId":"q"}`))
		},
	})

	queryID, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.NoError(t, err)
	assert.Equal(t, "q", queryID)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts), "expected exactly one retry")
}

func TestDoSearchRequest_401TwiceFails(t *testing.T) {
	ms, org := setupMock(t)

	var attempts int32
	installSearchHandler(ms, &searchMockState{
		HandleInit: func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			http.Error(w, "still stale", http.StatusUnauthorized)
		},
	})

	_, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 401")
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts), "no third attempt after second 401")
}

// ---- ValidateSearch ----

func TestValidateSearch_HappyPath(t *testing.T) {
	ms, org := setupMock(t)
	installSearchHandler(ms, &searchMockState{
		HandleValidate: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"estimatedPrice":{"value":0.01,"currency":"USD"}}`))
		},
	})

	v, err := org.ValidateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, "", v.Error)
	assert.NotNil(t, v.EstimatedPrice)
}

// ---- CancelSearch ----

func TestCancelSearch_HappyPath(t *testing.T) {
	ms, org := setupMock(t)
	var deleted string
	installSearchHandler(ms, &searchMockState{
		HandleCancel: func(w http.ResponseWriter, r *http.Request, queryID string) {
			deleted = queryID
			w.WriteHeader(http.StatusOK)
		},
	})

	err := org.CancelSearch("q-42")
	require.NoError(t, err)
	assert.Equal(t, "q-42", deleted)
}
