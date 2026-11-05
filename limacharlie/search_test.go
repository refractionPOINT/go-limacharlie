package limacharlie

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const searchTestOID = "00000000-0000-0000-0000-000000000043"

// cannedResponse is one reply the search router will hand out. A zero status
// means 200.
type cannedResponse struct {
	status int
	body   string
}

func jsonOK(body string) cannedResponse { return cannedResponse{body: body} }

func jsonStatus(status int, body string) cannedResponse {
	return cannedResponse{status: status, body: body}
}

// searchRouter is a single CustomHandler covering the whole /v1/search prefix.
// It records every request and replies from a per-"METHOD /path" queue, so a
// test can script a sequence of pages without the prefix-collision and random
// map iteration that several overlapping CustomHandlers would suffer from.
//
// A request to a path with nothing registered answers 404 rather than an empty
// 200, so a call that goes somewhere unintended fails loudly instead of
// looking like a search with no results.
type searchRouter struct {
	mu        sync.Mutex
	calls     []capturedRequest
	responses map[string][]cannedResponse
	// onCall, when set, runs for every request after it is recorded and before
	// it is answered. Tests use it to act at a known point in the exchange,
	// such as cancelling a context once a page fetch is in flight, instead of
	// racing a wall-clock deadline.
	onCall func(capturedRequest)
}

func newSearchRouter() *searchRouter {
	return &searchRouter{responses: map[string][]cannedResponse{}}
}

// on queues the replies for an exact method and path. Replies are handed out
// in order; the last one repeats once the queue is down to it.
func (sr *searchRouter) on(method, path string, responses ...cannedResponse) *searchRouter {
	sr.responses[method+" "+path] = append(sr.responses[method+" "+path], responses...)
	return sr
}

