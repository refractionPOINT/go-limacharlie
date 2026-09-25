package limacharlie

// Search API submission for LimaCharlie.
//
// A search runs in three steps:
//
//  1. POST /v1/search registers the query and answers with a query id.
//  2. GET /v1/search/{queryId} fetches a page. The server answers
//     completed=false with a suggested poll delay while the page is still
//     being produced, and completed=true once the page is ready.
//  3. A paginated search continues by handing the previous page's nextToken
//     back on the next GET. The search is over when a completed page carries
//     no token.
//
// Only step 1 carries a body. Every continuation is a GET with a token query
// parameter and nothing else, so the criteria submitted once - query, time
// range, stream, pagination and mode - govern every page of the search.
//
// A submission with no mode set asks for SearchModeBatch, on the grounds that
// what calls an SDK is a program reading every page rather than a person
// watching one. SearchRequest.WithMode and SearchRequest.WithoutMode are how to
// ask for anything else.
//
// These calls are served from the search host resolved from the
// organization's URL map (the "search" key), not from the main API host.
// Organization.getServiceRoot handles that resolution, and it is the same one
// ListOpenQueries and GetSearchLimits use.
//
// The three layers here are meant to be composable. InitiateSearch and
// PollSearch are single HTTP calls. FetchSearchPage polls one page to
// completion. ExecuteSearch drives a whole paginated search and hands each
// completed page to a callback. Reach for the lowest layer that does the job:
// a client that persists a query id across processes, or resumes from a stored
// token, wants the primitives rather than the driver.
//
// Organization.Query in query.go is a different, older endpoint on a different
// host and is unrelated to this API.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	// searchRequestTimeout bounds a single Search API HTTP call. A submit or a
	// page fetch can legitimately occupy the server for far longer than the
	// SDK-wide default, so the Search calls raise it to the same ceiling
	// Organization.Query uses.
	searchRequestTimeout = 120 * time.Second

	// defaultSearchPollInterval is the floor applied to the server's suggested
	// poll delay. The server advises a cadence per page and the client honours
	// it; the floor only stops an unusually small or absent suggestion from
	// turning into a busy loop.
	defaultSearchPollInterval = 500 * time.Millisecond

	// fallbackSearchPollInterval is the delay used when a page that is not yet
	// complete suggests no cadence at all.
	fallbackSearchPollInterval = time.Second

	// defaultSearchMaxPollAttempts bounds how many times FetchSearchPage polls
	// a single page before giving up. It is a guard against an unbounded loop,
	// not a deadline: prefer a context with a deadline for that.
	defaultSearchMaxPollAttempts = 300

	// cancelSearchTimeout is the budget for the best-effort cancel issued when
	// a caller's context is cancelled mid-search.
	cancelSearchTimeout = 5 * time.Second
)

// SearchMode declares how the caller intends to CONSUME a search. It is not a
// request for more or less data, and it does not change which events match:
// the same query over the same range returns the same results either way, in
// the same order. Only where one page ends and the next begins moves.
//
// It is sent once, on the submission, and governs every page of that search.
// Continuation requests carry no body and cannot change it.
//
// The server resolves the mode a search runs as in this order: a server-side
// override, then the mode in the submission, then the organization's own
// default. That ordering is why sending SearchModeInteractive and sending no
// mode at all are different requests, not two spellings of one. An explicit
// mode takes precedence over the organization's default; omitting the field is
// what lets that default apply. SearchRequest.WithoutMode is how to omit it.
//
// A server that does not recognise the value ignores it and runs the search
// interactively, so setting a mode is safe against any deployment. For the
// same reason the SDK does not reject unknown values: a mode added later must
// reach the server rather than being refused by an older client.
//
// What actually ran is reported per page in SearchStats.SearchMode, which is
// the field to read - not the one that was requested.
type SearchMode string

const (
	// SearchModeInteractive suits a caller reading pages as they arrive, such
	// as a UI paging through results while someone watches.
	SearchModeInteractive SearchMode = "interactive"

	// SearchModeBatch suits a caller that will consume the whole result set
	// without a person waiting on any individual page, such as an export or a
	// scheduled job. It is what this SDK submits when the caller sets no mode,
	// because a program reading every page pays a fixed cost per round trip and
	// is not the audience interactive is shaped for.
	SearchModeBatch SearchMode = "batch"
)

