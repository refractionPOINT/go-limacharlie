package limacharlie

// End-to-end tests for the Search API against a live organization.
//
// These run in CI (cloudbuild.yaml sets LC_SEARCH_E2E=1 next to the build's
// _OID and _KEY) and skip everywhere else by default. Everything in
// search_test.go runs against the package's MockServer and is this package's
// regression suite; this file checks the same calls against a real server, so
// a failure here can come from the deployment as well as from this code, which
// is why the two are kept in separate files with separate name prefixes.
//
// They are gated on their own environment variable rather than on credentials
// alone, deliberately. The rest of this package's credential-gated tests run
// wherever _OID and _KEY happen to be set. These also start real searches,
// which take a concurrency slot and are billed, and they need an organization
// with recent telemetry. Setting the variable is what says both are intended.
//
// To run them:
//
//	cd limacharlie && LC_SEARCH_E2E=1 _OID=<oid> _KEY=<api key> go test -run TestSearchE2E -v .
//
// limacharlie/ is its own Go module, so the tests run from inside it, as CI
// runs them.
//
// Optional knobs, both with defaults that keep a run small:
//
//	LC_SEARCH_E2E_LOOKBACK_HOURS   how far back to search (default 2)
//	LC_SEARCH_E2E_MAX_PAGES        pages to walk before stopping (default 3)
//
// Cost and time. Every window is as small as its assertion needs: two hours
// for the tests that need one page with rows, and one settled hour for the
// cross-mode comparison. The tests run in parallel with each other (t.Parallel)
// and share one Organization, built and authenticated once. The number of
// searches in flight at once is bounded by a semaphore sized from the
// organization's own concurrency limit (GetSearchLimits), at half of it so a
// concurrent build against the same organization still has room, and at most
// searchE2EMaxParallel; without that bound, parallel tests would be refused
// with 429 and wait out the request layer's retry back-off instead of running.
// Polling follows the server's advised cadence (FetchSearchPage); nothing here
// sleeps for a fixed time.
//
// Each search is bounded by searchE2EBudget, and every context also ends
// searchE2ECleanupMargin before the test binary's -timeout deadline, so a slow
// or hung search fails its test with a context error while the server-side
// cancel still runs, instead of the binary panicking with searches left open.
// Every search these tests start is cancelled server-side when they stop
// early, so none is left holding a concurrency slot.
//
// A window with no events is not a failure: the tests that need rows skip,
// saying so, because which organization CI points at and how busy it is are
// outside this code.
//
// What they do and do not assert: a page is asserted to REPORT an applied
// mode, not to report the mode that was requested. The server resolves the
// mode it runs a search as, and it is entitled to resolve it to something
// other than what the submission asked for. A test demanding equality would
// fail for a legitimate reason. Equality is only worth asserting where the
// test controls the organization's own configuration, and these do not.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	// searchE2EGateEnv opts in to the tests in this file.
	searchE2EGateEnv = "LC_SEARCH_E2E"
	// searchE2ELookbackEnv and searchE2EMaxPagesEnv size a run.
	searchE2ELookbackEnv = "LC_SEARCH_E2E_LOOKBACK_HOURS"
	searchE2EMaxPagesEnv = "LC_SEARCH_E2E_MAX_PAGES"

	defaultSearchE2ELookbackHours = 2
	defaultSearchE2EMaxPages      = 3

	// searchE2EBudget bounds one search. A first page over these windows
	// takes seconds; the bound is there so a hang fails fast rather than
	// eating the build's time.
	searchE2EBudget = 90 * time.Second

	// searchE2ECleanupMargin is how long before the test binary's deadline a
	// test's context ends, which leaves time for the server-side cancel.
	searchE2ECleanupMargin = 30 * time.Second

	// searchE2ECancelBudget bounds the cancel a test issues while cleaning up.
	searchE2ECancelBudget = 15 * time.Second

	// searchE2EMaxParallel caps how many searches these tests run at once,
	// whatever the organization's limit allows.
	searchE2EMaxParallel = 4

	// searchE2EQuery matches everything in the window, which is what gives a
	// paginated search the best chance of producing more than one page.
	searchE2EQuery = "* | * | *"
)