func (sr *searchRouter) install(ms *MockServer) {
	ms.CustomHandlers["/v1/search"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		call := capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			body:   body,
		}
		sr.mu.Lock()
		sr.calls = append(sr.calls, call)
		onCall := sr.onCall
		key := r.Method + " " + r.URL.Path
		queued := sr.responses[key]
		var reply cannedResponse
		found := len(queued) > 0
		if found {
			reply = queued[0]
			if len(queued) > 1 {
				sr.responses[key] = queued[1:]
			}
		}
		sr.mu.Unlock()

		if onCall != nil {
			onCall(call)
		}

		w.Header().Set("Content-Type", "application/json")
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"no canned response for ` + key + `"}`))
			return
		}
		if reply.status != 0 {
			w.WriteHeader(reply.status)
		}
		_, _ = w.Write([]byte(reply.body))
	}
}

func (sr *searchRouter) Calls() []capturedRequest {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	out := make([]capturedRequest, len(sr.calls))
	copy(out, sr.calls)
	return out
}

// callsFor returns the recorded calls made with the given HTTP method.
func (sr *searchRouter) callsFor(method string) []capturedRequest {
	out := []capturedRequest{}
	for _, c := range sr.Calls() {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func newSearchTestOrg(t *testing.T) (*MockServer, *Organization, *searchRouter) {
	t.Helper()
	ms := NewMockServer(searchTestOID)
	t.Cleanup(ms.Close)
	sr := newSearchRouter()
	sr.install(ms)
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	return ms, org, sr
}

// bodyKeys returns the sorted top-level key names of a JSON object body.
func bodyKeys(t *testing.T, body []byte) []string {
	t.Helper()
	parsed := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(body, &parsed), "body is not a JSON object: %s", body)
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// fastPolling keeps the tests off the real poll cadence without changing which
// code path runs.
var fastPolling = SearchExecuteOptions{PollInterval: time.Millisecond}

// ---------------------------------------------------------------------------
// Submission body
// ---------------------------------------------------------------------------

// TestSearchSubmitBodyShape pins the exact set of keys a submission carries for
// each combination of the optional fields. The absent cases are the load
// bearing ones: an optional field that leaks into the body as a zero value is
// indistinguishable, to the server, from a caller asking for that zero.
func TestSearchSubmitBodyShape(t *testing.T) {
	base := SearchRequest{Query: "* | NEW_PROCESS | *", StartTime: 1700000000, EndTime: 1700003600}

	t.Run("only the required fields", func(t *testing.T) {
		body, err := buildSearchSubmitBody(searchTestOID, base)
		require.NoError(t, err)
		// mode is here because an unset Mode submits batch; stream is not,
		// because an unset Stream submits nothing.
		require.Equal(t, []string{"endTime", "mode", "oid", "paginated", "query", "startTime"}, bodyKeys(t, body))
	})

	t.Run("stream included when set", func(t *testing.T) {
		req := base
		req.Stream = "detection"
		body, err := buildSearchSubmitBody(searchTestOID, req)
		require.NoError(t, err)
		require.Equal(t, []string{"endTime", "mode", "oid", "paginated", "query", "startTime", "stream"}, bodyKeys(t, body))
	})

	t.Run("mode dropped when explicitly omitted", func(t *testing.T) {
		body, err := buildSearchSubmitBody(searchTestOID, base.WithoutMode())
		require.NoError(t, err)
		require.Equal(t, []string{"endTime", "oid", "paginated", "query", "startTime"}, bodyKeys(t, body))
	})

	t.Run("values", func(t *testing.T) {
		req := base.WithMode(SearchModeInteractive)
		req.Stream = "audit"
		body, err := buildSearchSubmitBody(searchTestOID, req)
		require.NoError(t, err)

		parsed := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(body, &parsed))
		require.Equal(t, searchTestOID, parsed["oid"])
		require.Equal(t, "* | NEW_PROCESS | *", parsed["query"])
		// Times go out as strings, which is what the endpoint accepts.
		require.Equal(t, "1700000000", parsed["startTime"])
		require.Equal(t, "1700003600", parsed["endTime"])
		require.Equal(t, true, parsed["paginated"])
		require.Equal(t, "audit", parsed["stream"])
		require.Equal(t, "interactive", parsed["mode"])
	})
}

// TestSearchSubmitBodyMode covers the tri-state of the Mode field, including a
// value the SDK does not know. An unknown mode must reach the server, which
// treats it as no mode; rejecting it client-side would make this SDK the thing
// that blocks a mode added later.
//
// The two states that look alike and are not: an unset Mode asks for batch,
// while WithoutMode asks for nothing and lets the organization's own default
// decide. Only the second can be satisfied by that default.
func TestSearchSubmitBodyMode(t *testing.T) {
	base := SearchRequest{Query: "*", StartTime: 1, EndTime: 2}

	for name, tc := range map[string]struct {
		req      SearchRequest
		wantJSON string
	}{
		"unset submits batch":         {req: base, wantJSON: "batch"},
		"explicit batch":              {req: base.WithMode(SearchModeBatch), wantJSON: "batch"},
		"explicit interactive":        {req: base.WithMode(SearchModeInteractive), wantJSON: "interactive"},
		"unrecognised is sent":        {req: base.WithMode(SearchMode("something-new")), wantJSON: "something-new"},
		"whitespace is a value":       {req: base.WithMode(SearchMode(" ")), wantJSON: " "},
		"WithoutMode sends no key":    {req: base.WithoutMode(), wantJSON: ""},
		"WithMode empty sends no key": {req: base.WithMode(SearchMode("")), wantJSON: ""},
	} {
		t.Run(name, func(t *testing.T) {
			body, err := buildSearchSubmitBody(searchTestOID, tc.req)
			require.NoError(t, err)

			parsed := map[string]interface{}{}
			require.NoError(t, json.Unmarshal(body, &parsed))
			got, present := parsed["mode"]
			if tc.wantJSON == "" {
				require.False(t, present, "mode must be absent, got body %s", body)
				return
			}
			require.True(t, present, "mode must be present, got body %s", body)
			require.Equal(t, tc.wantJSON, got)
		})
	}

	t.Run("the builders do not mutate the receiver", func(t *testing.T) {
		original := base
		_ = original.WithMode(SearchModeInteractive)
		_ = original.WithoutMode()
		require.Nil(t, original.Mode)
	})
}

// TestSearchSubmitBodyDefaultsToBatch is half of the compatibility pair: a
// caller that sets no mode submits batch, which is the behaviour change this
// SDK makes on their behalf.
//
// Comparing against a literal rather than against another encoder is
// deliberate, so the expectation cannot drift with the code it is checking.
func TestSearchSubmitBodyDefaultsToBatch(t *testing.T) {
	body, err := buildSearchSubmitBody(searchTestOID, SearchRequest{
		Query:     "* | * | *",
		StartTime: 1700000000,
		EndTime:   1700003600,
		Stream:    "event",
	})
	require.NoError(t, err)

	const expected = `{"oid":"` + searchTestOID + `",` +
		`"query":"* | * | *",` +
		`"startTime":"1700000000",` +
		`"endTime":"1700003600",` +
		`"paginated":true,` +
		`"stream":"event",` +
		`"mode":"batch"}`
	require.Equal(t, expected, string(body))
}

// TestSearchSubmitBodyWithoutModeSendsNoKey is the other half: WithoutMode must
// produce the exact bytes an SDK build with no mode field at all would have
// produced, since that is the documented way to restore the previous request.
func TestSearchSubmitBodyWithoutModeSendsNoKey(t *testing.T) {
	body, err := buildSearchSubmitBody(searchTestOID, SearchRequest{
		Query:     "* | * | *",
		StartTime: 1700000000,
		EndTime:   1700003600,
		Stream:    "event",
	}.WithoutMode())
	require.NoError(t, err)

	const expected = `{"oid":"` + searchTestOID + `",` +
		`"query":"* | * | *",` +
		`"startTime":"1700000000",` +
		`"endTime":"1700003600",` +
		`"paginated":true,` +
		`"stream":"event"}`
	require.Equal(t, expected, string(body))
	require.NotContains(t, string(body), "mode")
}

// TestSearchSubmitBodyPaginated covers the tri-state of the Paginated field.
// Unset must mean paginated: an unpaginated search returns its whole result
// set in one response, which is not a safe default to arrive at by leaving a
// field alone.
func TestSearchSubmitBodyPaginated(t *testing.T) {
	base := SearchRequest{Query: "*", StartTime: 1, EndTime: 2}

	for name, tc := range map[string]struct {
		req  SearchRequest
		want bool
	}{
		"unset defaults to paginated": {req: base, want: true},
		"explicit true":               {req: base.WithPaginated(true), want: true},
		"explicit false":              {req: base.WithPaginated(false), want: false},
	} {
		t.Run(name, func(t *testing.T) {
			body, err := buildSearchSubmitBody(searchTestOID, tc.req)
			require.NoError(t, err)
			parsed := map[string]interface{}{}
			require.NoError(t, json.Unmarshal(body, &parsed))
			require.Equal(t, tc.want, parsed["paginated"])
		})
	}

	t.Run("WithPaginated does not mutate the receiver", func(t *testing.T) {
		original := base
		_ = original.WithPaginated(false)
		require.Nil(t, original.Paginated)
	})
}

// TestSearchValidateBodyOmitsDeliveryFields pins that validation sends only the
// criteria. Pagination and mode describe how results are delivered and a
// validation delivers none, so sending them would assert something about a run
// that never happens.
func TestSearchValidateBodyOmitsDeliveryFields(t *testing.T) {
	body, err := buildSearchValidateBody(searchTestOID, SearchRequest{
		Query:     "*",
		StartTime: 1,
		EndTime:   2,
		Stream:    "event",
	}.WithMode(SearchModeBatch).WithPaginated(true))
	require.NoError(t, err)
	require.Equal(t, []string{"endTime", "oid", "query", "startTime", "stream"}, bodyKeys(t, body))
}

// ---------------------------------------------------------------------------
// InitiateSearch
// ---------------------------------------------------------------------------

func TestInitiateSearch(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))

	queryID, err := org.InitiateSearch(SearchRequest{
		Query:     "* | * | *",
		StartTime: 1700000000,
		EndTime:   1700003600,
	}.WithMode(SearchModeInteractive))
	require.NoError(t, err)
	require.Equal(t, "query-1", queryID)

	calls := sr.Calls()
	require.Len(t, calls, 1)
	require.Equal(t, http.MethodPost, calls[0].method)
	require.Equal(t, "/v1/search", calls[0].path)
	require.Empty(t, calls[0].query, "the criteria travel in the body, not the query string")

	parsed := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(calls[0].body, &parsed))
	require.Equal(t, "interactive", parsed["mode"])
	require.Equal(t, searchTestOID, parsed["oid"])
}

func TestInitiateSearchSubmitsBatchWhenModeUnset(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))

	_, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
	require.NoError(t, err)
	require.Contains(t, string(sr.Calls()[0].body), `"mode":"batch"`)
}

func TestInitiateSearchOmitsModeWhenAsked(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))

	_, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2}.WithoutMode())
	require.NoError(t, err)
	require.NotContains(t, string(sr.Calls()[0].body), "mode",
		"WithoutMode must leave the organization's own default in charge")
}

// TestInitiateSearchFailures covers every way a submission can fail to yield a
// usable query id. Each one must be an error: a caller that gets ("", nil) has
// no way to tell "no results" from "nothing ran".
func TestInitiateSearchFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		reply     cannedResponse
		wantInErr string
	}{
		"bad request":              {reply: jsonStatus(http.StatusBadRequest, `{"error":"unparseable query"}`), wantInErr: "unparseable query"},
		"forbidden":                {reply: jsonStatus(http.StatusForbidden, `{"error":"no"}`), wantInErr: "403"},
		"service unavailable":      {reply: jsonStatus(http.StatusServiceUnavailable, `{"error":"try later"}`), wantInErr: "try later"},
		"error reported in a 200":  {reply: jsonOK(`{"error":"query rejected"}`), wantInErr: "query rejected"},
		"no query id":              {reply: jsonOK(`{"completed":false}`), wantInErr: "no query id"},
		"body is not json":         {reply: jsonOK(`not json at all`), wantInErr: "error parsing response"},
		"body is a json fragment":  {reply: jsonOK(`{"queryId":`), wantInErr: "error parsing response"},
		"body is the wrong shape":  {reply: jsonOK(`["queryId"]`), wantInErr: "error parsing response"},
		"query id is not a string": {reply: jsonOK(`{"queryId":42}`), wantInErr: "error parsing response"},
	} {
		t.Run(name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodPost, "/v1/search", tc.reply)

			queryID, err := org.InitiateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantInErr)
			require.Empty(t, queryID)
		})
	}
}

// ---------------------------------------------------------------------------
// PollSearch
// ---------------------------------------------------------------------------

// TestPollSearchRequestShape pins that a continuation is a token-only GET. The
// criteria submitted with the search govern every page, so restating any of
// them here would be a second, competing source of truth.
func TestPollSearchRequestShape(t *testing.T) {
	t.Run("first page carries no token", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[]}`))

		page, err := org.PollSearch("query-1", "")
		require.NoError(t, err)
		require.True(t, page.Completed)

		call := sr.Calls()[0]
		require.Equal(t, http.MethodGet, call.method)
		require.Equal(t, "/v1/search/query-1", call.path)
		require.False(t, call.query.Has("token"))
		require.Empty(t, call.body)
	})

	t.Run("continuation carries the token and nothing else", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[]}`))

		// A token is opaque and the server is free to use characters that
		// need escaping, so a round trip through the query string has to
		// survive them intact.
		const cursor = "cursor/with+odd chars=&?#"
		_, err := org.PollSearch("query-1", cursor)
		require.NoError(t, err)

		call := sr.Calls()[0]
		require.Equal(t, cursor, call.query.Get("token"))
		require.Equal(t, []string{"token"}, func() []string {
			keys := make([]string, 0, len(call.query))
			for k := range call.query {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		}())
		require.Empty(t, call.body)
	})
}

func TestPollSearchRejectsEmptyQueryID(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)

	_, err := org.PollSearch("", "some-token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "query id is required")
	// A missing id must not become a request to the collection path.
	require.Empty(t, sr.Calls())
}

func TestPollSearchFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		reply     cannedResponse
		wantInErr string
	}{
		"not found":        {reply: jsonStatus(http.StatusNotFound, `{"error":"unknown query"}`), wantInErr: "unknown query"},
		"server error":     {reply: jsonStatus(http.StatusInternalServerError, `boom`), wantInErr: "500"},
		"body is not json": {reply: jsonOK(`<html>gateway</html>`), wantInErr: "error parsing response"},
	} {
		t.Run(name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodGet, "/v1/search/query-1", tc.reply)

			page, err := org.PollSearch("query-1", "")
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantInErr)
			require.Nil(t, page)
		})
	}
}

// ---------------------------------------------------------------------------
// Response decoding
// ---------------------------------------------------------------------------

// TestSearchStatsDecoding covers the per-page report of how a search actually
// ran. A mode the server did not recognise or declined is treated as no mode,
// so what the page says it ran as is the only trustworthy answer - not what
// was asked for.
func TestSearchStatsDecoding(t *testing.T) {
	t.Run("reported", func(t *testing.T) {
		var page SearchPoll
		require.NoError(t, json.Unmarshal([]byte(`{
			"queryId":"query-1",
			"completed":true,
			"results":[{"type":"events","rows":[],"stats":{
				"searchMode":"batch",
				"pageSize":1000,
				"paginatedByteCap":1048576,
				"eventsScanned":1234,
				"eventsMatched":7
			}}]
		}`), &page))

		stats := page.Results[0].Stats
		require.Equal(t, SearchModeBatch, stats.SearchMode)
		require.Equal(t, int64(1000), stats.PageSize)
		require.Equal(t, int64(1048576), stats.PaginatedByteCap)
		require.Equal(t, uint64(1234), stats.EventsScanned)
		require.Equal(t, uint64(7), stats.EventsMatched)
	})

	t.Run("omitted for a search that does not paginate", func(t *testing.T) {
		var page SearchPoll
		require.NoError(t, json.Unmarshal([]byte(`{
			"queryId":"query-1",
			"completed":true,
			"results":[{"type":"events","rows":[],"stats":{"eventsScanned":10}}]
		}`), &page))

		stats := page.Results[0].Stats
		require.Equal(t, SearchMode(""), stats.SearchMode)
		require.Zero(t, stats.PageSize)
		require.Zero(t, stats.PaginatedByteCap)
		require.Equal(t, uint64(10), stats.EventsScanned)
	})

	t.Run("no stats block at all", func(t *testing.T) {
		var page SearchPoll
		require.NoError(t, json.Unmarshal([]byte(`{"completed":true,"results":[{"type":"events"}]}`), &page))
		require.Equal(t, SearchStats{}, page.Results[0].Stats)
	})

	t.Run("unknown fields are ignored", func(t *testing.T) {
		var page SearchPoll
		require.NoError(t, json.Unmarshal([]byte(`{
			"completed":true,
			"somethingAddedLater":{"a":1},
			"results":[{"type":"events","stats":{"searchMode":"batch","addedLater":2}}]
		}`), &page))
		require.Equal(t, SearchModeBatch, page.Results[0].Stats.SearchMode)
	})
}

// TestSearchPollNextToken pins where the continuation token is read from. It
// travels on a result item rather than on the response, so the helper is what
// callers use and the position within Results must not matter.
func TestSearchPollNextToken(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"no results":                 {body: `{"completed":true,"results":[]}`, want: ""},
		"single item without token":  {body: `{"completed":true,"results":[{"type":"events"}]}`, want: ""},
		"single item with token":     {body: `{"completed":true,"results":[{"type":"events","nextToken":"t1"}]}`, want: "t1"},
		"token on a trailing item":   {body: `{"completed":true,"results":[{"type":"facets"},{"type":"events","nextToken":"t2"}]}`, want: "t2"},
		"token on a leading item":    {body: `{"completed":true,"results":[{"type":"events","nextToken":"t3"},{"type":"facets"}]}`, want: "t3"},
		"empty token is no token":    {body: `{"completed":true,"results":[{"type":"events","nextToken":""}]}`, want: ""},
		"prev token is not a cursor": {body: `{"completed":true,"results":[{"type":"events","prevToken":"back"}]}`, want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			var page SearchPoll
			require.NoError(t, json.Unmarshal([]byte(tc.body), &page))
			require.Equal(t, tc.want, page.NextToken())
		})
	}

	t.Run("nil page", func(t *testing.T) {
		var page *SearchPoll
		require.Equal(t, "", page.NextToken())
	})
}

// TestSearchPartialDecoding pins that a page known to be missing data says so.
// A partial page is a successful response, so a client that ignores the block
// silently under-reports; an unrecognised reason must still count as an
// omission.
func TestSearchPartialDecoding(t *testing.T) {
	var page SearchPoll
	require.NoError(t, json.Unmarshal([]byte(`{
		"completed":true,
		"results":[],
		"partial":{"reasons":["event_too_large","something_new"],"eventsSkippedTooLarge":3}
	}`), &page))

	require.NotNil(t, page.Partial)
	require.Equal(t, []string{"event_too_large", "something_new"}, page.Partial.Reasons)
	require.Equal(t, uint64(3), page.Partial.EventsSkippedTooLarge)
	require.Zero(t, page.Partial.BatchFetchErrors)

	var complete SearchPoll
	require.NoError(t, json.Unmarshal([]byte(`{"completed":true,"results":[]}`), &complete))
	require.Nil(t, complete.Partial, "a complete page must not look partial")
}

// ---------------------------------------------------------------------------
// FetchSearchPage
// ---------------------------------------------------------------------------

// TestFetchSearchPagePollsUntilComplete covers the wait loop. A page that is
// not ready answers completed=false, which is not an empty page: the same
// request has to be repeated, with the same token, until the server says the
// page is done.
func TestFetchSearchPagePollsUntilComplete(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":false,"nextPollInMs":1}`),
		jsonOK(`{"completed":false,"nextPollInMs":1}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e1"}}],"nextToken":"t2"}]}`),
	)

	page, err := org.FetchSearchPage(context.Background(), "query-1", "t1", fastPolling)
	require.NoError(t, err)
	require.True(t, page.Completed)
	require.Equal(t, "t2", page.NextToken())

	calls := sr.callsFor(http.MethodGet)
	require.Len(t, calls, 3)
	for i, call := range calls {
		require.Equal(t, "t1", call.query.Get("token"), "poll %d must repeat the same token", i)
		require.Empty(t, call.body)
	}
}

