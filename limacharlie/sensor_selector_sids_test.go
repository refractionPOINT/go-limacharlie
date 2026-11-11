package limacharlie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const sidsTestOID = "00000000-0000-4000-8000-000000000001"

// sidsRecorder is an httptest handler for GET sensors/{oid}/sids that records
// every request it receives and answers with a fixed status and body.
type sidsRecorder struct {
	mu       sync.Mutex
	requests []*http.Request
	status   int
	body     string
}

func (s *sidsRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, r.Clone(context.Background()))
	status, body := s.status, s.body
	s.mu.Unlock()
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (s *sidsRecorder) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *sidsRecorder) last(t *testing.T) *http.Request {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	require.NotEmpty(t, s.requests, "no request reached the server")
	return s.requests[len(s.requests)-1]
}

// deadlineRecorder wraps a transport and records the deadline of the context
// each request is sent with, which is how the per-request client timeout of
// reliableRequest is applied.
type deadlineRecorder struct {
	next      http.RoundTripper
	mu        sync.Mutex
	deadlines []time.Duration
}

func (d *deadlineRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	if dl, ok := r.Context().Deadline(); ok {
		d.mu.Lock()
		d.deadlines = append(d.deadlines, time.Until(dl))
		d.mu.Unlock()
	}
	return d.next.RoundTrip(r)
}

// newSIDsTestOrg returns an Organization talking to handler. The client has a
// JWT and no API key, so no JWT exchange happens before the request.
func newSIDsTestOrg(t *testing.T, handler http.Handler) (*Organization, *deadlineRecorder) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	httpClient := srv.Client()
	rec := &deadlineRecorder{next: httpClient.Transport}
	httpClient.Transport = rec
	return &Organization{client: &Client{
		options:    ClientOptions{OID: sidsTestOID, JWT: "test-jwt"},
		logger:     &LCLoggerEmpty{},
		httpClient: httpClient,
		baseURL:    srv.URL,
	}}, rec
}

const okSIDsBody = `{"sids":["a","b"],"matched_count":2,"truncated":false}`

func TestResolveSelectorSIDsRequest(t *testing.T) {
	t.Run("default request carries only the selector", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.NoError(t, err)

		r := rec.last(t)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/sensors/"+sidsTestOID+"/sids", r.URL.Path)
		require.Equal(t, url.Values{"selector": {"plat == windows"}}, r.URL.Query())
		require.Equal(t, "bearer test-jwt", r.Header.Get("Authorization"))
	})

	t.Run("zero options value is the default request", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows", ResolveSelectorSIDsOptions{})
		require.NoError(t, err)
		require.Equal(t, url.Values{"selector": {"plat == windows"}}, rec.last(t).URL.Query())
	})

	t.Run("positive limit is sent", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == linux", ResolveSelectorSIDsOptions{Limit: 150000})
		require.NoError(t, err)
		require.Equal(t, url.Values{"selector": {"plat == linux"}, "limit": {"150000"}}, rec.last(t).URL.Query())
	})

	t.Run("negative limit defers to the server default", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == linux", ResolveSelectorSIDsOptions{Limit: -5})
		require.NoError(t, err)
		require.NotContains(t, rec.last(t).URL.Query(), "limit")
	})

	t.Run("online only is sent as true", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == linux", ResolveSelectorSIDsOptions{OnlineOnly: true})
		require.NoError(t, err)
		require.Equal(t, url.Values{"selector": {"plat == linux"}, "is_online_only": {"true"}}, rec.last(t).URL.Query())
	})

	t.Run("limit and online only compose", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == linux", ResolveSelectorSIDsOptions{Limit: 7, OnlineOnly: true})
		require.NoError(t, err)
		require.Equal(t, url.Values{"selector": {"plat == linux"}, "limit": {"7"}, "is_online_only": {"true"}}, rec.last(t).URL.Query())
	})

	t.Run("selector with reserved characters round trips", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		selector := `"prod&dmz" in tags and hostname contains "a=b#c?d" or plat == 0x10000000/2+1`
		_, err := org.ResolveSelectorSIDs(context.Background(), selector)
		require.NoError(t, err)
		require.Equal(t, selector, rec.last(t).URL.Query().Get("selector"))
	})

	t.Run("client timeout outlasts the default and the gateway", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, deadlines := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == linux")
		require.NoError(t, err)
		require.Len(t, deadlines.deadlines, 1)
		require.Greater(t, deadlines.deadlines[0], 120*time.Second)
		require.LessOrEqual(t, deadlines.deadlines[0], resolveSelectorSIDsTimeout)
	})

	t.Run("caller deadline shorter than the client timeout wins", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, deadlines := newSIDsTestOrg(t, rec)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := org.ResolveSelectorSIDs(ctx, "plat == linux")
		require.NoError(t, err)
		require.Len(t, deadlines.deadlines, 1)
		require.LessOrEqual(t, deadlines.deadlines[0], 5*time.Second)
	})
}

