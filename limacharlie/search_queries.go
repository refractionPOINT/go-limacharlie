package limacharlie

// Open-search listing for LimaCharlie.
//
// A search is "open" from the moment it is submitted until it finishes, is
// cancelled, or its state expires. That is not the same population as the
// searches consuming the organization's concurrency limit: a paginated search
// sitting between pages is open and resumable but holds no slot. The listing
// reports both numbers separately, which is what makes "why is my organization
// at its limit" answerable.
//
// Served from the search host resolved from the organization's URL map (the
// "search" key), not from the main API host.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// SearchQueryState describes what an open search is doing, and by extension
// whether it counts against the organization's concurrency limit.
type SearchQueryState string

const (
	// SearchQueryExecuting means a page of this search is running. Counts
	// against the limit.
	SearchQueryExecuting SearchQueryState = "executing"
	// SearchQueryQueued means a slot is held but the work has not been
	// delivered to a worker yet. Counts against the limit; seeing many of
	// these is what a dispatch backlog looks like.
	SearchQueryQueued SearchQueryState = "queued"
	// SearchQueryIdle means the search is open and resumable with nothing
	// running. Does NOT count against the limit.
	SearchQueryIdle SearchQueryState = "idle"
	// SearchQueryUnknown means slot state is not being tracked, so whether
	// this search consumes a slot cannot be determined. Reported rather than
	// guessing "idle", which would read as "consumes nothing".
	SearchQueryUnknown SearchQueryState = "unknown"
)