// SearchRequest is the criteria for one search, sent once on submission.
//
// Query, StartTime and EndTime are required. The rest are optional, and the
// two pointer fields carry three states each: unset, which applies this SDK's
// default, and two explicit choices. Set them through WithPaginated, WithMode
// and WithoutMode rather than taking the address of a local, so that a value
// the caller never chose is never mistaken for one they did.
type SearchRequest struct {
	// Query is the LCQL query string.
	Query string
	// StartTime and EndTime bound the search, in Unix seconds.
	StartTime int64
	EndTime   int64
	// Stream selects which stream to search: "event", "detection" or "audit".
	// Omitted when empty, which lets the server apply its own default rather
	// than the SDK pinning one that can drift from it.
	Stream string
	// Paginated asks the server to break the results into pages joined by a
	// continuation token. Nil means paginated, which is what a client wanting
	// a bounded response per call should use and what ExecuteSearch assumes.
	// Set it through WithPaginated.
	Paginated *bool
	// Mode declares how the caller will consume the search. See SearchMode.
	//
	// Nil, the zero value, submits SearchModeBatch. A pointer to a mode submits
	// that mode. A pointer to the empty SearchMode submits no mode field at
	// all, which is not the same request: it lets the organization's own
	// default apply where any explicit mode would have overridden it.
	//
	// Use WithMode to choose a mode and WithoutMode to omit the field.
	Mode *SearchMode
}

// WithPaginated returns a copy of the request with Paginated set explicitly.
//
// Pass false only for a query whose entire result set is known to be small, or
// one that does not paginate at all such as an aggregation: an unpaginated
// search returns everything in a single response.
func (r SearchRequest) WithPaginated(paginated bool) SearchRequest {
	r.Paginated = &paginated
	return r
}

// WithMode returns a copy of the request that submits the given mode, in place
// of the SearchModeBatch an unset Mode would have submitted.
//
// The empty SearchMode means "send no mode", the same as WithoutMode.
func (r SearchRequest) WithMode(mode SearchMode) SearchRequest {
	r.Mode = &mode
	return r
}

// WithoutMode returns a copy of the request that submits no mode field at all,
// so the organization's own default decides how the search runs.
//
// This is the one way to get that, and it is distinct from submitting
// SearchModeInteractive: an explicit mode overrides the organization's default,
// and an absent one does not. It is also how to restore the exact request an
// SDK with no mode field would have sent.
func (r SearchRequest) WithoutMode() SearchRequest {
	var none SearchMode
	r.Mode = &none
	return r
}

// isPaginated resolves the tri-state Paginated field. Unset means paginated.
func (r SearchRequest) isPaginated() bool {
	return r.Paginated == nil || *r.Paginated
}

// resolveMode resolves the tri-state Mode field to what goes on the wire. Unset
// means SearchModeBatch; the empty SearchMode means the key is omitted, which
// the omitempty tag on searchSubmitBody.Mode then does.
func (r SearchRequest) resolveMode() SearchMode {
	if r.Mode == nil {
		return SearchModeBatch
	}
	return *r.Mode
}

// searchSubmitBody is the wire form of a POST /v1/search body.
//
// Times go out as strings: that is what the endpoint accepts. Stream and Mode
// are omitempty, which is what lets SearchRequest.WithoutMode produce a body
// with no mode key rather than one asserting an empty mode.
type searchSubmitBody struct {
	OID       string     `json:"oid"`
	Query     string     `json:"query"`
	StartTime string     `json:"startTime"`
	EndTime   string     `json:"endTime"`
	Paginated bool       `json:"paginated"`
	Stream    string     `json:"stream,omitempty"`
	Mode      SearchMode `json:"mode,omitempty"`
}