// knownAppliedSearchModes are the modes this build recognises in a page's
// stats.
//
// A value outside this set is not wrong on its own: a mode can be added
// server-side, and this SDK deliberately forwards a mode it does not know. The
// assertion exists so that a new one is added here on purpose, with someone
// looking at it, rather than the check being dropped the first time it is
// inconvenient.
var knownAppliedSearchModes = map[SearchMode]struct{}{
	SearchModeInteractive: {},
	SearchModeBatch:       {},
}

// searchE2EEnv is what every test in this file shares: one authenticated
// Organization and the semaphore that bounds concurrent searches.
type searchE2EEnv struct {
	org *Organization
	// slots holds one token per search allowed in flight.
	slots chan struct{}
}

var (
	searchE2EOnce   sync.Once
	searchE2EShared *searchE2EEnv
	searchE2EErr    error
)

// newSearchE2EEnv returns the shared environment, building it on first use,
// or skips the test.
//
// It SKIPS rather than fails when the environment is absent. The shared
// fixture in test_fixture.go calls FailNow for a missing _OID, which is why a
// checkout without credentials reports failures rather than skips; these tests
// must not add to that.
//
// The Organization is built once and authenticated by the GetSearchLimits
// call that sizes the semaphore, so the parallel tests start with a JWT in
// place rather than each exchanging the API key.
func newSearchE2EEnv(t *testing.T) *searchE2EEnv {
	t.Helper()

	if os.Getenv(searchE2EGateEnv) == "" {
		t.Skipf("skipping Search API end-to-end test: set %s=1, plus _OID and _KEY for a live organization with recent "+
			"telemetry, to run it. It is opt-in rather than credential-gated because it starts real, billed searches.",
			searchE2EGateEnv)
	}
	oid, key := os.Getenv("_OID"), os.Getenv("_KEY")
	if oid == "" || key == "" {
		t.Skipf("skipping Search API end-to-end test: %s is set but _OID and/or _KEY are not, and both are needed to "+
			"authenticate against a live organization.", searchE2EGateEnv)
	}

	searchE2EOnce.Do(func() {
		org, err := NewOrganizationFromClientOptions(ClientOptions{OID: oid, APIKey: key}, &LCLoggerGCP{})
		if err != nil {
			searchE2EErr = err
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), searchE2ECancelBudget)
		defer cancel()
		parallel := 1
		if limits, err := org.GetSearchLimitsWithContext(ctx); err == nil {
			parallel = limits.Concurrency.MaxConcurrentQueries / 2
		} else {
			// Serial is always safe; the limit only decides how much faster
			// than serial the run may go.
			t.Logf("could not read the organization's search limits, running searches one at a time: %v", err)
		}
		if parallel < 1 {
			parallel = 1
		}
		if parallel > searchE2EMaxParallel {
			parallel = searchE2EMaxParallel
		}
		t.Logf("running up to %d search(es) at once", parallel)
		searchE2EShared = &searchE2EEnv{org: org, slots: make(chan struct{}, parallel)}
	})
	require.NoError(t, searchE2EErr, "could not build an organization from _OID and _KEY")
	return searchE2EShared
}

// acquire takes a search slot, or fails the test if ctx ends first. The
// returned func gives it back and is safe to call more than once.
func (e *searchE2EEnv) acquire(t *testing.T, ctx context.Context) (release func()) {
	t.Helper()
	select {
	case e.slots <- struct{}{}:
	case <-ctx.Done():
		require.NoError(t, ctx.Err(), "timed out waiting for a search slot")
	}
	var once sync.Once
	return func() { once.Do(func() { <-e.slots }) }
}

// searchE2EContext returns a context for one search: searchE2EBudget long, but
// ending searchE2ECleanupMargin before the test binary's -timeout deadline when
// that comes first, so the test fails cleanly and its cleanup runs.
func searchE2EContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	deadline := time.Now().Add(searchE2EBudget)
	if testDeadline, ok := t.Deadline(); ok {
		if bound := testDeadline.Add(-searchE2ECleanupMargin); bound.Before(deadline) {
			deadline = bound
		}
	}
	return context.WithDeadline(context.Background(), deadline)
}