func TestFetchSearchPageSurfacesSearchError(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"error":"query exceeded its budget","results":[]}`))

	page, err := org.FetchSearchPage(context.Background(), "query-1", "", fastPolling)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query exceeded its budget")
	require.Nil(t, page, "a failed search must not come back as a page with no rows")
}

// TestFetchSearchPageGivesUp covers the poll bound. It exists so a page the
// server never finishes cannot loop forever, and it has to report that rather
// than return an incomplete page.
func TestFetchSearchPageGivesUp(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":false,"nextPollInMs":1}`))

	page, err := org.FetchSearchPage(context.Background(), "query-1", "", SearchExecuteOptions{
		PollInterval:    time.Millisecond,
		MaxPollAttempts: 3,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "did not produce a page within 3 polls")
	require.Nil(t, page)
	require.Len(t, sr.callsFor(http.MethodGet), 3)
}

// TestFetchSearchPageHonoursContext cancels the caller's context once the
// first poll is in flight, so the outcome does not depend on how fast the
// machine is: the call must stop and report the cancellation rather than poll
// on.
func TestFetchSearchPageHonoursContext(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":false,"nextPollInMs":1}`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.onCall = func(c capturedRequest) {
		if c.method == http.MethodGet {
			cancel()
		}
	}

	page, err := org.FetchSearchPage(ctx, "query-1", "", SearchExecuteOptions{PollInterval: time.Millisecond})
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, page)
	require.Len(t, sr.callsFor(http.MethodGet), 1, "no poll may be sent after the context is cancelled")
}

// ---------------------------------------------------------------------------
// ExecuteSearch
// ---------------------------------------------------------------------------

