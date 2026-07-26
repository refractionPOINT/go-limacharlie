package limacharlie

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const searchQueriesTestOID = "00000000-0000-0000-0000-000000000042"

// searchQueriesRouter records every request to the listing path and replies
// with canned bodies in order, so a test can drive multi-page walks.
type searchQueriesRouter struct {
	bodies []string
	calls  []capturedRequest
	status int
}

func newSearchQueriesRouter(bodies ...string) *searchQueriesRouter {
	return &searchQueriesRouter{bodies: bodies}
}

func (r *searchQueriesRouter) install(ms *MockServer) {
	ms.CustomHandlers["/v1/search/"] = func(w http.ResponseWriter, req *http.Request) {
		r.calls = append(r.calls, capturedRequest{
			method: req.Method,
			path:   req.URL.Path,
			query:  req.URL.Query(),
		})
		w.Header().Set("Content-Type", "application/json")
		if r.status != 0 {
			w.WriteHeader(r.status)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		idx := len(r.calls) - 1
		if idx >= len(r.bodies) {
			idx = len(r.bodies) - 1
		}
		_, _ = w.Write([]byte(r.bodies[idx]))
	}
}

func newSearchQueriesTestOrg(t *testing.T, router *searchQueriesRouter) (*MockServer, *Organization) {
	t.Helper()
	ms := NewMockServer(searchQueriesTestOID)
	router.install(ms)
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	return ms, org
}

// TestListOpenQueriesRequestShape pins where the request goes and what it
// carries. The organization is named in the path rather than as a parameter,
// so a listing is never addressable without naming one.
func TestListOpenQueriesRequestShape(t *testing.T) {
	router := newSearchQueriesRouter(`{"oid":"x","limit":10,"slotsHeld":0,"count":0,"queries":[]}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	_, err := org.ListOpenQueries(OpenSearchQueriesFilters{State: "executing", Limit: 25, Offset: 50})
	require.NoError(t, err)

	require.Len(t, router.calls, 1)
	call := router.calls[0]
	require.Equal(t, http.MethodGet, call.method)
	require.Equal(t, "/v1/search/"+searchQueriesTestOID+"/queries", call.path)
	require.Equal(t, "executing", call.query.Get("state"))
	require.Equal(t, "25", call.query.Get("limit"))
	require.Equal(t, "50", call.query.Get("offset"))
}

// TestListOpenQueriesDefaultsAndOmissions covers the zero-value filter. Sending
// limit=0 would make the server clamp via its malformed-value path rather than
// apply its documented default.
func TestListOpenQueriesDefaultsAndOmissions(t *testing.T) {
	router := newSearchQueriesRouter(`{"queries":[]}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	_, err := org.ListOpenQueries(OpenSearchQueriesFilters{})
	require.NoError(t, err)

	call := router.calls[0]
	require.Equal(t, "all", call.query.Get("state"), "an unset state must become the explicit default")
	require.False(t, call.query.Has("limit"))
	require.False(t, call.query.Has("offset"))
}

func TestListOpenQueriesRejectsUnknownState(t *testing.T) {
	router := newSearchQueriesRouter(`{"queries":[]}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	for _, bad := range []string{"running", "active", "queued", "sideways"} {
		_, err := org.ListOpenQueries(OpenSearchQueriesFilters{State: bad})
		require.Error(t, err, "state %q", bad)
		require.Contains(t, err.Error(), "must be one of")
	}
	// A typo must be a local error, not a round trip that comes back 400.
	require.Empty(t, router.calls)
}

func TestListOpenQueriesNormalisesState(t *testing.T) {
	router := newSearchQueriesRouter(`{"queries":[]}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	for _, ok := range []string{"IDLE", " idle ", "Executing", "ALL"} {
		_, err := org.ListOpenQueries(OpenSearchQueriesFilters{State: ok})
		require.NoError(t, err, "state %q", ok)
	}
	require.Equal(t, "idle", router.calls[0].query.Get("state"))
	require.Equal(t, "idle", router.calls[1].query.Get("state"))
	require.Equal(t, "executing", router.calls[2].query.Get("state"))
	require.Equal(t, "all", router.calls[3].query.Get("state"))
}

// TestListOpenQueriesDecoding is the contract test for the response: every
// field a caller acts on has to survive decoding with its meaning intact,
// including the pointers that distinguish "absent" from "zero".
func TestListOpenQueriesDecoding(t *testing.T) {
	body := `{
	  "oid": "` + searchQueriesTestOID + `",
	  "limit": 10,
	  "slotsHeld": 1,
	  "count": 3,
	  "truncated": false,
	  "queries": [
	    {
	      "queryId": "running-one",
	      "state": "executing",
	      "holdsSlot": true,
	      "page": "46fff3d9d1f91a7c",
	      "query": "-24h | plat == windows | * | *",
	      "stream": "event",
	      "startTime": "1785000909",
	      "endTime": "1785087309",
	      "submittedBy": "someone@example.com",
	      "userAgent": "go-limacharlie/1.8.0",
	      "submittedAt": "2026-07-25T14:31:12Z",
	      "startedAt": "2026-07-25T14:31:19Z",
	      "runningForMs": 41000,
	      "pagesCompleted": 2,
	      "hasMorePages": true,
	      "batchesCompleted": 45,
	      "batchesInScope": 200,
	      "progressPercent": 22.5,
	      "eventsScanned": 9000000,
	      "billedEvents": 8100000,
	      "slotExpiresAt": "2026-07-25T14:40:12Z",
	      "resumableUntil": "2026-07-26T14:31:12Z"
	    },
	    {
	      "queryId": "paused-one",
	      "state": "idle",
	      "holdsSlot": false,
	      "submittedAt": "2026-07-25T14:12:00Z",
	      "pagesCompleted": 7,
	      "batchesCompleted": 0,
	      "batchesInScope": 0,
	      "eventsScanned": 0,
	      "billedEvents": 0,
	      "resumableUntil": "2026-07-26T14:12:00Z"
	    },
	    {
	      "queryId": "not-tracked",
	      "state": "unknown",
	      "holdsSlot": null,
	      "submittedAt": "2026-07-25T14:20:00Z",
	      "pagesCompleted": 0,
	      "batchesCompleted": 0,
	      "batchesInScope": 0,
	      "eventsScanned": 0,
	      "billedEvents": 0,
	      "resumableUntil": "2026-07-26T14:20:00Z"
	    }
	  ]
	}`
	router := newSearchQueriesRouter(body)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	resp, err := org.ListOpenQueries(OpenSearchQueriesFilters{})
	require.NoError(t, err)

	// The distinction the endpoint exists to make: three searches open, one
	// consuming the limit.
	require.Equal(t, int64(3), resp.Count)
	require.Equal(t, int64(1), resp.SlotsHeld)
	require.Equal(t, 10, resp.Limit)
	require.Len(t, resp.Queries, 3)

	running := resp.Queries[0]
	require.Equal(t, SearchQueryExecuting, running.State)
	require.NotNil(t, running.HoldsSlot)
	require.True(t, *running.HoldsSlot)
	require.NotNil(t, running.RunningForMs)
	require.Equal(t, int64(41000), *running.RunningForMs)
	require.NotNil(t, running.ProgressPercent)
	require.InDelta(t, 22.5, *running.ProgressPercent, 0.001)
	require.NotNil(t, running.HasMorePages)
	require.True(t, *running.HasMorePages)
	require.Equal(t, uint64(9000000), running.EventsScanned)
	require.Equal(t, "someone@example.com", running.SubmittedBy)
	// The page is a digest of the cursor, never the cursor itself, so it can
	// never be used to resume somebody else's search.
	require.Equal(t, "46fff3d9d1f91a7c", running.Page)

	idle := resp.Queries[1]
	require.Equal(t, SearchQueryIdle, idle.State)
	require.NotNil(t, idle.HoldsSlot)
	require.False(t, *idle.HoldsSlot, "an idle search must never report as consuming a slot")
	// Absent rather than zero: no page is running, so there is no running time.
	require.Nil(t, idle.RunningForMs)
	require.Empty(t, idle.StartedAt)
	// No denominator means progress is unavailable, not zero.
	require.Nil(t, idle.ProgressPercent)
	require.Equal(t, 7, idle.PagesCompleted)

	unknown := resp.Queries[2]
	require.Equal(t, SearchQueryUnknown, unknown.State)
	require.Nil(t, unknown.HoldsSlot, "a null holdsSlot must decode as nil, not as false")
}

func TestListOpenQueriesPropagatesServerFailure(t *testing.T) {
	router := newSearchQueriesRouter(`{}`)
	router.status = http.StatusInternalServerError
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	// An empty listing on failure would read as "nothing is open", which is
	// the wrong conclusion for a caller deciding whether to retry a search.
	resp, err := org.ListOpenQueries(OpenSearchQueriesFilters{})
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "failed to list open queries")
}

func TestListAllOpenQueriesWalksEveryPage(t *testing.T) {
	page := func(truncated bool, ids ...string) string {
		entries := make([]OpenSearchQuery, 0, len(ids))
		for _, id := range ids {
			entries = append(entries, OpenSearchQuery{QueryID: id, State: SearchQueryIdle})
		}
		raw, err := json.Marshal(OpenSearchQueries{Truncated: truncated, Queries: entries})
		if err != nil {
			panic(err)
		}
		return string(raw)
	}

	router := newSearchQueriesRouter(
		page(true, "q1", "q2"),
		page(true, "q3"),
		page(false, "q4"),
	)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	all, err := org.ListAllOpenQueries(context.Background(), "all")
	require.NoError(t, err)
	require.Equal(t, []string{"q1", "q2", "q3", "q4"}, openQueryIDs(all))
	require.Len(t, router.calls, 3)

	offsets := make([]string, 0, len(router.calls))
	for _, c := range router.calls {
		offsets = append(offsets, c.query.Get("offset"))
	}
	require.Equal(t, []string{"", "200", "400"}, offsets)
}

// TestListAllOpenQueriesKeepsWalkingPastAnEmptyPage covers the filter's
// interaction with paging: the server filters after paging, so a page can come
// back empty while more entries exist. Stopping there would silently truncate.
func TestListAllOpenQueriesKeepsWalkingPastAnEmptyPage(t *testing.T) {
	router := newSearchQueriesRouter(
		`{"truncated":true,"queries":[]}`,
		`{"truncated":false,"queries":[{"queryId":"q9","state":"idle"}]}`,
	)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	all, err := org.ListAllOpenQueries(context.Background(), "idle")
	require.NoError(t, err)
	require.Equal(t, []string{"q9"}, openQueryIDs(all))
	require.Len(t, router.calls, 2)
}

// TestListAllOpenQueriesDeduplicates covers the listing shifting under the
// walk: entries leave the index as their searches finish, which moves later
// offsets back and can re-serve one already returned.
func TestListAllOpenQueriesDeduplicates(t *testing.T) {
	router := newSearchQueriesRouter(
		`{"truncated":true,"queries":[{"queryId":"q1"},{"queryId":"q2"}]}`,
		`{"truncated":false,"queries":[{"queryId":"q2"},{"queryId":"q3"}]}`,
	)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	all, err := org.ListAllOpenQueries(context.Background(), "all")
	require.NoError(t, err)
	require.Equal(t, []string{"q1", "q2", "q3"}, openQueryIDs(all))
}

func TestListAllOpenQueriesEmptyOrg(t *testing.T) {
	router := newSearchQueriesRouter(`{"count":0,"truncated":false,"queries":[]}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	all, err := org.ListAllOpenQueries(context.Background(), "all")
	require.NoError(t, err)
	require.Empty(t, all)
	require.NotNil(t, all, "an empty result must be an empty slice, not nil")
}

func TestListAllOpenQueriesStopsOnError(t *testing.T) {
	router := newSearchQueriesRouter(`{}`)
	router.status = http.StatusInternalServerError
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	all, err := org.ListAllOpenQueries(context.Background(), "all")
	require.Error(t, err)
	require.Nil(t, all, "a partial walk must not be returned as if it were complete")
}

func TestListOpenQueriesHonoursContextCancellation(t *testing.T) {
	router := newSearchQueriesRouter(`{"queries":[]}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := org.ListOpenQueriesWithContext(ctx, OpenSearchQueriesFilters{})
	require.Error(t, err)
}

func openQueryIDs(entries []OpenSearchQuery) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.QueryID)
	}
	return out
}