// OpenSearchQuery is one search an organization currently has open.
type OpenSearchQuery struct {
	QueryID string           `json:"queryId"`
	State   SearchQueryState `json:"state"`
	// HoldsSlot reports whether this search is counted against the
	// concurrency limit. Nil only when State is SearchQueryUnknown; an idle
	// search is always false.
	HoldsSlot *bool `json:"holdsSlot"`
	// Page identifies which page holds the slot, for a paginated search past
	// its first. It is a digest of the pagination cursor, never the cursor.
	Page string `json:"page,omitempty"`

	// Query, Stream, StartTime and EndTime are echoed as submitted. Absent
	// when the search's record has expired. Query is shortened by the server
	// past a length bound, marked with a trailing "...".
	Query     string `json:"query,omitempty"`
	Stream    string `json:"stream,omitempty"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	// SubmittedBy is the authenticated identity that submitted the search and
	// UserAgent the client it arrived from.
	SubmittedBy string `json:"submittedBy,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"`

	SubmittedAt string `json:"submittedAt"`
	// StartedAt and RunningForMs describe the currently executing page, so
	// both are absent for an idle search. RunningForMs is computed server-side
	// rather than left to the caller to subtract.
	StartedAt    string `json:"startedAt,omitempty"`
	RunningForMs *int64 `json:"runningForMs,omitempty"`
	// LastActivityAt is when the server most recently finished producing a
	// page; absent until one has. It does not move when a client merely polls.
	LastActivityAt string `json:"lastActivityAt,omitempty"`
	// PagesCompleted counts pages successfully produced, not pages a client
	// collected. A failed page is not counted, and a page still executing
	// counts only once it completes.
	PagesCompleted int `json:"pagesCompleted"`
	// HasMorePages is set only once a page has completed, because that is when
	// the answer is known. Nil means "not yet determined", not "no more pages".
	HasMorePages *bool `json:"hasMorePages,omitempty"`

	// BatchesCompleted and BatchesInScope are the progress pair. BatchesInScope
	// is 0 when the scope estimate was unavailable, which means progress cannot
	// be computed rather than that nothing has been done.
	//
	// Both advance at page boundaries, so a search that returns everything in
	// one page - notably any aggregation, which does not paginate - reports 0
	// for its entire life and then leaves the listing.
	BatchesCompleted uint64 `json:"batchesCompleted"`
	BatchesInScope   uint64 `json:"batchesInScope"`
	// ProgressPercent is the pair above as a percentage, clamped to [0, 100].
	// Nil when there is no denominator to divide by.
	ProgressPercent *float64 `json:"progressPercent,omitempty"`
	// EventsScanned is the total scanned so far and BilledEvents the charged
	// portion of it, both cumulative across pages. In practice these are the
	// most actionable fields: the search worth cancelling is the one that has
	// scanned the most.
	EventsScanned uint64 `json:"eventsScanned"`
	BilledEvents  uint64 `json:"billedEvents"`

	// SlotExpiresAt is when the concurrency slot is reclaimed if the search
	// neither finishes nor is cancelled. Absent when no slot is held.
	SlotExpiresAt string `json:"slotExpiresAt,omitempty"`
	// ResumableUntil is when the search's state expires and it can no longer
	// be resumed - the answer to "how long may a user leave a search paused".
	// Unrelated to SlotExpiresAt, which is why they are separate fields.
	ResumableUntil string `json:"resumableUntil"`
}

// OpenSearchQueries is one page of an organization's open searches.
type OpenSearchQueries struct {
	OID string `json:"oid"`
	// Limit is the organization's maximum concurrent searches.
	Limit int `json:"limit"`
	// SlotsHeld is how many of those are in use. This, not Count, is what
	// Limit applies to: idle searches are open but consume nothing.
	SlotsHeld int64 `json:"slotsHeld"`
	// Count is how many searches are open in total.
	Count int64 `json:"count"`
	// Truncated reports that more entries exist past this page. It is computed
	// against the index page, before any State filter, so a caller filtering to
	// idle can legitimately see it set alongside an empty Queries slice.
	Truncated bool              `json:"truncated"`
	Queries   []OpenSearchQuery `json:"queries"`
}

// OpenSearchQueriesFilters are the optional filters and paging options for
// ListOpenQueries. Zero values are omitted from the request.
type OpenSearchQueriesFilters struct {
	// State selects which searches to return: "all" (the default), "executing"
	// (a page is running, or a slot is held and the work has not reached a
	// worker yet) or "idle" (open and resumable, consuming no slot).
	State string
	// Limit is the page size. The server defaults to 50 and clamps to 200.
	Limit int
	// Offset walks the listing, newest first.
	Offset int
}

// validSearchQueryStates are what the server accepts for the State filter.
// Checked client-side so a typo is a clear local error rather than a 400.
var validSearchQueryStates = map[string]struct{}{
	"":          {},
	"all":       {},
	"executing": {},
	"idle":      {},
}

// ListOpenQueries lists the searches this organization currently has open.
//
// Reach for this when a search is refused for concurrency: the response names
// what is holding the slots, who submitted each one, how long it has been
// running and how much it has scanned, so the search worth cancelling is
// identifiable. Cancel one with the search API's delete endpoint.
//
// Example:
//
//	open, err := org.ListOpenQueries(OpenSearchQueriesFilters{State: "executing"})
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("%d of %d slots in use\n", open.SlotsHeld, open.Limit)
//	for _, q := range open.Queries {
//	    fmt.Println(q.QueryID, q.SubmittedBy, q.EventsScanned)
//	}
func (org *Organization) ListOpenQueries(filters OpenSearchQueriesFilters) (*OpenSearchQueries, error) {
	return org.ListOpenQueriesWithContext(context.Background(), filters)
}

// ListOpenQueriesWithContext is ListOpenQueries with a context for
// cancellation and deadlines.
func (org *Organization) ListOpenQueriesWithContext(ctx context.Context, filters OpenSearchQueriesFilters) (*OpenSearchQueries, error) {
	state := strings.ToLower(strings.TrimSpace(filters.State))
	if _, ok := validSearchQueryStates[state]; !ok {
		return nil, fmt.Errorf("invalid state %q: must be one of all, executing, idle", filters.State)
	}
	if state == "" {
		state = "all"
	}

	root, err := org.getServiceRoot("search")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve search service root: %w", err)
	}

	qp := Dict{"state": state}
	if filters.Limit != 0 {
		qp["limit"] = strconv.Itoa(filters.Limit)
	}
	if filters.Offset != 0 {
		qp["offset"] = strconv.Itoa(filters.Offset)
	}

	resp := OpenSearchQueries{}
	req := makeDefaultRequest(&resp).withURLRoot(root).withQueryData(qp)
	// The organization is named in the path rather than as a parameter: the
	// listing is per-organization and is never addressable without one.
	path := "/v1/search/" + org.GetOID() + "/queries"
	if err := org.client.reliableRequest(ctx, http.MethodGet, path, req); err != nil {
		return nil, fmt.Errorf("failed to list open queries: %w", err)
	}
	return &resp, nil
}

// ListAllOpenQueries walks every page of the listing and returns the whole set.
//
// An organization can open and finish searches while this runs, so treat the
// result as a sample rather than a transaction; ListOpenQueries reports Count
// if an exact total at one instant is what is needed. Entries are de-duplicated
// by query id, because a search leaving the index mid-walk shifts later offsets
// back and can re-serve one already returned.
func (org *Organization) ListAllOpenQueries(ctx context.Context, state string) ([]OpenSearchQuery, error) {
	const pageSize = 200

	out := []OpenSearchQuery{}
	seen := map[string]struct{}{}
	for offset := 0; ; offset += pageSize {
		page, err := org.ListOpenQueriesWithContext(ctx, OpenSearchQueriesFilters{
			State:  state,
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		for _, q := range page.Queries {
			if _, dup := seen[q.QueryID]; dup {
				continue
			}
			seen[q.QueryID] = struct{}{}
			out = append(out, q)
		}
		// Truncated describes the index page rather than the filtered rows, so
		// it - not the length of Queries - is what says whether to continue: the
		// State filter is applied after paging, so a page can come back empty
		// while more entries exist.
		if !page.Truncated {
			return out, nil
		}
	}
}

// SearchConcurrencyLimits describes how many searches an organization may run at
// once.
type SearchConcurrencyLimits struct {
	// MaxConcurrentQueries is how many searches may be EXECUTING at the same
	// time. A submission past it is refused with 429.
	//
	// A paginated search parked between pages consumes nothing, so this is not a
	// cap on how many searches may be open. ListOpenQueries reports both numbers.
	MaxConcurrentQueries int `json:"maxConcurrentQueries"`
}

// SearchPaginationLimits describes the shape of one page of results.
type SearchPaginationLimits struct {
	// ResultsPerPage is the maximum events one page returns before the server
	// hands back a continuation token.
	ResultsPerPage int64 `json:"resultsPerPage"`
	// MaxPageDurationSeconds is how long the server spends producing one page
	// before cutting it short and returning a continuation token. Reaching it is
	// normal for a broad query and does not mean the search failed.
	MaxPageDurationSeconds int64 `json:"maxPageDurationSeconds"`
	// MaxCursorBytes is the largest continuation token the server accepts back.
	MaxCursorBytes int `json:"maxCursorBytes"`
}

// SearchRetentionLimits describes how long a search's state and results survive.
type SearchRetentionLimits struct {
	// ResumableForSeconds is how long a search can be resumed for, measured from
	// submission and never extended by activity.
	ResumableForSeconds int64 `json:"resumableForSeconds"`
	// PageResultsForSeconds is how long a produced page's results are retained.
	// It can be shorter than ResumableForSeconds, in which case re-reading an
	// older page recomputes it rather than failing - so this is a latency
	// characteristic, not a deadline.
	PageResultsForSeconds int64 `json:"pageResultsForSeconds"`
}

// SearchExecutionLimits describes the bounds on a single search's execution.
//
// Every field is nil when the limit is not enforced. Nil rather than zero is
// load-bearing: in a set of limits a zero would read as "nothing allowed".
type SearchExecutionLimits struct {
	// MaxQueryDurationSeconds is the deadline for a non-aggregation search.
	MaxQueryDurationSeconds *int64 `json:"maxQueryDurationSeconds"`
	// MaxAggregationDurationSeconds is the deadline for an aggregation, which
	// gets a separate budget because it cannot return partial pages.
	MaxAggregationDurationSeconds *int64 `json:"maxAggregationDurationSeconds"`
	// MaxResponseBytes is the ceiling on a single response's accumulated size.
	MaxResponseBytes *int64 `json:"maxResponseBytes"`
}

// SearchRequestLimits describes the bounds on a request itself.
type SearchRequestLimits struct {
	// MaxRequestBodyBytes is the largest body the search endpoints accept. In
	// practice this bounds query length and how many sensors a query may name.
	MaxRequestBodyBytes int `json:"maxRequestBodyBytes"`
}

// SearchCapabilities reports optional features, so a client can decide what to
// offer rather than probing an endpoint and interpreting the failure.
type SearchCapabilities struct {
	// OpenQueryListing says whether ListOpenQueries reports searches that are
	// open but idle. When false it still answers, but only sees searches
	// currently holding a concurrency slot.
	OpenQueryListing bool `json:"openQueryListing"`
}

// SearchLimits is an organization's resolved search limits.
//
// Fields are additive: ignore ones you do not recognise, and treat an absent one
// as "not applicable to this deployment" rather than as zero.
type SearchLimits struct {
	OID          string                  `json:"oid"`
	Concurrency  SearchConcurrencyLimits `json:"concurrency"`
	Pagination   SearchPaginationLimits  `json:"pagination"`
	Retention    SearchRetentionLimits   `json:"retention"`
	Execution    SearchExecutionLimits   `json:"execution"`
	Request      SearchRequestLimits     `json:"request"`
	Capabilities SearchCapabilities      `json:"capabilities"`
}

// GetSearchLimits reports this organization's resolved search limits.
//
// Every limit here is otherwise discoverable only by hitting it: a refusal for
// concurrency does not say what the cap was, and a paginated search stops being
// resumable with no way to have known the window. Read this once and size the
// client's behaviour to it.
//
// Example:
//
//	limits, err := org.GetSearchLimits()
//	if err != nil {
//	    return err
//	}
//	sem := make(chan struct{}, limits.Concurrency.MaxConcurrentQueries)
//	if d := limits.Execution.MaxQueryDurationSeconds; d != nil {
//	    fmt.Printf("queries are cut off after %ds\n", *d)
//	}
func (org *Organization) GetSearchLimits() (*SearchLimits, error) {
	return org.GetSearchLimitsWithContext(context.Background())
}

// GetSearchLimitsWithContext is GetSearchLimits with a context for cancellation
// and deadlines.
func (org *Organization) GetSearchLimitsWithContext(ctx context.Context) (*SearchLimits, error) {
	root, err := org.getServiceRoot("search")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve search service root: %w", err)
	}

	resp := SearchLimits{}
	req := makeDefaultRequest(&resp).withURLRoot(root)
	// The organization is named in the path rather than as a parameter: limits
	// are per-organization and are never addressable without one.
	path := "/v1/search/" + org.GetOID() + "/limits"
	if err := org.client.reliableRequest(ctx, http.MethodGet, path, req); err != nil {
		return nil, fmt.Errorf("failed to get search limits: %w", err)
	}
	return &resp, nil
}