// TestExecuteSearchPagination is the whole-flow shape: one POST that carries
// the criteria, then GETs that carry only a token. Mode is part of the
// criteria, so it is submitted once and must not appear on any continuation -
// a page cannot change the mode of a search that is already running.
func TestExecuteSearchPagination(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e1"}}],"nextToken":"token-2"}]}`),
		jsonOK(`{"completed":false,"nextPollInMs":1}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e2"}}],"nextToken":"token-3"}]}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e3"}}]}]}`),
	)

	pages := []string{}
	rows := 0
	initiated := []string{}
	completed := []string{}
	opts := fastPolling
	opts.OnQueryInitiated = func(queryID string) { initiated = append(initiated, queryID) }
	opts.OnPageCompleted = func(pageNumber int, nextToken string) {
		completed = append(completed, string(rune('0'+pageNumber))+":"+nextToken)
	}

	err := org.ExecuteSearch(context.Background(), SearchRequest{
		Query:     "* | * | *",
		StartTime: 1700000000,
		EndTime:   1700003600,
	}, opts, func(page *SearchPoll) (bool, error) {
		pages = append(pages, page.NextToken())
		for _, item := range page.Results {
			rows += len(item.Rows)
		}
		return true, nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"token-2", "token-3", ""}, pages)
	require.Equal(t, 3, rows)
	require.Equal(t, []string{"query-1"}, initiated)
	require.Equal(t, []string{"1:token-2", "2:token-3", "3:"}, completed)

	posts := sr.callsFor(http.MethodPost)
	require.Len(t, posts, 1, "the criteria are submitted exactly once")
	submitted := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(posts[0].body, &submitted))
	require.Equal(t, true, submitted["paginated"], "an unset Paginated submits paginated")
	require.Equal(t, "batch", submitted["mode"], "an unset Mode submits batch")

	// The token threading, and the absence of the mode on every continuation.
	gets := sr.callsFor(http.MethodGet)
	require.Len(t, gets, 4)
	wantTokens := []string{"", "token-2", "token-2", "token-3"}
	for i, call := range gets {
		require.Equal(t, "/v1/search/query-1", call.path)
		require.Equal(t, wantTokens[i], call.query.Get("token"), "page fetch %d", i)
		require.Empty(t, call.body, "a continuation carries no body, so it cannot restate the mode")
		require.False(t, call.query.Has("mode"), "page fetch %d must not resend the mode", i)
		require.False(t, call.query.Has("query"))
		require.False(t, call.query.Has("paginated"))
	}
}

// TestExecuteSearchEndsWithoutToken pins the termination condition: a completed
// page with no continuation token ends the search. Nothing else does.
func TestExecuteSearchEndsWithoutToken(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[{"type":"events","rows":[]}]}`))

	pages := 0
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(page *SearchPoll) (bool, error) {
			pages++
			return true, nil
		})
	require.NoError(t, err)
	require.Equal(t, 1, pages)
	require.Len(t, sr.callsFor(http.MethodGet), 1, "no token means no further fetch")
}

// TestExecuteSearchFollowsEmptyPages pins that a page with no rows is not the
// end of a search. A paginated page can legitimately come back empty, for
// example when a stretch of the time range holds no matching events, and only
// the missing token ends the search. Stopping at the first empty page would
// hand a caller a truncated result that looks complete.
func TestExecuteSearchFollowsEmptyPages(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":true,"results":[{"type":"events","rows":[],"nextToken":"token-2"}]}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"token-3"}]}`),
		jsonOK(`{"completed":true,"results":[],"nextToken":"ignored"}`),
	)

	// The third reply carries no result item, so it holds no token: a token on
	// the response itself is not where the server puts it.
	pages := 0
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(page *SearchPoll) (bool, error) {
			pages++
			return true, nil
		})
	require.NoError(t, err)
	require.Equal(t, 3, pages, "every empty page with a token must be followed")

	gets := sr.callsFor(http.MethodGet)
	require.Len(t, gets, 3)
	require.Equal(t, "", gets[0].query.Get("token"))
	require.Equal(t, "token-2", gets[1].query.Get("token"))
	require.Equal(t, "token-3", gets[2].query.Get("token"))
	require.Empty(t, sr.callsFor(http.MethodDelete), "a search read to its end is not cancelled")
}

// TestExecuteSearchSubmitsOverrides pins that the defaults are only defaults:
// a caller's explicit pagination and mode choices reach the wire unchanged
// through the driver, not only through the body builder.
func TestExecuteSearchSubmitsOverrides(t *testing.T) {
	base := SearchRequest{Query: "* | * | *", StartTime: 1700000000, EndTime: 1700003600}

	for name, tc := range map[string]struct {
		req           SearchRequest
		wantPaginated bool
		wantMode      string
	}{
		"defaults":                    {req: base, wantPaginated: true, wantMode: "batch"},
		"unpaginated and interactive": {req: base.WithPaginated(false).WithMode(SearchModeInteractive), wantPaginated: false, wantMode: "interactive"},
		"explicit paginated batch":    {req: base.WithPaginated(true).WithMode(SearchModeBatch), wantPaginated: true, wantMode: "batch"},
		"without mode":                {req: base.WithoutMode(), wantPaginated: true, wantMode: ""},
		"unpaginated without mode":    {req: base.WithPaginated(false).WithoutMode(), wantPaginated: false, wantMode: ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
			// An unpaginated search answers in one page with no token, which
			// is also what ends a paginated one.
			sr.on(http.MethodGet, "/v1/search/query-1",
				jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e1"}}]}]}`))

			err := org.ExecuteSearch(context.Background(), tc.req, fastPolling,
				func(page *SearchPoll) (bool, error) { return true, nil })
			require.NoError(t, err)

			posts := sr.callsFor(http.MethodPost)
			require.Len(t, posts, 1)
			submitted := map[string]interface{}{}
			require.NoError(t, json.Unmarshal(posts[0].body, &submitted))
			require.Equal(t, tc.wantPaginated, submitted["paginated"])
			mode, present := submitted["mode"]
			if tc.wantMode == "" {
				require.False(t, present, "mode must be absent, got body %s", posts[0].body)
			} else {
				require.Equal(t, tc.wantMode, mode)
			}
			require.Len(t, sr.callsFor(http.MethodGet), 1)
		})
	}
}

func TestExecuteSearchHandlerStopsEarly(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"more"}]}`))
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

	pages := 0
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(page *SearchPoll) (bool, error) {
			pages++
			return false, nil
		})
	require.NoError(t, err, "stopping early is not a failure")
	require.Equal(t, 1, pages)
	require.Len(t, sr.callsFor(http.MethodGet), 1, "a token that is not followed must not be fetched")
	require.Len(t, sr.callsFor(http.MethodDelete), 1, "a search left with pages unread must be released")
}

func TestExecuteSearchHandlerErrorAborts(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"more"}]}`))

	sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

	sentinel := errors.New("consumer is gone")
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(page *SearchPoll) (bool, error) { return true, sentinel })
	require.ErrorIs(t, err, sentinel, "the handler's error must reach the caller unchanged")
	require.Len(t, sr.callsFor(http.MethodGet), 1)
	require.Len(t, sr.callsFor(http.MethodDelete), 1, "an aborted search must be released")
}

// TestExecuteSearchReleasesOnlyAnUnfinishedSearch pins when the best-effort
// cancel is sent. Every exit that leaves pages unread sends one, including a
// handler that never returns because it called t.FailNow; a search that ran
// to its last page sends none, because the server has already finished it.
func TestExecuteSearchReleasesOnlyAnUnfinishedSearch(t *testing.T) {
	lastPage := jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e1"}}]}]}`)
	morePages := jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"more"}]}`)

	t.Run("not after the last page", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
		sr.on(http.MethodGet, "/v1/search/query-1", lastPage)

		err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
			func(page *SearchPoll) (bool, error) { return true, nil })
		require.NoError(t, err)
		require.Empty(t, sr.callsFor(http.MethodDelete))
	})

	t.Run("not when the handler fails on the last page", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
		sr.on(http.MethodGet, "/v1/search/query-1", lastPage)

		err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
			func(page *SearchPoll) (bool, error) { return true, errors.New("consumer is gone") })
		require.Error(t, err)
		require.Empty(t, sr.callsFor(http.MethodDelete))
	})

	t.Run("when a page fails", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
		sr.on(http.MethodGet, "/v1/search/query-1", morePages,
			jsonStatus(http.StatusInternalServerError, `{"error":"page failed"}`))
		sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

		err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
			func(page *SearchPoll) (bool, error) { return true, nil })
		require.Error(t, err)
		require.Len(t, sr.callsFor(http.MethodDelete), 1)
	})

	t.Run("when the poll bound is reached", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
		sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":false,"nextPollInMs":1}`))
		sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

		err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
			SearchExecuteOptions{PollInterval: time.Millisecond, MaxPollAttempts: 2},
			func(page *SearchPoll) (bool, error) { return true, nil })
		require.Error(t, err)
		require.Len(t, sr.callsFor(http.MethodDelete), 1)
	})

	t.Run("when the handler exits through Goexit", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
		sr.on(http.MethodGet, "/v1/search/query-1", morePages)
		sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
				func(page *SearchPoll) (bool, error) {
					runtime.Goexit()
					return true, nil
				})
		}()
		<-done
		require.Len(t, sr.callsFor(http.MethodDelete), 1)
	})
}