// cancelSearchE2E releases a search a test started, on a bounded context of
// its own, logging rather than failing when the search is already gone.
func cancelSearchE2E(t *testing.T, org *Organization, queryID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), searchE2ECancelBudget)
	defer cancel()
	if err := org.CancelSearchWithContext(ctx, queryID); err != nil {
		t.Logf("could not cancel search %s: %v", queryID, err)
	}
}

// pageEventRows counts the event rows on a page.
func pageEventRows(page *SearchPoll) int {
	rows := 0
	for _, item := range page.Results {
		if item.Type == "events" {
			rows += len(item.Rows)
		}
	}
	return rows
}

// searchE2EEnvInt reads a positive integer knob, falling back to its default.
func searchE2EEnvInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	require.NoError(t, err, "%s must be an integer, got %q", name, raw)
	require.Greater(t, v, 0, "%s must be positive", name)
	return v
}

// searchE2EWindow returns the time range to search, ending now.
func searchE2EWindow(t *testing.T) (start, end int64) {
	t.Helper()
	hours := searchE2EEnvInt(t, searchE2ELookbackEnv, defaultSearchE2ELookbackHours)
	now := time.Now()
	return now.Add(-time.Duration(hours) * time.Hour).Unix(), now.Unix()
}

// appliedSearchMode returns the mode a page reports having run as.
//
// The mode is reported per result item, so this also checks that the items of
// one page agree: a single page running partly as one mode and partly as
// another would not be a meaningful answer to "what did this run as".
func appliedSearchMode(t *testing.T, page *SearchPoll) SearchMode {
	t.Helper()
	found := SearchMode("")
	for i, item := range page.Results {
		if item.Stats.SearchMode == "" {
			continue
		}
		if found == "" {
			found = item.Stats.SearchMode
			continue
		}
		require.Equal(t, found, item.Stats.SearchMode,
			"result item %d reports a different applied mode than an earlier item on the same page", i)
	}
	return found
}

// requireAppliedSearchMode asserts a paginated page says what it ran as.
//
// A page with no events and no reported mode means the window held nothing to
// page through, which says nothing about the SDK or the server, so the test
// skips with that reason rather than failing. A page with events must report a
// mode.
func requireAppliedSearchMode(t *testing.T, page *SearchPoll) SearchMode {
	t.Helper()
	mode := appliedSearchMode(t, page)
	if mode == "" && pageEventRows(page) == 0 {
		t.Skipf("the first page holds no events and reports no mode, so the window has nothing to page through; "+
			"widen %s or point the test at an organization with recent telemetry", searchE2ELookbackEnv)
	}
	require.NotEmpty(t, mode,
		"a paginated page must report the mode it ran as in stats.searchMode; an empty value means either that the "+
			"deployment predates the field or that the search did not paginate")
	_, known := knownAppliedSearchModes[mode]
	require.True(t, known,
		"page reports applied mode %q, which this build does not know. If a new mode has shipped, add it to "+
			"knownAppliedSearchModes rather than removing this check.", mode)
	return mode
}

// requireWholeRows asserts the events on a page arrived intact.
//
// It checks the event body rather than the metadata: a page cut short mid-row
// is what this is looking for, and the event under "data" is the part that
// would be missing. Metadata shape varies with the result projection, so it is
// deliberately not asserted here.
func requireWholeRows(t *testing.T, page *SearchPoll) int {
	t.Helper()
	rows := 0
	for _, item := range page.Results {
		if item.Type != "events" {
			continue
		}
		for i, row := range item.Rows {
			require.NotEmpty(t, row, "row %d came back empty", i)
			data, ok := row["data"]
			require.True(t, ok, "row %d has no data; a whole row carries the event under \"data\"", i)
			event, ok := data.(map[string]interface{})
			require.True(t, ok, "row %d has a data field that is not an object: %T", i, data)
			require.NotEmpty(t, event, "row %d carries an empty event", i)
			rows++
		}
	}
	return rows
}