func TestResolveSelectorSIDsResponse(t *testing.T) {
	t.Run("decodes sids, matched count and truncated", func(t *testing.T) {
		rec := &sidsRecorder{body: `{"sids":["0a","1b","2c"],"matched_count":3,"truncated":true}`}
		org, _ := newSIDsTestOrg(t, rec)
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows", ResolveSelectorSIDsOptions{Limit: 3})
		require.NoError(t, err)
		require.Equal(t, SelectorSIDs{SIDs: []string{"0a", "1b", "2c"}, MatchedCount: 3, Truncated: true}, got)
	})

	t.Run("matched count is passed through as sent", func(t *testing.T) {
		// The server sends len(sids). A count that differs from it shows the
		// client decodes the field instead of deriving it from SIDs.
		rec := &sidsRecorder{body: `{"sids":["0a","1b","2c"],"matched_count":7,"truncated":true}`}
		org, _ := newSIDsTestOrg(t, rec)
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows", ResolveSelectorSIDsOptions{Limit: 3})
		require.NoError(t, err)
		require.Equal(t, 7, got.MatchedCount)
	})

	t.Run("complete result is not truncated", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.NoError(t, err)
		require.Equal(t, SelectorSIDs{SIDs: []string{"a", "b"}, MatchedCount: 2, Truncated: false}, got)
	})

	for name, body := range map[string]string{
		"empty array": `{"sids":[],"matched_count":0,"truncated":false}`,
		"null array":  `{"sids":null,"matched_count":0,"truncated":false}`,
		"absent":      `{"matched_count":0,"truncated":false}`,
		"empty body":  ``,
	} {
		t.Run("no match is a non-nil empty slice ("+name+")", func(t *testing.T) {
			rec := &sidsRecorder{body: body}
			org, _ := newSIDsTestOrg(t, rec)
			got, err := org.ResolveSelectorSIDs(context.Background(), "hostname == nothing")
			require.NoError(t, err)
			require.NotNil(t, got.SIDs)
			require.Empty(t, got.SIDs)
			require.Zero(t, got.MatchedCount)
			require.False(t, got.Truncated)
		})
	}

	t.Run("malformed body is an error", func(t *testing.T) {
		rec := &sidsRecorder{body: `{"sids":"not-a-list"}`}
		org, _ := newSIDsTestOrg(t, rec)
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.ErrorContains(t, err, "error parsing response")
		require.False(t, IsResolveSelectorSIDsUnavailable(err))
		require.Equal(t, SelectorSIDs{}, got)
	})
}

func TestResolveSelectorSIDsBlankSelector(t *testing.T) {
	for name, selector := range map[string]string{
		"empty":      "",
		"spaces":     "   ",
		"whitespace": " \t\r\n ",
	} {
		t.Run(name, func(t *testing.T) {
			rec := &sidsRecorder{body: okSIDsBody}
			org, _ := newSIDsTestOrg(t, rec)
			got, err := org.ResolveSelectorSIDs(context.Background(), selector, ResolveSelectorSIDsOptions{Limit: 5})
			require.ErrorIs(t, err, errBlankSelector)
			require.False(t, IsResolveSelectorSIDsUnavailable(err))
			require.Equal(t, SelectorSIDs{}, got)
			require.Zero(t, rec.count(), "a blank selector must not reach the server")
		})
	}
}

func TestResolveSelectorSIDsContext(t *testing.T) {
	t.Run("cancelled context sends nothing", func(t *testing.T) {
		rec := &sidsRecorder{body: okSIDsBody}
		org, _ := newSIDsTestOrg(t, rec)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := org.ResolveSelectorSIDs(ctx, "plat == windows")
		require.ErrorIs(t, err, context.Canceled)
		require.False(t, IsResolveSelectorSIDsUnavailable(err))
		require.Zero(t, rec.count())
	})

	t.Run("cancelling the caller context interrupts an in-flight request", func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		entered := make(chan struct{})
		var once sync.Once
		var mu sync.Mutex
		calls := 0
		org, _ := newSIDsTestOrg(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			calls++
			mu.Unlock()
			once.Do(func() { close(entered) })
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		// The deadline is only a safety net: the context is cancelled once
		// the server holds the request, so the outcome does not depend on
		// how fast the request reaches the handler.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		go func() {
			<-entered
			cancel()
		}()
		_, err := org.ResolveSelectorSIDs(ctx, "plat == windows")
		require.ErrorIs(t, err, context.Canceled)
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, 1, calls, "once the caller context is cancelled no further attempt may reach the server")
	})
}