func TestExecuteSearchRequiresHandler(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)

	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "page handler is required")
	require.Empty(t, sr.Calls(), "nothing may be submitted when there is nowhere to put the results")
}

func TestExecuteSearchSubmissionFailureStopsThere(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonStatus(http.StatusBadRequest, `{"error":"bad query"}`))

	called := false
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(page *SearchPoll) (bool, error) {
			called = true
			return true, nil
		})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad query")
	require.False(t, called)
	require.Empty(t, sr.callsFor(http.MethodGet))
}

func TestExecuteSearchPageFailureSurfaces(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"token-2"}]}`),
		jsonOK(`{"completed":true,"error":"the search could not be completed","results":[]}`),
	)

	pages := 0
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(page *SearchPoll) (bool, error) {
			pages++
			return true, nil
		})
	require.Error(t, err)
	require.Contains(t, err.Error(), "the search could not be completed")
	require.Equal(t, 1, pages, "the pages already delivered stand; the failure stops the rest")
}

// TestExecuteSearchCancelsServerSide covers what happens when the caller walks
// away. A search left running holds a concurrency slot and keeps billing
// events, so the server is told to stop before the call returns.
//
// The context is cancelled from inside the server once a page fetch arrives,
// so the submission has certainly completed and the cancellation certainly
// lands mid-poll, however loaded the machine running the test is.
func TestExecuteSearchCancelsServerSide(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":false,"nextPollInMs":1}`))
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.onCall = func(c capturedRequest) {
		if c.method == http.MethodGet {
			cancel()
		}
	}

	err := org.ExecuteSearch(ctx, SearchRequest{Query: "*", StartTime: 1, EndTime: 2},
		SearchExecuteOptions{PollInterval: time.Millisecond}, func(page *SearchPoll) (bool, error) {
			return true, nil
		})
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, sr.callsFor(http.MethodGet), 1, "no poll may be sent after the context is cancelled")

	// The cancel runs on its own context, synchronously, so it has already
	// happened by the time ExecuteSearch returns.
	deletes := sr.callsFor(http.MethodDelete)
	require.Len(t, deletes, 1)
	require.Equal(t, "/v1/search/query-1", deletes[0].path)
}

// ---------------------------------------------------------------------------
// CancelSearch and ValidateSearch
// ---------------------------------------------------------------------------

// TestSearchQueryIDIsOnePathSegment covers a caller-supplied query id, such as
// one read back from storage. It must address that search and nothing else, so
// a "?" in it cannot start a query string and a "/" cannot add a segment.
func TestSearchQueryIDIsOnePathSegment(t *testing.T) {
	const hostile = "q/../validate?token=injected"

	t.Run("poll", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodGet, "/v1/search/"+hostile, jsonOK(`{"completed":true,"results":[]}`))

		_, err := org.PollSearch(hostile, "")
		require.NoError(t, err)
		call := sr.Calls()[0]
		require.Equal(t, "/v1/search/"+hostile, call.path)
		require.Empty(t, call.query)
	})

	t.Run("cancel", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodDelete, "/v1/search/"+hostile, jsonOK(`{}`))

		require.NoError(t, org.CancelSearch(hostile))
		call := sr.Calls()[0]
		require.Equal(t, "/v1/search/"+hostile, call.path)
		require.Empty(t, call.query)
	})
}

func TestCancelSearch(t *testing.T) {
	t.Run("sends a delete for the query", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

		require.NoError(t, org.CancelSearch("query-1"))
		calls := sr.Calls()
		require.Len(t, calls, 1)
		require.Equal(t, http.MethodDelete, calls[0].method)
		require.Equal(t, "/v1/search/query-1", calls[0].path)
		require.Empty(t, calls[0].body)
	})

	t.Run("rejects an empty query id", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		err := org.CancelSearch("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "query id is required")
		require.Empty(t, sr.Calls(), "a missing id must not become a delete on the collection")
	})

	t.Run("surfaces a refusal", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodDelete, "/v1/search/query-1", jsonStatus(http.StatusForbidden, `{"error":"not yours"}`))
		err := org.CancelSearch("query-1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not yours")
	})
}

func TestValidateSearch(t *testing.T) {
	t.Run("returns the estimate", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search/validate", jsonOK(`{
			"query":"* | * | *",
			"startTime":1700000000,
			"endTime":1700003600,
			"stats":{"eventsInScope":10,"estimatedPrice":{"amount":0.25,"currency":"USD"}},
			"estimatedPrice":{"amount":0.25,"currency":"USD"}
		}`))

		v, err := org.ValidateSearch(SearchRequest{Query: "* | * | *", StartTime: 1700000000, EndTime: 1700003600})
		require.NoError(t, err)
		require.Empty(t, v.Error)
		require.Equal(t, int64(1700000000), v.StartTime)
		require.NotNil(t, v.EstimatedPrice)
		require.Equal(t, "USD", v.EstimatedPrice["currency"])

		calls := sr.Calls()
		require.Len(t, calls, 1)
		require.Equal(t, "/v1/search/validate", calls[0].path)
	})

	t.Run("a rejected query is reported in the body", func(t *testing.T) {
		_, org, sr := newSearchTestOrg(t)
		sr.on(http.MethodPost, "/v1/search/validate", jsonOK(`{"error":"unexpected token at offset 4"}`))

		v, err := org.ValidateSearch(SearchRequest{Query: "* | ?", StartTime: 1, EndTime: 2})
		require.NoError(t, err, "a query the server can judge is a successful validation")
		require.Equal(t, "unexpected token at offset 4", v.Error)
	})

	t.Run("a failed call is an error", func(t *testing.T) {
		_, org, _ := newSearchTestOrg(t)
		// Nothing is registered for the validate path, so the router answers
		// 404 the way an endpoint that is not there would.
		v, err := org.ValidateSearch(SearchRequest{Query: "*", StartTime: 1, EndTime: 2})
		require.Error(t, err)
		require.Nil(t, v)
	})
}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

func TestSearchExecuteOptionsDefaults(t *testing.T) {
	for name, tc := range map[string]struct {
		in       SearchExecuteOptions
		attempts int
		interval time.Duration
	}{
		"zero value":       {in: SearchExecuteOptions{}, attempts: defaultSearchMaxPollAttempts, interval: defaultSearchPollInterval},
		"negative":         {in: SearchExecuteOptions{MaxPollAttempts: -1, PollInterval: -time.Second}, attempts: defaultSearchMaxPollAttempts, interval: defaultSearchPollInterval},
		"caller's choices": {in: SearchExecuteOptions{MaxPollAttempts: 7, PollInterval: 3 * time.Second}, attempts: 7, interval: 3 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			got := tc.in.withDefaults()
			require.Equal(t, tc.attempts, got.MaxPollAttempts)
			require.Equal(t, tc.interval, got.PollInterval)
		})
	}
}

// TestFetchSearchPageRespectsServerCadence pins that the server's suggested
// delay governs whenever it is longer than the caller's floor. The floor is
// only there to stop an absent or tiny suggestion from becoming a busy loop.
func TestFetchSearchPageRespectsServerCadence(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":false,"nextPollInMs":120}`),
		jsonOK(`{"completed":true,"results":[]}`),
	)

	started := time.Now()
	_, err := org.FetchSearchPage(context.Background(), "query-1", "", SearchExecuteOptions{PollInterval: time.Millisecond})
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(started), 120*time.Millisecond,
		"a 120ms suggestion must not be shortened to the 1ms floor")
	require.Len(t, sr.callsFor(http.MethodGet), 2)
}

