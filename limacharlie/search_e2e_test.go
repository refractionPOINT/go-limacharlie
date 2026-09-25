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
// with telemetry in the last day. Setting the variable is what says both are
// intended.
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
//	LC_SEARCH_E2E_LOOKBACK_HOURS   how far back to search (default 24)
//	LC_SEARCH_E2E_MAX_PAGES        pages to walk before stopping (default 3)
//
// Widen the lookback on a quiet organization to get past a single page, and
// narrow it on a busy one to keep the scan cheap. Every search these tests
// start is cancelled server-side when they stop early, so none is left holding
// a concurrency slot.
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
	"os"
	"strconv"
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

	defaultSearchE2ELookbackHours = 24
	defaultSearchE2EMaxPages      = 3

	// searchE2EBudget bounds one test. A live page can take minutes on a broad
	// query, and the point of the bound is to fail rather than hang.
	searchE2EBudget = 5 * time.Minute

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

// newSearchE2EOrg returns an Organization pointed at a live deployment, or
// skips the test.
//
// It SKIPS rather than fails when the environment is absent. The shared
// fixture in test_fixture.go calls FailNow for a missing _OID, which is why a
// checkout without credentials reports failures rather than skips; these tests
// must not add to that.
func newSearchE2EOrg(t *testing.T) *Organization {
	t.Helper()

	if os.Getenv(searchE2EGateEnv) == "" {
		t.Skipf("skipping Search API end-to-end test: set %s=1, plus _OID and _KEY for a live organization with telemetry "+
			"in the last day, to run it. It is opt-in rather than credential-gated because it starts real, billed searches.",
			searchE2EGateEnv)
	}

	oid, key := os.Getenv("_OID"), os.Getenv("_KEY")
	if oid == "" || key == "" {
		t.Skipf("skipping Search API end-to-end test: %s is set but _OID and/or _KEY are not, and both are needed to "+
			"authenticate against a live organization.", searchE2EGateEnv)
	}

	org, err := NewOrganizationFromClientOptions(ClientOptions{OID: oid, APIKey: key}, &LCLoggerGCP{})
	require.NoError(t, err, "could not build an organization from _OID and _KEY")
	return org
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
func requireAppliedSearchMode(t *testing.T, page *SearchPoll) SearchMode {
	t.Helper()
	mode := appliedSearchMode(t, page)
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
// completed page, cancelling the search afterwards so it does not sit holding
// a concurrency slot for the rest of its life.
func runSearchE2EFirstPage(t *testing.T, org *Organization, req SearchRequest) *SearchPoll {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), searchE2EBudget)
	defer cancel()

	queryID, err := org.InitiateSearchWithContext(ctx, req)
	require.NoError(t, err, "submitting the search failed")
	require.NotEmpty(t, queryID)
	t.Cleanup(func() {
		if err := org.CancelSearch(queryID); err != nil {
			t.Logf("could not cancel search %s: %v", queryID, err)
		}
	})

	page, err := org.FetchSearchPage(ctx, queryID, "", SearchExecuteOptions{})
	require.NoError(t, err, "fetching the first page failed")
	require.True(t, page.Completed)
	if page.Partial != nil {
		t.Logf("page is partial, reasons %v", page.Partial.Reasons)
	}
	return page
}

// TestSearchE2EDefaultModeIsApplied is the one that matters most: it proves the
// mode this SDK fills in for a caller who sets none reaches a real server and
// comes back applied, rather than being silently dropped somewhere in between.
func TestSearchE2EDefaultModeIsApplied(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)

	// No mode set, which is what most callers will do and what submits batch.
	page := runSearchE2EFirstPage(t, org, SearchRequest{
		Query:     searchE2EQuery,
		StartTime: start,
		EndTime:   end,
	})

	mode := requireAppliedSearchMode(t, page)
	t.Logf("submitted no mode (so %q went on the wire), page ran as %q", SearchModeBatch, mode)
}