// searchValidateBody is the wire form of a POST /v1/search/validate body.
//
// It is deliberately a subset of searchSubmitBody: validation inspects the
// query and estimates its cost without running it, so pagination and mode -
// both of which describe how results are delivered - do not apply.
type searchValidateBody struct {
	OID       string `json:"oid"`
	Query     string `json:"query"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Stream    string `json:"stream,omitempty"`
}

// buildSearchSubmitBody encodes the submission body for req.
func buildSearchSubmitBody(oid string, req SearchRequest) ([]byte, error) {
	return json.Marshal(searchSubmitBody{
		OID:       oid,
		Query:     req.Query,
		StartTime: strconv.FormatInt(req.StartTime, 10),
		EndTime:   strconv.FormatInt(req.EndTime, 10),
		Paginated: req.isPaginated(),
		Stream:    req.Stream,
		Mode:      req.resolveMode(),
	})
}

// buildSearchValidateBody encodes the validation body for req.
func buildSearchValidateBody(oid string, req SearchRequest) ([]byte, error) {
	return json.Marshal(searchValidateBody{
		OID:       oid,
		Query:     req.Query,
		StartTime: strconv.FormatInt(req.StartTime, 10),
		EndTime:   strconv.FormatInt(req.EndTime, 10),
		Stream:    req.Stream,
	})
}

// SearchStats describes what one page actually did. Every field is absent from
// a response that has nothing to report for it and reads as zero, so treat a
// zero as "not reported" rather than as a measured zero.
//
// Fields are additive. Ignore ones you do not recognise.
type SearchStats struct {
	// SearchMode is the mode the page RAN as, which is the value to trust: a
	// mode the server did not recognise is ignored and the search runs
	// interactively, and only this field says so. Empty on a search that does
	// not paginate, and on a server that does not report it.
	SearchMode SearchMode `json:"searchMode,omitempty"`
	// PageSize is how many events this page was allowed to return and
	// PaginatedByteCap how many bytes, both as resolved by the server for this
	// search. They are the two limits that decide where a page stops, so they
	// explain a page that came back smaller than expected. Both absent, and so
	// zero, for a search that does not paginate.
	PageSize         int64 `json:"pageSize,omitempty"`
	PaginatedByteCap int64 `json:"paginatedByteCap,omitempty"`

	// BytesScanned and EventsScanned are the uncompressed bytes and the events
	// the page read from storage; EventsMatched is how many satisfied the
	// query. Scanned is the workload, matched is the answer, and the ratio
	// between them is what makes a query expensive.
	BytesScanned    uint64 `json:"bytesScanned,omitempty"`
	EventsScanned   uint64 `json:"eventsScanned,omitempty"`
	EventsMatched   uint64 `json:"eventsMatched,omitempty"`
	EventsProcessed uint64 `json:"eventsProcessed,omitempty"`
	RulesEvaluated  uint64 `json:"rulesEvaluated,omitempty"`
	// Walltime is how long the page took, in seconds.
	Walltime float64 `json:"walltime,omitempty"`
	// EstimatedPrice is the cost estimate. Left untyped because its shape is
	// owned by billing and grows independently of this API.
	EstimatedPrice Dict `json:"estimatedPrice,omitempty"`
}

// SearchResultItem is one entry in a page's Results.
//
// The server emits a separate item per result kind, so inspect Type before
// reading the typed fields: "events" carries Rows, and facet and timeline
// results carry Facets and Timeseries respectively.
type SearchResultItem struct {
	SearchResultID string `json:"searchResultId,omitempty"`
	Created        string `json:"created,omitempty"`
	Type           string `json:"type"`
	// Rows are the matched events for an "events" item. Left as Dict because
	// an event's shape is defined by its stream and event type, not by this
	// API. Each row carries the event under "data" and its identity under
	// "mtd".
	Rows       []Dict `json:"rows,omitempty"`
	Facets     []Dict `json:"facets,omitempty"`
	Timeseries []Dict `json:"timeseries,omitempty"`
	// NextToken, when non-empty, is the continuation token for the page after
	// this one. Use SearchPoll.NextToken rather than reading it here: which
	// item of a page carries the token is the server's business.
	NextToken string `json:"nextToken,omitempty"`
	PrevToken string `json:"prevToken,omitempty"`
	// Stats is what producing this item cost and how it was run.
	Stats SearchStats `json:"stats"`
}

// SearchPartialInfo says that a page is known to be non-exhaustive: data that
// matched the query is missing from it.
//
// It is separate from an error because a page can be useful and incomplete at
// the same time. A client that renders results must not silently drop this;
// the whole point of the block is that the omission is reported rather than
// looking like an empty result.
//
// Reasons is an open set. Treat a value you do not recognise as "omitted for a
// reason this client does not know about", never as no omission.
type SearchPartialInfo struct {
	Reasons                 []string `json:"reasons"`
	BatchFetchErrors        uint64   `json:"batchFetchErrors,omitempty"`
	EventsSkippedTooLarge   uint64   `json:"eventsSkippedTooLarge,omitempty"`
	BatchesAbandonedTooSlow uint64   `json:"batchesAbandonedTooSlow,omitempty"`
}

// SearchPoll is one response from the Search API: the answer to a submission
// or to a page fetch, both of which return the same shape.
type SearchPoll struct {
	QueryID string `json:"queryId"`
	// Completed reports whether the page is finished. False means the server is
	// still producing it and the same request should be repeated, not that the
	// search failed or that there is nothing to return.
	Completed bool `json:"completed"`
	// NextPollInMs is how long the server suggests waiting before asking again
	// for a page that is not yet complete.
	NextPollInMs int64              `json:"nextPollInMs"`
	Results      []SearchResultItem `json:"results"`
	// Error is set when the search itself failed. It is reported in a 200
	// response body, so a caller polling by hand must check it: a page with an
	// error is not a page with no results.
	Error string `json:"error,omitempty"`
	// Partial is set only when the results are known to be non-exhaustive.
	Partial *SearchPartialInfo `json:"partial,omitempty"`
}

// NextToken returns the continuation token for the page after this one, or ""
// when this page ends the search.
//
// The token is carried on a result item rather than on the response, and which
// item holds it is not something a caller should depend on, so read it here.
func (p *SearchPoll) NextToken() string {
	if p == nil {
		return ""
	}
	token := ""
	for _, item := range p.Results {
		if item.NextToken != "" {
			token = item.NextToken
		}
	}
	return token
}

// SearchValidation is the answer to ValidateSearch: whether a query is usable
// and what running it is estimated to cost.
type SearchValidation struct {
	Query     string `json:"query"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	// Error is the validation failure, reported in a 200 response body. A
	// non-empty Error means the query is not usable.
	Error          string      `json:"error,omitempty"`
	Stats          SearchStats `json:"stats"`
	EstimatedPrice Dict        `json:"estimatedPrice,omitempty"`
}