// ---------------------------------------------------------------------------
// Input validation
// ---------------------------------------------------------------------------

// TestSearchRequestValidation covers criteria that cannot describe a search.
// Each must be refused before any request is sent: a submission that reaches
// the server either fails there, after a round trip, or, for a millisecond
// timestamp, succeeds and searches a window thousands of years ahead, which
// comes back empty and reads as "nothing matched".
func TestSearchRequestValidation(t *testing.T) {
	now := int64(1700000000)
	invalid := map[string]struct {
		req       SearchRequest
		wantInErr string
	}{
		"empty query":                {req: SearchRequest{StartTime: now, EndTime: now + 1}, wantInErr: "a query is required"},
		"whitespace query":           {req: SearchRequest{Query: " \t\n", StartTime: now, EndTime: now + 1}, wantInErr: "a query is required"},
		"negative start":             {req: SearchRequest{Query: "*", StartTime: -1, EndTime: now}, wantInErr: "cannot be negative"},
		"negative end":               {req: SearchRequest{Query: "*", StartTime: 0, EndTime: -1}, wantInErr: "cannot be negative"},
		"most negative start":        {req: SearchRequest{Query: "*", StartTime: math.MinInt64, EndTime: now}, wantInErr: "cannot be negative"},
		"inverted range":             {req: SearchRequest{Query: "*", StartTime: now, EndTime: now - 1}, wantInErr: "is before start"},
		"start in milliseconds":      {req: SearchRequest{Query: "*", StartTime: now * 1000, EndTime: now * 1000}, wantInErr: "Unix seconds"},
		"end in milliseconds":        {req: SearchRequest{Query: "*", StartTime: now, EndTime: now * 1000}, wantInErr: "Unix seconds"},
		"end at the int64 maximum":   {req: SearchRequest{Query: "*", StartTime: now, EndTime: math.MaxInt64}, wantInErr: "Unix seconds"},
		"one past the plausible max": {req: SearchRequest{Query: "*", StartTime: now, EndTime: maxPlausibleSearchTime + 1}, wantInErr: "Unix seconds"},
	}

	for name, tc := range invalid {
		t.Run("submit refuses "+name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			queryID, err := org.InitiateSearch(tc.req)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantInErr)
			require.Empty(t, queryID)
			require.Empty(t, sr.Calls(), "an invalid search must not reach the server")
		})
		t.Run("validate refuses "+name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			v, err := org.ValidateSearch(tc.req)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantInErr)
			require.Nil(t, v)
			require.Empty(t, sr.Calls())
		})
		t.Run("execute refuses "+name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			called := false
			err := org.ExecuteSearch(context.Background(), tc.req, fastPolling, func(*SearchPoll) (bool, error) {
				called = true
				return true, nil
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantInErr)
			require.False(t, called)
			require.Empty(t, sr.Calls(), "no submission, and so nothing to cancel either")
		})
	}

	// The boundaries the server itself accepts must stay accepted: an empty
	// range (start equal to end), the epoch, and the largest plausible time.
	valid := map[string]SearchRequest{
		"empty range":              {Query: "*", StartTime: now, EndTime: now},
		"epoch":                    {Query: "*", StartTime: 0, EndTime: 0},
		"largest plausible time":   {Query: "*", StartTime: 0, EndTime: maxPlausibleSearchTime},
		"query with outer spacing": {Query: "  * | * | *  ", StartTime: now, EndTime: now + 1},
	}
	for name, req := range valid {
		t.Run("submit accepts "+name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
			queryID, err := org.InitiateSearch(req)
			require.NoError(t, err)
			require.Equal(t, "query-1", queryID)

			submitted := map[string]interface{}{}
			require.NoError(t, json.Unmarshal(sr.Calls()[0].body, &submitted))
			require.Equal(t, req.Query, submitted["query"], "the query is sent as given, not trimmed")
			require.Equal(t, strconv.FormatInt(req.StartTime, 10), submitted["startTime"])
			require.Equal(t, strconv.FormatInt(req.EndTime, 10), submitted["endTime"])
		})
	}
}

// TestSearchQueryIDRejectsDotSegments covers the two ids that escaping cannot
// keep to one segment. "/v1/search/.." normalizes to "/v1/", so a stored id of
// ".." would send a poll or a delete somewhere other than the search it names.
func TestSearchQueryIDRejectsDotSegments(t *testing.T) {
	for _, id := range []string{".", ".."} {
		t.Run("poll "+id, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			page, err := org.PollSearch(id, "")
			require.Error(t, err)
			require.Contains(t, err.Error(), "not a valid path segment")
			require.Nil(t, page)
			require.Empty(t, sr.Calls())
		})
		t.Run("cancel "+id, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			err := org.CancelSearch(id)
			require.Error(t, err)
			require.Contains(t, err.Error(), "not a valid path segment")
			require.Empty(t, sr.Calls())
		})
		t.Run("fetch "+id, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			page, err := org.FetchSearchPage(context.Background(), id, "", fastPolling)
			require.Error(t, err)
			require.Nil(t, page)
			require.Empty(t, sr.Calls())
		})
	}

	// Ids that merely contain dots, or an escaped dot-segment, are ordinary
	// single segments and must still be addressed.
	for _, id := range []string{"...", "a.b", ".hidden", "%2E%2E", "query-1."} {
		t.Run("ordinary id "+id, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodGet, "/v1/search/"+id, jsonOK(`{"completed":true,"results":[]}`))
			_, err := org.PollSearch(id, "")
			require.NoError(t, err)
			require.Equal(t, "/v1/search/"+id, sr.Calls()[0].path)
		})
	}
}

// ---------------------------------------------------------------------------
// Poll cadence
// ---------------------------------------------------------------------------

// TestSearchPollDelay pins how the server's suggested delay becomes a wait. The
// suggestion is honoured between the caller's floor and a one-minute ceiling;
// an absent one falls back to a second; and no value, however corrupt, may
// overflow into a negative or an unbounded wait.
func TestSearchPollDelay(t *testing.T) {
	for name, tc := range map[string]struct {
		nextPollInMs int64
		floor        time.Duration
		want         time.Duration
	}{
		"absent suggestion falls back":            {nextPollInMs: 0, floor: time.Millisecond, want: fallbackSearchPollInterval},
		"negative suggestion falls back":          {nextPollInMs: -5, floor: time.Millisecond, want: fallbackSearchPollInterval},
		"most negative suggestion falls back":     {nextPollInMs: math.MinInt64, floor: time.Millisecond, want: fallbackSearchPollInterval},
		"suggestion honoured above the floor":     {nextPollInMs: 750, floor: 500 * time.Millisecond, want: 750 * time.Millisecond},
		"floor wins over a smaller suggestion":    {nextPollInMs: 10, floor: 500 * time.Millisecond, want: 500 * time.Millisecond},
		"floor wins over the fallback":            {nextPollInMs: 0, floor: 3 * time.Second, want: 3 * time.Second},
		"just under the ceiling":                  {nextPollInMs: 59_999, floor: time.Millisecond, want: 59_999 * time.Millisecond},
		"at the ceiling":                          {nextPollInMs: 60_000, floor: time.Millisecond, want: maxSearchPollInterval},
		"past the ceiling is capped":              {nextPollInMs: 3_600_000, floor: time.Millisecond, want: maxSearchPollInterval},
		"overflowing suggestion is capped":        {nextPollInMs: math.MaxInt64, floor: time.Millisecond, want: maxSearchPollInterval},
		"overflow boundary is capped":             {nextPollInMs: math.MaxInt64/int64(time.Millisecond) + 1, floor: time.Millisecond, want: maxSearchPollInterval},
		"a caller's floor above the ceiling wins": {nextPollInMs: 3_600_000, floor: 2 * time.Minute, want: 2 * time.Minute},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, searchPollDelay(tc.nextPollInMs, tc.floor))
		})
	}
}