// TestSearchE2EInteractiveModeIsApplied submits the other mode explicitly and
// asserts a page still reports what it ran as.
//
// It does not assert that the answer is interactive. The server decides, and
// this test does not control the organization's configuration.
func TestSearchE2EInteractiveModeIsApplied(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)

	page := runSearchE2EFirstPage(t, org, SearchRequest{
		Query:     searchE2EQuery,
		StartTime: start,
		EndTime:   end,
	}.WithMode(SearchModeInteractive))

	mode := requireAppliedSearchMode(t, page)
	t.Logf("submitted %q, page ran as %q", SearchModeInteractive, mode)
}

// TestSearchE2EPaginatedPageReportsItsLimits asserts a paginated page reports
// the two limits that decide where it stopped, alongside the mode.
//
// These are the three fields a client needs to explain a page that came back
// smaller than expected, and they are only meaningful together.
func TestSearchE2EPaginatedPageReportsItsLimits(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)

	page := runSearchE2EFirstPage(t, org, SearchRequest{
		Query:     searchE2EQuery,
		StartTime: start,
		EndTime:   end,
	})

	require.NotEmpty(t, page.Results, "a completed page should carry at least one result item")
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
	t.Logf("applied mode %q, pageSize %d, paginatedByteCap %d, eventsScanned %d",
		mode, stats.PageSize, stats.PaginatedByteCap, stats.EventsScanned)
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
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)
	maxPages := searchE2EEnvInt(t, searchE2EMaxPagesEnv, defaultSearchE2EMaxPages)

	ctx, cancel := context.WithTimeout(context.Background(), searchE2EBudget)
	defer cancel()

	var queryID string
	pages, rows := 0, 0
	firstMode := SearchMode("")
	reachedEnd := false

	err := org.ExecuteSearch(ctx, SearchRequest{
		Query:     searchE2EQuery,
		StartTime: start,
		EndTime:   end,
	}, SearchExecuteOptions{
		OnQueryInitiated: func(id string) { queryID = id },
	}, func(page *SearchPoll) (bool, error) {
		pages++
		mode := requireAppliedSearchMode(t, page)
		if firstMode == "" {
			firstMode = mode
		}
		require.Equal(t, firstMode, mode,
			"page %d ran as %q where page 1 ran as %q; a search is submitted once, so its mode cannot change between pages",
			pages, mode, firstMode)
		rows += requireWholeRows(t, page)
		if page.NextToken() == "" {
			reachedEnd = true
		}
		return pages < maxPages, nil
	})
	require.NoError(t, err)

	// Stopping at the page cap leaves the search open, so release its slot.
	if !reachedEnd && queryID != "" {
		if err := org.CancelSearch(queryID); err != nil {
			t.Logf("could not cancel search %s: %v", queryID, err)
		}
	}

	require.GreaterOrEqual(t, pages, 1)
	if pages == 1 {
		t.Logf("the search completed in a single page, so this run did not exercise pagination; "+
			"raise %s to widen the window", searchE2ELookbackEnv)
	}
	t.Logf("walked %d page(s) as %q, %d row(s), reached the end: %v", pages, firstMode, rows, reachedEnd)
}

// searchE2ESettledWindow is a window that ended an hour ago. Two searches over
// it see the same data: telemetry still arriving for the last hour cannot land
// in one and not the other, which is what a test comparing two runs needs.
func searchE2ESettledWindow() (start, end int64) {
	settled := time.Now().Add(-time.Hour)
	return settled.Add(-2 * time.Hour).Unix(), settled.Unix()
}

// collectSearchE2ERows runs one search to the end, or to maxRows, and returns
// its event rows in the order the pages delivered them, each serialized so two
// runs can be compared exactly. reachedEnd reports whether the search finished
// rather than being stopped at the cap.
func collectSearchE2ERows(t *testing.T, org *Organization, req SearchRequest, maxRows int) (rows []string, reachedEnd bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), searchE2EBudget)
	defer cancel()

	var queryID string
	err := org.ExecuteSearch(ctx, req, SearchExecuteOptions{
		OnQueryInitiated: func(id string) { queryID = id },
	}, func(page *SearchPoll) (bool, error) {
		for _, item := range page.Results {
			if item.Type != "events" {
				continue
			}
			for _, row := range item.Rows {
				b, err := json.Marshal(row)
				require.NoError(t, err)
				rows = append(rows, string(b))
			}
		}
		if page.NextToken() == "" {
			reachedEnd = true
		}
		return len(rows) < maxRows, nil
	})
	require.NoError(t, err)
	if !reachedEnd && queryID != "" {
		if err := org.CancelSearch(queryID); err != nil {
			t.Logf("could not cancel search %s: %v", queryID, err)
		}
	}
	return rows, reachedEnd
}