// runSearchE2EFirstPage submits a paginated search and returns its first
// completed page. The search holds a slot from submission until it is
// cancelled in the test's cleanup, so it does not sit holding a concurrency
// slot for the rest of its life.
func runSearchE2EFirstPage(t *testing.T, env *searchE2EEnv, req SearchRequest) *SearchPoll {
	t.Helper()
	ctx, cancel := searchE2EContext(t)
	defer cancel()

	release := env.acquire(t, ctx)
	queryID, err := env.org.InitiateSearchWithContext(ctx, req)
	if err != nil {
		release()
	}
	require.NoError(t, err, "submitting the search failed")
	require.NotEmpty(t, queryID)
	// Cleanups run last-in first-out: cancel, then give the slot back.
	t.Cleanup(release)
	t.Cleanup(func() { cancelSearchE2E(t, env.org, queryID) })

	page, err := env.org.FetchSearchPage(ctx, queryID, "", SearchExecuteOptions{})
	require.NoError(t, err, "fetching the first page failed")
	require.True(t, page.Completed)
	if page.Partial != nil {
		t.Logf("page is partial, reasons %v", page.Partial.Reasons)
	}
	return page
}

// TestSearchE2EFirstPageReportsAppliedMode submits one search per way of
// setting the mode and asserts each first page reports a mode it ran as. The
// three are independent and run in parallel.
//
// It does not assert that the answer equals what was requested: the server
// decides, and this test does not control the organization's configuration.
func TestSearchE2EFirstPageReportsAppliedMode(t *testing.T) {
	t.Parallel()
	env := newSearchE2EEnv(t)
	start, end := searchE2EWindow(t)
	base := SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end}

	// The one that matters most: the mode this SDK fills in for a caller who
	// sets none reaches a real server and comes back applied. The same page
	// is a paginated one, so it must also report the two limits that decide
	// where it stopped; they are only meaningful together with the mode.
	t.Run("unset mode submits batch", func(t *testing.T) {
		t.Parallel()
		page := runSearchE2EFirstPage(t, env, base)
		mode := requireAppliedSearchMode(t, page)

		var stats SearchStats
		for _, item := range page.Results {
			if item.Stats.SearchMode != "" {
				stats = item.Stats
				break
			}
		}
		require.Greater(t, stats.PageSize, int64(0),
			"a paginated page must report the event count it was allowed to return in stats.pageSize")
		require.Greater(t, stats.PaginatedByteCap, int64(0),
			"a paginated page must report the byte ceiling it was allowed to return in stats.paginatedByteCap")
		t.Logf("submitted no mode (so %q went on the wire), page ran as %q; pageSize %d, paginatedByteCap %d, "+
			"eventsScanned %d", SearchModeBatch, mode, stats.PageSize, stats.PaginatedByteCap, stats.EventsScanned)
	})

	t.Run("explicit interactive", func(t *testing.T) {
		t.Parallel()
		page := runSearchE2EFirstPage(t, env, base.WithMode(SearchModeInteractive))
		mode := requireAppliedSearchMode(t, page)
		t.Logf("submitted %q, page ran as %q", SearchModeInteractive, mode)
	})

	// No mode key at all, the request an SDK without the field would send.
	// Which mode applies is the organization's configuration to decide, so it
	// is logged rather than asserted.
	t.Run("without mode leaves it to the organization", func(t *testing.T) {
		t.Parallel()
		page := runSearchE2EFirstPage(t, env, base.WithoutMode())
		mode := requireAppliedSearchMode(t, page)
		t.Logf("submitted no mode key, page ran as %q", mode)
	})
}