// SearchExecuteOptions tunes how pages are polled and observed. The zero value
// is usable and applies the defaults described on each field.
type SearchExecuteOptions struct {
	// MaxPollAttempts bounds how many times a single page is polled before the
	// call gives up. Defaults to 300. It exists so a stuck page cannot loop
	// forever; a real time budget belongs on the context.
	MaxPollAttempts int
	// PollInterval is the floor under the server's suggested poll delay. The
	// server's suggestion governs whenever it is larger. Defaults to 500ms.
	PollInterval time.Duration
	// OnQueryInitiated, when set, is called by ExecuteSearch with the query id
	// as soon as the submission is accepted and before any page is fetched.
	// It is the hook for persisting the id, which is what makes a search
	// resumable or cancellable after the calling process is gone.
	OnQueryInitiated func(queryID string)
	// OnPageCompleted, when set, is called by ExecuteSearch once per completed
	// page with the 1-based page number and that page's continuation token,
	// empty on the last page. It runs before the page is handed to the
	// handler.
	OnPageCompleted func(pageNumber int, nextToken string)
}

// withDefaults returns a copy with unset polling knobs filled in.
func (o SearchExecuteOptions) withDefaults() SearchExecuteOptions {
	if o.MaxPollAttempts <= 0 {
		o.MaxPollAttempts = defaultSearchMaxPollAttempts
	}
	if o.PollInterval <= 0 {
		o.PollInterval = defaultSearchPollInterval
	}
	return o
}

// SearchPageHandler receives each completed page from ExecuteSearch.
//
// Return (true, nil) to continue to the next page, (false, nil) to stop early
// without an error, or a non-nil error to abort the search with it.
type SearchPageHandler func(page *SearchPoll) (keepGoing bool, err error)

// InitiateSearch submits a search and returns the query id that identifies it.
//
// It does not wait for any results: fetch them with FetchSearchPage, or use
// ExecuteSearch to do both. Mode, stream and pagination are fixed by this call
// for the whole search.
//
// A submission that fails may still have started a search, because the failure
// can happen after the server accepted it. Organization.ListOpenQueries shows
// what an organization actually has open.
//
// Example:
//
//	queryID, err := org.InitiateSearch(SearchRequest{
//	    Query:     "* | NEW_PROCESS | *",
//	    StartTime: start.Unix(),
//	    EndTime:   end.Unix(),
//	})
func (org *Organization) InitiateSearch(req SearchRequest) (string, error) {
	return org.InitiateSearchWithContext(context.Background(), req)
}