// TestSearchE2EBothModesReturnTheSameRows asserts the property the mode rests
// on: it moves where one page ends and the next begins, never which rows come
// back or in what order.
//
// Both runs read the same settled window. Where both reach the end the whole
// sequences must be equal; where the row cap stops one first, the rows both
// runs saw must be equal and in the same order.
func TestSearchE2EBothModesReturnTheSameRows(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2ESettledWindow()
	const maxRows = 20000
	req := SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end}

	batch, batchEnd := collectSearchE2ERows(t, org, req.WithMode(SearchModeBatch), maxRows)
	interactive, interactiveEnd := collectSearchE2ERows(t, org, req.WithMode(SearchModeInteractive), maxRows)
	if len(batch) == 0 && len(interactive) == 0 {
		t.Skipf("the settled window holds no events for this organization, so there is nothing to compare")
	}

	if batchEnd && interactiveEnd {
		require.Equal(t, len(batch), len(interactive), "the two modes returned a different number of rows for the same window")
	}
	common := len(batch)
	if len(interactive) < common {
		common = len(interactive)
	}
	for i := 0; i < common; i++ {
		require.Equal(t, batch[i], interactive[i], "row %d differs between batch and interactive", i)
	}
	t.Logf("compared %d row(s); batch returned %d (end %v), interactive %d (end %v)",
		common, len(batch), batchEnd, len(interactive), interactiveEnd)
}

// TestSearchE2EWithoutModeLeavesItToTheOrganization sends no mode key at all,
// the request an SDK without the field would send, and asserts a page still
// reports a mode it ran as. Which one is the organization's configuration to
// decide, so it is logged rather than asserted.
func TestSearchE2EWithoutModeLeavesItToTheOrganization(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)

	page := runSearchE2EFirstPage(t, org, SearchRequest{
		Query:     searchE2EQuery,
		StartTime: start,
		EndTime:   end,
	}.WithoutMode())

	mode := requireAppliedSearchMode(t, page)
	t.Logf("submitted no mode key, page ran as %q", mode)
}

// TestSearchE2EValidateReportsBothOutcomes asserts that validation reports a
// good query as valid and a malformed one through the result's Error field,
// not as a failed call: a caller must be able to show the reason to a user.
func TestSearchE2EValidateReportsBothOutcomes(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)

	t.Run("valid query", func(t *testing.T) {
		v, err := org.ValidateSearch(SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end})
		require.NoError(t, err)
		require.Empty(t, v.Error, "a valid query was reported invalid")
	})
	t.Run("malformed query", func(t *testing.T) {
		v, err := org.ValidateSearch(SearchRequest{Query: "* | * | event/FILE_PATH ==== (((", StartTime: start, EndTime: end})
		require.NoError(t, err, "a malformed query is a validation outcome, not a failed call")
		require.NotEmpty(t, v.Error, "a malformed query was reported valid")
		t.Logf("validation error: %s", v.Error)
	})
}

// TestSearchE2ECancelEndsTheSearch asserts that a cancelled search is gone
// server-side: it cannot be polled afterwards, so it holds no concurrency slot
// and bills nothing further.
func TestSearchE2ECancelEndsTheSearch(t *testing.T) {
	org := newSearchE2EOrg(t)
	start, end := searchE2EWindow(t)

	queryID, err := org.InitiateSearch(SearchRequest{Query: searchE2EQuery, StartTime: start, EndTime: end})
	require.NoError(t, err)
	require.NotEmpty(t, queryID)

	require.NoError(t, org.CancelSearch(queryID), "cancelling a running search failed")

	_, err = org.PollSearch(queryID, "")
	require.Error(t, err, "a cancelled search could still be polled")
	t.Logf("poll after cancel: %v", err)
}