// TestSearchE2EPaginationIsConsistentAcrossPages walks a paginated search and
// asserts the mode does not change under it and the rows arrive whole.
//
// A search is submitted once and every page after the first is a token-only
// request, so the mode cannot change between pages. This is the check that the
// server holds up its end of that.
//
// One page is a legitimate result for a quiet organization or a short window:
// the invariants asserted here hold for any number of pages, and the page
// count is logged so a run that never exercised pagination is visible rather
// than silently passing as if it had. Widen LC_SEARCH_E2E_LOOKBACK_HOURS to
// get past a single page.
func TestSearchE2EPaginationIsConsistentAcrossPages(t *testing.T) {
	t.Parallel()
	env := newSearchE2EEnv(t)
	start, end := searchE2EWindow(t)
	maxPages := searchE2EEnvInt(t, searchE2EMaxPagesEnv, defaultSearchE2EMaxPages)

	ctx, cancel := searchE2EContext(t)
	defer cancel()
	release := env.acquire(t, ctx)
	defer release()

	pages, rows := 0, 0
	firstMode := SearchMode("")
	reachedEnd := false

	// Stopping at the page cap leaves pages unread, and ExecuteSearch
	// releases the search itself when that happens, including when an
	// assertion below stops the test inside the handler.
	err := env.org.ExecuteSearch(ctx, SearchRequest{
		Query:     searchE2EQuery,
		StartTime: start,
		EndTime:   end,
	}, SearchExecuteOptions{}, func(page *SearchPoll) (bool, error) {
		pages++
		if page.NextToken() == "" {
			reachedEnd = true
		}
		// Past the first page, an empty page without a mode is the tail of
		// the window rather than a page that ran, so it has nothing to agree
		// with the others about.
		if pages > 1 && pageEventRows(page) == 0 && appliedSearchMode(t, page) == "" {
			return pages < maxPages, nil
		}
		mode := requireAppliedSearchMode(t, page)
		if firstMode == "" {
			firstMode = mode
		}
		require.Equal(t, firstMode, mode,
			"page %d ran as %q where page 1 ran as %q; a search is submitted once, so its mode cannot change between pages",
			pages, mode, firstMode)
		rows += requireWholeRows(t, page)
		return pages < maxPages, nil
	})
	require.NoError(t, err)

	require.GreaterOrEqual(t, pages, 1)
	if pages == 1 {
		t.Logf("the search completed in a single page, so this run did not exercise pagination; "+
			"raise %s to widen the window", searchE2ELookbackEnv)
	}
	t.Logf("walked %d page(s) as %q, %d row(s), reached the end: %v", pages, firstMode, rows, reachedEnd)
}

// searchE2ESettledWindow is a one-hour window that ended a day ago. Two
// searches over it see the same data: telemetry that arrives late, such as a
// sensor uploading what it buffered while offline, has had a day to land, so it
// cannot reach one run and not the other, which is what a test comparing two
// runs needs.
func searchE2ESettledWindow() (start, end int64) {
	settled := time.Now().Add(-24 * time.Hour)
	return settled.Add(-time.Hour).Unix(), settled.Unix()
}

// searchE2ERows is what one cross-mode run collected.
type searchE2ERows struct {
	rows       []string
	reachedEnd bool
}

// collectSearchE2ERows runs one search to the end, or until it has returned
// maxRows rows, and returns every event row it delivered, each serialized so
// two runs can be compared exactly. reachedEnd reports whether the search
// finished rather than being stopped at the cap.
func collectSearchE2ERows(t *testing.T, env *searchE2EEnv, req SearchRequest, maxRows int) searchE2ERows {
	t.Helper()
	ctx, cancel := searchE2EContext(t)
	defer cancel()
	release := env.acquire(t, ctx)
	defer release()

	var out searchE2ERows
	err := env.org.ExecuteSearch(ctx, req, SearchExecuteOptions{}, func(page *SearchPoll) (bool, error) {
		for _, item := range page.Results {
			if item.Type != "events" {
				continue
			}
			for _, row := range item.Rows {
				b, err := json.Marshal(row)
				require.NoError(t, err)
				out.rows = append(out.rows, string(b))
			}
		}
		if page.NextToken() == "" {
			out.reachedEnd = true
		}
		return len(out.rows) < maxRows, nil
	})
	require.NoError(t, err)
	return out
}