// InitiateSearchWithContext is InitiateSearch with a context for cancellation
// and deadlines.
func (org *Organization) InitiateSearchWithContext(ctx context.Context, req SearchRequest) (string, error) {
	root, err := org.getServiceRoot("search")
	if err != nil {
		return "", fmt.Errorf("failed to resolve search service root: %w", err)
	}
	body, err := buildSearchSubmitBody(org.GetOID(), req)
	if err != nil {
		return "", fmt.Errorf("failed to encode search request: %w", err)
	}

	resp := SearchPoll{}
	restReq := makeDefaultRequest(&resp).
		withURLRoot(root).
		withRawBody(body, "application/json").
		withTimeout(searchRequestTimeout)
	if err := org.client.reliableRequest(ctx, http.MethodPost, "/v1/search", restReq); err != nil {
		return "", fmt.Errorf("failed to initiate search: %w", err)
	}
	// The failure is reported in the body of a 200, so it has to be checked
	// here or it reads as a search with no results.
	if resp.Error != "" {
		return "", fmt.Errorf("failed to initiate search: %s", resp.Error)
	}
	if resp.QueryID == "" {
		return "", fmt.Errorf("search submission returned no query id")
	}
	return resp.QueryID, nil
}

// PollSearch asks once for a page of a search and returns whatever the server
// says, complete or not.
//
// Pass an empty token for the first page, and a token from a previous page's
// NextToken for any page after it. This is the raw primitive: a page that is
// still being produced comes back with Completed false, and it is the caller's
// job to wait and ask again. FetchSearchPage does that.
func (org *Organization) PollSearch(queryID, token string) (*SearchPoll, error) {
	return org.PollSearchWithContext(context.Background(), queryID, token)
}

// PollSearchWithContext is PollSearch with a context for cancellation and
// deadlines.
func (org *Organization) PollSearchWithContext(ctx context.Context, queryID, token string) (*SearchPoll, error) {
	if queryID == "" {
		return nil, fmt.Errorf("a query id is required to poll a search")
	}
	root, err := org.getServiceRoot("search")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve search service root: %w", err)
	}

	resp := SearchPoll{}
	restReq := makeDefaultRequest(&resp).withURLRoot(root).withTimeout(searchRequestTimeout)
	// A continuation carries the token and nothing else. The criteria are the
	// ones submitted with the search and are never restated.
	if token != "" {
		restReq = restReq.withQueryData(Dict{"token": token})
	}
	if err := org.client.reliableRequest(ctx, http.MethodGet, "/v1/search/"+queryID, restReq); err != nil {
		return nil, fmt.Errorf("failed to poll search %s: %w", queryID, err)
	}
	return &resp, nil
}

// FetchSearchPage fetches one page of a search and returns it once the server
// reports it complete, polling at the cadence the server advises.
//
// Pass an empty token for the first page. The returned page's NextToken is
// what fetches the one after it; an empty NextToken means the search is over.
//
// Only the polling fields of opts are used. A search error reported in the
// page body is returned as an error, so a non-nil page is always a page that
// ran.
func (org *Organization) FetchSearchPage(ctx context.Context, queryID, token string, opts SearchExecuteOptions) (*SearchPoll, error) {
	opts = opts.withDefaults()

	for attempt := 0; attempt < opts.MaxPollAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := org.PollSearchWithContext(ctx, queryID, token)
		if err != nil {
			return nil, err
		}
		if page.Error != "" {
			return nil, fmt.Errorf("search %s failed: %s", queryID, page.Error)
		}
		if page.Completed {
			return page, nil
		}

		wait := time.Duration(page.NextPollInMs) * time.Millisecond
		if wait <= 0 {
			wait = fallbackSearchPollInterval
		}
		if wait < opts.PollInterval {
			wait = opts.PollInterval
		}
		if err := waitForRetry(ctx, wait); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("search %s did not produce a page within %d polls", queryID, opts.MaxPollAttempts)
}