// TestGetSearchLimitsRequestShape pins where the request goes. The organization
// is named in the path rather than as a parameter, so limits are never
// addressable without naming one.
func TestGetSearchLimitsRequestShape(t *testing.T) {
	router := newSearchQueriesRouter(`{"oid":"x"}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	_, err := org.GetSearchLimits()
	require.NoError(t, err)

	require.Len(t, router.calls, 1)
	require.Equal(t, http.MethodGet, router.calls[0].method)
	require.Equal(t, "/v1/search/"+searchQueriesTestOID+"/limits", router.calls[0].path)
	// The endpoint takes no parameters; sending some would be silently ignored
	// rather than erroring, so an accidental addition needs catching here.
	require.Empty(t, router.calls[0].query)
}

// TestGetSearchLimitsDecoding pins the full response contract, including the
// distinction the whole design turns on: a limit that is not enforced arrives as
// null and must decode to nil, never to zero.
func TestGetSearchLimitsDecoding(t *testing.T) {
	body := `{
		"oid": "` + searchQueriesTestOID + `",
		"concurrency": {"maxConcurrentQueries": 25},
		"pagination": {"resultsPerPage": 200, "maxPageDurationSeconds": 30, "maxCursorBytes": 4096},
		"retention": {"resumableForSeconds": 86400, "pageResultsForSeconds": 900},
		"execution": {
			"maxQueryDurationSeconds": 480,
			"maxAggregationDurationSeconds": 540,
			"maxResponseBytes": null
		},
		"request": {"maxRequestBodyBytes": 102400},
		"capabilities": {"openQueryListing": true}
	}`
	router := newSearchQueriesRouter(body)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	limits, err := org.GetSearchLimits()
	require.NoError(t, err)

	require.Equal(t, searchQueriesTestOID, limits.OID)
	require.Equal(t, 25, limits.Concurrency.MaxConcurrentQueries)

	require.Equal(t, int64(200), limits.Pagination.ResultsPerPage)
	require.Equal(t, int64(30), limits.Pagination.MaxPageDurationSeconds)
	require.Equal(t, 4096, limits.Pagination.MaxCursorBytes)

	require.Equal(t, int64(86400), limits.Retention.ResumableForSeconds)
	require.Equal(t, int64(900), limits.Retention.PageResultsForSeconds)
	// The TTL split: page results age out before the search stops being
	// resumable, and re-reading such a page recomputes it rather than failing.
	require.Less(t, limits.Retention.PageResultsForSeconds, limits.Retention.ResumableForSeconds)

	require.NotNil(t, limits.Execution.MaxQueryDurationSeconds)
	require.Equal(t, int64(480), *limits.Execution.MaxQueryDurationSeconds)
	require.NotNil(t, limits.Execution.MaxAggregationDurationSeconds)
	require.Equal(t, int64(540), *limits.Execution.MaxAggregationDurationSeconds)
	require.Nil(t, limits.Execution.MaxResponseBytes, "an unenforced limit must decode to nil, not zero")

	require.Equal(t, 102400, limits.Request.MaxRequestBodyBytes)
	require.True(t, limits.Capabilities.OpenQueryListing)
}

// A deployment enforcing nothing must be distinguishable from one enforcing a
// zero-second deadline. Decoding null to zero would tell a caller it has no time
// to run a query at all.
func TestGetSearchLimitsUnenforcedLimitsDecodeToNil(t *testing.T) {
	body := `{
		"oid": "x",
		"execution": {
			"maxQueryDurationSeconds": null,
			"maxAggregationDurationSeconds": null,
			"maxResponseBytes": null
		},
		"capabilities": {"openQueryListing": false}
	}`
	router := newSearchQueriesRouter(body)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	limits, err := org.GetSearchLimits()
	require.NoError(t, err)

	require.Nil(t, limits.Execution.MaxQueryDurationSeconds)
	require.Nil(t, limits.Execution.MaxAggregationDurationSeconds)
	require.Nil(t, limits.Execution.MaxResponseBytes)
	require.False(t, limits.Capabilities.OpenQueryListing)
}

// Unknown fields must not break decoding: the contract is additive, and an older
// SDK has to keep working against a newer deployment.
func TestGetSearchLimitsIgnoresUnknownFields(t *testing.T) {
	body := `{
		"oid": "x",
		"concurrency": {"maxConcurrentQueries": 10, "somethingNew": 5},
		"aFutureGroup": {"whatever": true}
	}`
	router := newSearchQueriesRouter(body)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	limits, err := org.GetSearchLimits()
	require.NoError(t, err)
	require.Equal(t, 10, limits.Concurrency.MaxConcurrentQueries)
}

func TestGetSearchLimitsPropagatesServerFailure(t *testing.T) {
	router := newSearchQueriesRouter(`{}`)
	router.status = http.StatusInternalServerError
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	_, err := org.GetSearchLimits()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get search limits")
}

func TestGetSearchLimitsHonoursContextCancellation(t *testing.T) {
	router := newSearchQueriesRouter(`{"oid":"x"}`)
	ms, org := newSearchQueriesTestOrg(t, router)
	defer ms.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := org.GetSearchLimitsWithContext(ctx)
	require.Error(t, err)
}

// The JSON tags must survive a round trip, so a caller re-encoding what it read
// produces something the SDK can read back.
func TestSearchLimitsRoundTrips(t *testing.T) {
	deadline := int64(480)
	original := SearchLimits{
		OID:         searchQueriesTestOID,
		Concurrency: SearchConcurrencyLimits{MaxConcurrentQueries: 10},
		Retention:   SearchRetentionLimits{ResumableForSeconds: 600, PageResultsForSeconds: 600},
		Execution:   SearchExecutionLimits{MaxQueryDurationSeconds: &deadline},
	}

	encoded, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded SearchLimits
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	require.Equal(t, original.OID, decoded.OID)
	require.Equal(t, original.Concurrency, decoded.Concurrency)
	require.Equal(t, original.Retention, decoded.Retention)
	require.NotNil(t, decoded.Execution.MaxQueryDurationSeconds)
	require.Equal(t, deadline, *decoded.Execution.MaxQueryDurationSeconds)
	require.Nil(t, decoded.Execution.MaxResponseBytes)
}