// TestSearchE2EBothModesReturnTheSameRows asserts the property the mode rests
// on: it moves where one page ends and the next begins, never which rows come
// back. The two runs are independent and execute in parallel.
//
// The two result sets are compared as multisets, each row serialized whole.
// Order is deliberately not compared: events that share a timestamp have no
// order the API promises, and different page boundaries, which is exactly what
// the two modes produce, may legitimately list them differently.
//
// Only complete result sets are comparable. A run stopped at the row cap holds
// a prefix of its result set whose boundary depends on the page boundaries, so
// when either run is cut off the test skips, saying so, rather than comparing
// two prefixes that need not match.
func TestSearchE2EBothModesReturnTheSameRows(t *testing.T) {
	t.Parallel()
	env := newSearchE2EEnv(t)
	start, end := searchE2ESettledWindow()
	const maxRows = 5000
	req := SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end}

	var batch, interactive searchE2ERows
	// The group returns once both parallel subtests have finished.
	t.Run("collect", func(t *testing.T) {
		t.Run("batch", func(t *testing.T) {
			t.Parallel()
			batch = collectSearchE2ERows(t, env, req.WithMode(SearchModeBatch), maxRows)
		})
		t.Run("interactive", func(t *testing.T) {
			t.Parallel()
			interactive = collectSearchE2ERows(t, env, req.WithMode(SearchModeInteractive), maxRows)
		})
	})
	if t.Failed() {
		return
	}

	if len(batch.rows) == 0 && len(interactive.rows) == 0 {
		t.Skipf("the settled window holds no events for this organization, so there is nothing to compare")
	}
	if !batch.reachedEnd || !interactive.reachedEnd {
		t.Skipf("the settled window holds more than %d events (batch reached the end: %v, interactive: %v), so "+
			"neither run holds a complete result set to compare; point the test at a quieter organization",
			maxRows, batch.reachedEnd, interactive.reachedEnd)
	}

	require.Equal(t, len(batch.rows), len(interactive.rows),
		"the two modes returned a different number of rows for the same settled window")
	sort.Strings(batch.rows)
	sort.Strings(interactive.rows)
	for i := range batch.rows {
		require.Equal(t, batch.rows[i], interactive.rows[i],
			"the two modes returned different rows for the same settled window; first difference in sorted position %d", i)
	}
	t.Logf("compared %d row(s) as multisets; both runs reached the end", len(batch.rows))
}

// TestSearchE2EValidateReportsBothOutcomes asserts that validation reports a
// good query as valid and a malformed one through the result's Error field,
// not as a failed call: a caller must be able to show the reason to a user.
// Validation runs no search and holds no slot, so the two cases run in
// parallel without taking one.
func TestSearchE2EValidateReportsBothOutcomes(t *testing.T) {
	t.Parallel()
	env := newSearchE2EEnv(t)
	start, end := searchE2EWindow(t)

	t.Run("valid query", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := searchE2EContext(t)
		defer cancel()
		v, err := env.org.ValidateSearchWithContext(ctx, SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end})
		require.NoError(t, err)
		require.Empty(t, v.Error, "a valid query was reported invalid")
	})
	t.Run("malformed query", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := searchE2EContext(t)
		defer cancel()
		v, err := env.org.ValidateSearchWithContext(ctx, SearchRequest{Query: "* | * | event/FILE_PATH ==== (((", StartTime: start, EndTime: end})
		require.NoError(t, err, "a malformed query is a validation outcome, not a failed call")
		require.NotEmpty(t, v.Error, "a malformed query was reported valid")
		t.Logf("validation error: %s", v.Error)
	})
}

// TestSearchE2ECancelEndsTheSearch asserts that a cancelled search is gone
// server-side: polling it fails with 404, so it holds no concurrency slot and
// bills nothing further.
//
// The poll after the cancel is repeated with a short, bounded back-off rather
// than asserted once, so a server that completes a cancel asynchronously is
// not reported as a failure while the cancel is still landing.
func TestSearchE2ECancelEndsTheSearch(t *testing.T) {
	t.Parallel()
	env := newSearchE2EEnv(t)
	start, end := searchE2EWindow(t)

	ctx, cancel := searchE2EContext(t)
	defer cancel()
	release := env.acquire(t, ctx)
	defer release()

	queryID, err := env.org.InitiateSearchWithContext(ctx, SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end})
	require.NoError(t, err)
	require.NotEmpty(t, queryID)

	require.NoError(t, env.org.CancelSearchWithContext(ctx, queryID), "cancelling a running search failed")

	pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
	defer pollCancel()
	delay := 250 * time.Millisecond
	for attempt := 1; ; attempt++ {
		_, err = env.org.PollSearchWithContext(pollCtx, queryID, "")
		var restErr RESTError
		if errors.As(err, &restErr) && restErr.StatusCode() == http.StatusNotFound {
			t.Logf("poll %d after cancel: %v", attempt, err)
			return
		}
		require.NoError(t, waitForRetry(pollCtx, delay),
			"a cancelled search could still be polled 30s after the cancel; last poll error: %v", err)
		if delay < 2*time.Second {
			delay *= 2
		}
	}
}