// ExecuteSearch runs a whole search: it submits the criteria, fetches every
// page in turn, and hands each completed page to handler.
//
// It returns nil once a page comes back with no continuation token, or once
// handler asks to stop. A handler that returns an error aborts the search with
// that error. If ctx is cancelled the search is cancelled server-side on a
// best-effort basis so the work stops rather than running on unread, and
// ctx.Err() is returned.
//
// Mode travels with the submission only. Continuation requests carry no body,
// so every page of one ExecuteSearch call runs under the mode submitted here.
// An unset Mode submits SearchModeBatch, which is what reading every page in a
// loop wants; WithoutMode or WithMode changes that.
//
// Example:
//
//	err := org.ExecuteSearch(ctx, SearchRequest{
//	    Query:     "* | NEW_PROCESS | *",
//	    StartTime: start.Unix(),
//	    EndTime:   end.Unix(),
//	}, SearchExecuteOptions{}, func(page *SearchPoll) (bool, error) {
//	    for _, item := range page.Results {
//	        if item.Type == "events" {
//	            consume(item.Rows)
//	        }
//	    }
//	    return true, nil
//	})
func (org *Organization) ExecuteSearch(ctx context.Context, req SearchRequest, opts SearchExecuteOptions, handler SearchPageHandler) error {
	if handler == nil {
		return fmt.Errorf("a page handler is required to execute a search")
	}
	opts = opts.withDefaults()

	queryID, err := org.InitiateSearchWithContext(ctx, req)
	if err != nil {
		return err
	}
	if opts.OnQueryInitiated != nil {
		opts.OnQueryInitiated(queryID)
	}

	token := ""
	for pageNumber := 1; ; pageNumber++ {
		page, err := org.FetchSearchPage(ctx, queryID, token, opts)
		if err != nil {
			if ctx.Err() != nil {
				org.cancelSearchBestEffort(queryID)
			}
			return err
		}

		nextToken := page.NextToken()
		if opts.OnPageCompleted != nil {
			opts.OnPageCompleted(pageNumber, nextToken)
		}

		keepGoing, err := handler(page)
		if err != nil {
			return err
		}
		if !keepGoing || nextToken == "" {
			return nil
		}
		token = nextToken
	}
}

// CancelSearch asks the server to stop a search and release the concurrency
// slot it holds.
//
// Cancelling a search that has already finished or expired is not an error
// worth acting on: the outcome the caller wanted is the outcome either way.
func (org *Organization) CancelSearch(queryID string) error {
	return org.CancelSearchWithContext(context.Background(), queryID)
}

// CancelSearchWithContext is CancelSearch with a context for cancellation and
// deadlines.
func (org *Organization) CancelSearchWithContext(ctx context.Context, queryID string) error {
	if queryID == "" {
		return fmt.Errorf("a query id is required to cancel a search")
	}
	root, err := org.getServiceRoot("search")
	if err != nil {
		return fmt.Errorf("failed to resolve search service root: %w", err)
	}
	restReq := makeDefaultRequest(nil).withURLRoot(root).withTimeout(searchRequestTimeout)
	if err := org.client.reliableRequest(ctx, http.MethodDelete, "/v1/search/"+queryID, restReq); err != nil {
		return fmt.Errorf("failed to cancel search %s: %w", queryID, err)
	}
	return nil
}

// cancelSearchBestEffort tells the server to stop a search on a context of its
// own, because the caller's is already cancelled and would refuse the call.
// The outcome is deliberately discarded: the decision to stop has been made,
// and failing to deliver it changes nothing the caller can act on.
func (org *Organization) cancelSearchBestEffort(queryID string) {
	ctx, cancel := context.WithTimeout(context.Background(), cancelSearchTimeout)
	defer cancel()
	_ = org.CancelSearchWithContext(ctx, queryID)
}

// ValidateSearch checks that a query is usable and estimates what running it
// would cost, without running it.
//
// Only the criteria are validated. Pagination and mode describe how results
// are delivered and are not part of a validation, so they are not sent.
//
// A query the server rejects comes back as a SearchValidation with Error set,
// not as a call failure: the returned error is for a validation that could not
// be performed at all.
func (org *Organization) ValidateSearch(req SearchRequest) (*SearchValidation, error) {
	return org.ValidateSearchWithContext(context.Background(), req)
}

// ValidateSearchWithContext is ValidateSearch with a context for cancellation
// and deadlines.
func (org *Organization) ValidateSearchWithContext(ctx context.Context, req SearchRequest) (*SearchValidation, error) {
	root, err := org.getServiceRoot("search")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve search service root: %w", err)
	}
	body, err := buildSearchValidateBody(org.GetOID(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode search validation request: %w", err)
	}

	resp := SearchValidation{}
	restReq := makeDefaultRequest(&resp).
		withURLRoot(root).
		withRawBody(body, "application/json").
		withTimeout(searchRequestTimeout)
	if err := org.client.reliableRequest(ctx, http.MethodPost, "/v1/search/validate", restReq); err != nil {
		return nil, fmt.Errorf("failed to validate search: %w", err)
	}
	return &resp, nil
}