// TestFetchSearchPageNoWaitAfterLastPoll pins that the poll bound is reported
// as soon as the last allowed poll comes back incomplete. Waiting out the
// server's suggestion first would only delay an error that is already certain.
func TestFetchSearchPageNoWaitAfterLastPoll(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	// A 50s suggestion: a wait after the last poll would hold the call far past
	// the bound asserted below.
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":false,"nextPollInMs":50000}`))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	started := time.Now()
	page, err := org.FetchSearchPage(ctx, "query-1", "", SearchExecuteOptions{MaxPollAttempts: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "within 1 polls")
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, page)
	require.Less(t, time.Since(started), 10*time.Second)
	require.Len(t, sr.callsFor(http.MethodGet), 1)
}

// TestFetchSearchPageWaitIsInterruptible pins that a long suggested wait gives
// way to the caller's context rather than holding the call until it elapses.
func TestFetchSearchPageWaitIsInterruptible(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":false,"nextPollInMs":50000}`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.onCall = func(capturedRequest) {
		// Cancel shortly after the reply, so the call is in its wait.
		time.AfterFunc(20*time.Millisecond, cancel)
	}

	started := time.Now()
	_, err := org.FetchSearchPage(ctx, "query-1", "", SearchExecuteOptions{})
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(started), 10*time.Second)
	require.Len(t, sr.callsFor(http.MethodGet), 1)
}

// TestFetchSearchPageRetriesTransientFailure pins that a page fetch shares the
// SDK's request layer: a transient 5xx on one poll is retried within that
// poll, and the page is still delivered.
func TestFetchSearchPageRetriesTransientFailure(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonStatus(http.StatusServiceUnavailable, `{"error":"try again"}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"e1"}}]}]}`),
	)

	page, err := org.FetchSearchPage(context.Background(), "query-1", "", fastPolling)
	require.NoError(t, err)
	require.Len(t, page.Results, 1)
	require.Len(t, sr.callsFor(http.MethodGet), 2)
}

// ---------------------------------------------------------------------------
// ExecuteSearch: release on every abnormal exit, and termination
// ---------------------------------------------------------------------------

// TestExecuteSearchReleasesWhenACallbackDoesNotReturn covers the callbacks and
// handler leaving through a panic or runtime.Goexit. The search is already
// running by the time any of them is called, so each such exit must still
// cancel it, and a panic must still reach the caller.
func TestExecuteSearchReleasesWhenACallbackDoesNotReturn(t *testing.T) {
	type exit int
	const (
		viaPanic exit = iota
		viaGoexit
	)
	leave := func(how exit) {
		if how == viaPanic {
			panic("callback gave up")
		}
		runtime.Goexit()
	}

	for name, tc := range map[string]struct {
		how  exit
		opts func(exit) SearchExecuteOptions
		h    func(exit) SearchPageHandler
		gets int
	}{
		"OnQueryInitiated panics": {
			how: viaPanic,
			opts: func(how exit) SearchExecuteOptions {
				o := fastPolling
				o.OnQueryInitiated = func(string) { leave(how) }
				return o
			},
			gets: 0,
		},
		"OnQueryInitiated exits its goroutine": {
			how: viaGoexit,
			opts: func(how exit) SearchExecuteOptions {
				o := fastPolling
				o.OnQueryInitiated = func(string) { leave(how) }
				return o
			},
			gets: 0,
		},
		"OnPageCompleted panics": {
			how: viaPanic,
			opts: func(how exit) SearchExecuteOptions {
				o := fastPolling
				o.OnPageCompleted = func(int, string) { leave(how) }
				return o
			},
			gets: 1,
		},
		"handler panics": {
			how: viaPanic,
			h: func(how exit) SearchPageHandler {
				return func(*SearchPoll) (bool, error) { leave(how); return true, nil }
			},
			gets: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
			sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"more"}]}`))
			sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

			opts := fastPolling
			if tc.opts != nil {
				opts = tc.opts(tc.how)
			}
			handler := SearchPageHandler(func(*SearchPoll) (bool, error) { return true, nil })
			if tc.h != nil {
				handler = tc.h(tc.how)
			}

			var recovered interface{}
			returned := false
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() { recovered = recover() }()
				_ = org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, opts, handler)
				returned = true
			}()
			<-done

			require.False(t, returned, "the callback never returned, so neither may ExecuteSearch")
			if tc.how == viaPanic {
				require.Equal(t, "callback gave up", recovered, "the panic must reach the caller")
			} else {
				require.Nil(t, recovered)
			}
			require.Len(t, sr.callsFor(http.MethodGet), tc.gets)
			deletes := sr.callsFor(http.MethodDelete)
			require.Len(t, deletes, 1, "a search whose caller left mid-flight must be released")
			require.Equal(t, "/v1/search/query-1", deletes[0].path)
		})
	}
}

// TestExecuteSearchStopsOnARepeatedToken covers a server that answers a page
// with the token that fetched it. Following it would fetch the same page
// forever, so the driver hands that page over once, then stops with an error
// and releases the search.
func TestExecuteSearchStopsOnARepeatedToken(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"token-2"}]}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"token-2"}]}`),
	)
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

	pages := 0
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(*SearchPoll) (bool, error) {
			pages++
			return true, nil
		})
	require.Error(t, err)
	require.Contains(t, err.Error(), "continuation token it was fetched with")
	require.Equal(t, 2, pages, "both pages fetched are delivered; the repeat is what stops the search")

	gets := sr.callsFor(http.MethodGet)
	require.Len(t, gets, 2)
	require.Equal(t, "", gets[0].query.Get("token"))
	require.Equal(t, "token-2", gets[1].query.Get("token"))
	require.Len(t, sr.callsFor(http.MethodDelete), 1)
}

// TestExecuteSearchRepeatedTokenAfterStopIsNotAnError pins that the repeat
// check only guards a fetch that would actually happen: a handler that has
// already asked to stop gets a clean return.
func TestExecuteSearchRepeatedTokenAfterStopIsNotAnError(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1",
		jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"token-2"}]}`),
		jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"token-2"}]}`),
	)
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

	pages := 0
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(*SearchPoll) (bool, error) {
			pages++
			return pages < 2, nil
		})
	require.NoError(t, err)
	require.Equal(t, 2, pages)
	require.Len(t, sr.callsFor(http.MethodDelete), 1)
}

// TestExecuteSearchContextDoneBeforeSubmission pins that a caller whose
// context has already ended starts nothing: no search, so nothing to cancel.
func TestExecuteSearchContextDoneBeforeSubmission(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	initiated := false
	opts := fastPolling
	opts.OnQueryInitiated = func(string) { initiated = true }
	err := org.ExecuteSearch(ctx, SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, opts,
		func(*SearchPoll) (bool, error) { return true, nil })
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, initiated)
	require.Empty(t, sr.Calls(), "a done context must not reach the server, and leaves nothing to cancel")
}

// TestExecuteSearchCancelledDuringSubmission covers the context ending while
// the submission is in flight. The driver never learns a query id, so it
// cannot cancel anything; what it must do is report the cancellation and not
// go on to fetch a page.
func TestExecuteSearchCancelledDuringSubmission(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	sr.onCall = func(c capturedRequest) {
		if c.method == http.MethodPost {
			cancel()
			// Hold the reply until the client has given up on it, so the
			// cancellation, not the reply, decides the outcome.
			<-release
		}
	}
	defer close(release)

	err := org.ExecuteSearch(ctx, SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(*SearchPoll) (bool, error) { return true, nil })
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, sr.callsFor(http.MethodPost), 1, "a cancelled submission is not retried")
	require.Empty(t, sr.callsFor(http.MethodGet))
	require.Empty(t, sr.callsFor(http.MethodDelete))
}