func TestResolveSelectorSIDsErrors(t *testing.T) {
	t.Run("404 means the route is unavailable and is not retried", func(t *testing.T) {
		rec := &sidsRecorder{status: http.StatusNotFound, body: `{"error":"not found"}`}
		org, _ := newSIDsTestOrg(t, rec)
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.Error(t, err)
		require.True(t, IsResolveSelectorSIDsUnavailable(err))
		var restErr RESTError
		require.ErrorAs(t, err, &restErr)
		require.Equal(t, http.StatusNotFound, restErr.StatusCode())
		require.Equal(t, SelectorSIDs{}, got)
		require.Equal(t, 1, rec.count())
	})

	t.Run("504 is reported, not retried", func(t *testing.T) {
		rec := &sidsRecorder{status: http.StatusGatewayTimeout, body: `upstream timeout`}
		org, _ := newSIDsTestOrg(t, rec)
		_, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.Error(t, err)
		require.False(t, IsResolveSelectorSIDsUnavailable(err))
		var restErr RESTError
		require.ErrorAs(t, err, &restErr)
		require.Equal(t, http.StatusGatewayTimeout, restErr.StatusCode())
		require.Equal(t, 1, rec.count())
	})

	t.Run("500 surfaces as an error", func(t *testing.T) {
		rec := &sidsRecorder{status: http.StatusInternalServerError, body: `{"error":"boom"}`}
		org, _ := newSIDsTestOrg(t, rec)
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.ErrorContains(t, err, "boom")
		require.False(t, IsResolveSelectorSIDsUnavailable(err))
		require.Equal(t, SelectorSIDs{}, got)
	})

	for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusUnauthorized} {
		t.Run(fmt.Sprintf("%d is a failure, not unavailability", status), func(t *testing.T) {
			rec := &sidsRecorder{status: status, body: `{"error":"refused"}`}
			org, _ := newSIDsTestOrg(t, rec)
			_, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
			require.Error(t, err)
			require.False(t, IsResolveSelectorSIDsUnavailable(err))
		})
	}
}

func TestIsResolveSelectorSIDsUnavailable(t *testing.T) {
	notFound := RESTError{s: "404 Not Found", status: http.StatusNotFound}

	t.Run("nil", func(t *testing.T) {
		require.False(t, IsResolveSelectorSIDsUnavailable(nil))
	})
	t.Run("404 rest error", func(t *testing.T) {
		require.True(t, IsResolveSelectorSIDsUnavailable(notFound))
	})
	t.Run("wrapped 404 rest error", func(t *testing.T) {
		require.True(t, IsResolveSelectorSIDsUnavailable(fmt.Errorf("resolving: %w", notFound)))
	})
	t.Run("rest error without a status", func(t *testing.T) {
		require.False(t, IsResolveSelectorSIDsUnavailable(NewRESTError("404 Not Found")))
	})
	t.Run("plain error mentioning 404", func(t *testing.T) {
		require.False(t, IsResolveSelectorSIDsUnavailable(errors.New("api error: 404 Not Found")))
	})
	t.Run("resource not found sentinel", func(t *testing.T) {
		require.False(t, IsResolveSelectorSIDsUnavailable(ErrorResourceNotFound))
	})
}

func TestResolveSelectorSIDsMockServer(t *testing.T) {
	ms, org := setupMock(t)
	for sid, online := range map[string]bool{
		"cccccccc-0000-4000-8000-000000000003": true,
		"aaaaaaaa-0000-4000-8000-000000000001": false,
		"dddddddd-0000-4000-8000-000000000004": true,
		"bbbbbbbb-0000-4000-8000-000000000002": true,
	} {
		ms.SensorStore[sid] = &Sensor{SID: sid, OID: testOID}
		ms.SensorOnline[sid] = online
	}

	t.Run("all sensors sorted", func(t *testing.T) {
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.NoError(t, err)
		require.Equal(t, SelectorSIDs{SIDs: []string{
			"aaaaaaaa-0000-4000-8000-000000000001",
			"bbbbbbbb-0000-4000-8000-000000000002",
			"cccccccc-0000-4000-8000-000000000003",
			"dddddddd-0000-4000-8000-000000000004",
		}, MatchedCount: 4}, got)
	})

	t.Run("online only with a limit keeps the lowest sids", func(t *testing.T) {
		got, err := org.ResolveSelectorSIDs(context.Background(), "plat == windows", ResolveSelectorSIDsOptions{Limit: 2, OnlineOnly: true})
		require.NoError(t, err)
		require.Equal(t, SelectorSIDs{SIDs: []string{
			"bbbbbbbb-0000-4000-8000-000000000002",
			"cccccccc-0000-4000-8000-000000000003",
		}, MatchedCount: 2, Truncated: true}, got)
	})

	t.Run("empty store is a non-nil empty slice", func(t *testing.T) {
		_, org2 := setupMock(t)
		got, err := org2.ResolveSelectorSIDs(context.Background(), "plat == windows")
		require.NoError(t, err)
		require.NotNil(t, got.SIDs)
		require.Empty(t, got.SIDs)
	})
}