// TestExecuteSearchReleaseOutlivesTheCallersContext pins that the best-effort
// cancel is still delivered when the reason for stopping is the caller's own
// context, which by then refuses any request made on it.
func TestExecuteSearchReleaseOutlivesTheCallersContext(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"more"}]}`))
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonOK(`{}`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := org.ExecuteSearch(ctx, SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(*SearchPoll) (bool, error) {
			cancel()
			return true, nil
		})
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, sr.callsFor(http.MethodGet), 1)
	require.Len(t, sr.callsFor(http.MethodDelete), 1)
}

// TestExecuteSearchReleaseFailureIsNotReported pins that the best-effort cancel
// never replaces the error that made the driver stop: the caller learns why the
// search ended, not that the clean-up after it failed.
func TestExecuteSearchReleaseFailureIsNotReported(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodPost, "/v1/search", jsonOK(`{"queryId":"query-1"}`))
	sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(`{"completed":true,"results":[{"type":"events","nextToken":"more"}]}`))
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonStatus(http.StatusForbidden, `{"error":"no"}`))

	sentinel := errors.New("consumer is gone")
	err := org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, fastPolling,
		func(*SearchPoll) (bool, error) { return true, sentinel })
	require.Equal(t, sentinel, err, "the handler's error is returned as is")
	require.Len(t, sr.callsFor(http.MethodDelete), 1)
}

// TestExecuteSearchConcurrentSearches runs several searches through one
// Organization at once, each with its own pages. Run under -race it checks the
// driver keeps no state outside the call, and the per-query routing checks no
// page or cancel crosses from one search to another.
func TestExecuteSearchConcurrentSearches(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	const n = 8

	// The POSTs share one path, so each gets a distinct id in arrival order;
	// every search's pages are routed by that id.
	posts := make([]cannedResponse, 0, n)
	for i := 0; i < n; i++ {
		id := "query-" + strconv.Itoa(i)
		posts = append(posts, jsonOK(`{"queryId":"`+id+`"}`))
		sr.on(http.MethodGet, "/v1/search/"+id,
			jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"`+id+`-a"}}],"nextToken":"`+id+`-t2"}]}`),
			jsonOK(`{"completed":true,"results":[{"type":"events","rows":[{"mtd":{"id":"`+id+`-b"}}]}]}`),
		)
	}
	sr.on(http.MethodPost, "/v1/search", posts...)

	var wg sync.WaitGroup
	errs := make([]error, n)
	seen := make([][]string, n)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			opts := fastPolling
			opts.OnQueryInitiated = func(id string) { ids[i] = id }
			errs[i] = org.ExecuteSearch(context.Background(), SearchRequest{Query: "*", StartTime: 1, EndTime: 2}, opts,
				func(page *SearchPoll) (bool, error) {
					for _, item := range page.Results {
						for _, row := range item.Rows {
							seen[i] = append(seen[i], row["mtd"].(map[string]interface{})["id"].(string))
						}
					}
					return true, nil
				})
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, []string{ids[i] + "-a", ids[i] + "-b"}, seen[i], "search %d saw another search's rows", i)
	}
	require.Len(t, sr.callsFor(http.MethodGet), 2*n)
	require.Empty(t, sr.callsFor(http.MethodDelete), "every search ran to its end")
}

// ---------------------------------------------------------------------------
// CancelSearch status
// ---------------------------------------------------------------------------

// TestCancelSearchGoneReportsNotFound pins the contract the CancelSearch doc
// states: a search that has already finished or expired answers 404, and the
// returned error carries that status so a caller can treat it as done without
// matching on message text.
func TestCancelSearchGoneReportsNotFound(t *testing.T) {
	_, org, sr := newSearchTestOrg(t)
	sr.on(http.MethodDelete, "/v1/search/query-1", jsonStatus(http.StatusNotFound, `{"error":"search not found"}`))

	err := org.CancelSearch("query-1")
	require.Error(t, err)
	var restErr RESTError
	require.True(t, errors.As(err, &restErr), "the status must be reachable through errors.As, got %T: %v", err, err)
	require.Equal(t, http.StatusNotFound, restErr.StatusCode())
	require.Len(t, sr.callsFor(http.MethodDelete), 1, "a 404 is definitive and is not retried")
}

// TestPollSearchUndecodablePageIsAnError covers replies that are valid JSON but
// not a page. Each must be an error rather than a zero page, which would read
// as "not complete yet" and be polled again until the bound.
func TestPollSearchUndecodablePageIsAnError(t *testing.T) {
	for name, body := range map[string]string{
		"completed is a string":   `{"completed":"true","results":[]}`,
		"results is an object":    `{"completed":true,"results":{"type":"events"}}`,
		"rows hold a non-object":  `{"completed":true,"results":[{"type":"events","rows":[1]}]}`,
		"nextPollInMs is a float": `{"completed":false,"nextPollInMs":1.5}`,
		"partial is a string":     `{"completed":true,"results":[],"partial":"yes"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, org, sr := newSearchTestOrg(t)
			sr.on(http.MethodGet, "/v1/search/query-1", jsonOK(body))
			page, err := org.FetchSearchPage(context.Background(), "query-1", "", fastPolling)
			require.Error(t, err)
			require.Contains(t, err.Error(), "error parsing response")
			require.Nil(t, page)
		})
	}
}

// TestSearchRowNumbersKeepPrecision pins that an event's integers survive
// decoding exactly. Event bodies carry 64-bit values such as addresses and
// sizes that a float64 cannot hold, so rows must not round them.
func TestSearchRowNumbersKeepPrecision(t *testing.T) {
	var page SearchPoll
	require.NoError(t, json.Unmarshal([]byte(`{"completed":true,"results":[{"type":"events","rows":[
		{"mtd":{"id":"e1","ts":1700000000123},"data":{"event":{"BASE_ADDRESS":18446744073709551615,"SIZE":9007199254740993}}}
	]}]}`), &page))

	event := page.Results[0].Rows[0]["data"].(map[string]interface{})["event"].(map[string]interface{})
	require.Equal(t, uint64(18446744073709551615), event["BASE_ADDRESS"])
	require.Equal(t, uint64(9007199254740993), event["SIZE"])
}

// TestSearchRowFloatsInExponentFormDecode pins that a page whose events carry
// a float written with an exponent, which is how JSON encoders commonly write
// very small and very large values, still decodes, and that integers outside
// the 64-bit ranges decode as floats rather than failing the whole page.
func TestSearchRowFloatsInExponentFormDecode(t *testing.T) {
	var page SearchPoll
	require.NoError(t, json.Unmarshal([]byte(`{"completed":true,"results":[{"type":"events","rows":[
		{"mtd":{"id":"e1"},"data":{"event":{"LOAD":1e-07,"BIG":1E+21,"HUGE_INT":123456789012345678901234567890,"NEG":-99999999999999999999}}}
	]}]}`), &page))

	event := page.Results[0].Rows[0]["data"].(map[string]interface{})["event"].(map[string]interface{})
	require.Equal(t, 1e-07, event["LOAD"])
	require.Equal(t, 1e21, event["BIG"])
	require.Equal(t, 1.2345678901234568e29, event["HUGE_INT"])
	require.Equal(t, -1e20, event["NEG"])
}

// TestSearchCallsWithoutASearchHost covers an organization whose URL map has
// no search host. Every call must fail with that reason and send nothing,
// rather than fall back to the main API host, where the same paths mean
// something else or nothing at all.
func TestSearchCallsWithoutASearchHost(t *testing.T) {
	ms := NewMockServer(searchTestOID)
	t.Cleanup(ms.Close)
	sr := newSearchRouter()
	sr.install(ms)
	ms.URLs.Search = ""
	org, err := ms.NewOrganization()
	require.NoError(t, err)

	req := SearchRequest{Query: "*", StartTime: 1, EndTime: 2}
	for name, call := range map[string]func() error{
		"initiate": func() error { _, err := org.InitiateSearch(req); return err },
		"poll":     func() error { _, err := org.PollSearch("query-1", ""); return err },
		"fetch": func() error {
			_, err := org.FetchSearchPage(context.Background(), "query-1", "", fastPolling)
			return err
		},
		"execute": func() error {
			return org.ExecuteSearch(context.Background(), req, fastPolling, func(*SearchPoll) (bool, error) { return true, nil })
		},
		"cancel":   func() error { return org.CancelSearch("query-1") },
		"validate": func() error { _, err := org.ValidateSearch(req); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)
			require.Contains(t, err.Error(), "search URL not found")
		})
	}
	require.Empty(t, sr.Calls(), "no request may be sent without a search host")
}
