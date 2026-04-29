package limacharlie

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The Search API drives the "replay-search" flow: POST /v1/search to
// initiate a query against the event store, then poll GET /v1/search/{queryId}
// until completed=true, optionally following a nextToken across pages. This
// mirrors the Python SDK's Manager.executeSearch family of methods.
//
// The older Organization.Query() in query.go targets urls["replay"], which
// is a different, legacy endpoint. Prefer the Search API for new code.

const (
	// searchHTTPTimeout bounds each individual HTTP call in the Search API.
	// Matches the 120s cap used by Organization.QueryWithContext.
	searchHTTPTimeout = 120 * time.Second

	// defaultSearchPollInterval is the floor applied to server-specified
	// poll delays when the caller doesn't override it. Matches the Python
	// SDK default (Manager.py:1424).
	defaultSearchPollInterval = 2 * time.Second

	// defaultSearchMaxPollAttempts bounds how many polls ExecuteSearch will
	// make per page before giving up. Matches Python (Manager.py:1429).
	defaultSearchMaxPollAttempts = 300

	// cancelSearchTimeout is the short budget used for the best-effort
	// DELETE issued when the caller's context is cancelled mid-search.
	cancelSearchTimeout = 5 * time.Second
)

// SearchRequest is the input to InitiateSearch / ValidateSearch /
// ExecuteSearch. Query/StartTime/EndTime are required. Stream defaults to
// "event" and Paginated defaults to true — these are the values the Python
// SDK uses when the caller omits them. To override either, set them
// explicitly.
type SearchRequest struct {
	// Query is the LCQL string, e.g. "* | VARIST_SCAN_FILE_REP | *".
	Query string
	// StartTime and EndTime are unix seconds. Both required; the server
	// rejects zero bounds.
	StartTime int64
	EndTime   int64
	// Stream is "event", "detect", or "audit". Defaults to "event".
	Stream string
	// Paginated asks the server to break results into pages connected by
	// nextToken. Defaults to true.
	Paginated bool
	// paginatedSet lets us distinguish "caller left it false (so we should
	// default to true)" from "caller set it false on purpose". Internal.
	paginatedSet bool
}

// WithPaginated returns a copy of the request with Paginated set to the
// given value. Use this when you need Paginated=false; passing the field
// directly is ambiguous with the zero value and we default to true.
func (r SearchRequest) WithPaginated(p bool) SearchRequest {
	r.Paginated = p
	r.paginatedSet = true
	return r
}

// SearchResultItem is one entry in a poll response's Results array. The
// server emits separate items for "events", "facets", and "timeline"
// result types; inspect Type before reading the typed fields.
type SearchResultItem struct {
	Type       string `json:"type"`
	Rows       []Dict `json:"rows,omitempty"`
	Facets     []Dict `json:"facets,omitempty"`
	Timeseries []Dict `json:"timeseries,omitempty"`
	// NextToken, when non-empty, points at the next page of results. Per
	// the server contract it appears on the last item of a page only, so
	// callers should scan the full Results slice for any non-empty value.
	NextToken string `json:"nextToken,omitempty"`
}

// SearchPoll models one poll response body from GET /v1/search/{queryId}.
type SearchPoll struct {
	Completed    bool               `json:"completed"`
	NextPollInMs int                `json:"nextPollInMs"`
	Results      []SearchResultItem `json:"results"`
	Stats        Dict               `json:"stats,omitempty"`
	Error        string             `json:"error,omitempty"`
}

// SearchValidation is the response from POST /v1/search/validate. Fields
// beyond what the server documents are tolerated as extras.
type SearchValidation struct {
	Error          string `json:"error,omitempty"`
	EstimatedPrice Dict   `json:"estimatedPrice,omitempty"`
	Extras         Dict   `json:"-"`
}

// SearchExecuteOptions tunes ExecuteSearch's polling behaviour.
type SearchExecuteOptions struct {
	// MaxPollAttempts bounds the number of polls issued per page before
	// giving up with an error. Defaults to 300.
	MaxPollAttempts int
	// PollInterval is the minimum delay between polls. The server's
	// nextPollInMs hint is honored but clamped up to this floor.
	// Defaults to 2 seconds.
	PollInterval time.Duration
	// OnQueryInitiated, if set, is called once with the queryId returned
	// from the initial POST. Useful for logging or state persistence.
	OnQueryInitiated func(queryID string)
	// OnPageCompleted, if set, is called once per completed page with
	// the 1-based page index and the nextToken (empty when no more pages).
	OnPageCompleted func(pageNum int, nextToken string)
}

// SearchPageHandler receives each completed poll page in ExecuteSearch.
// Return (false, nil) to stop fetching further pages; return a non-nil
// error to abort the whole search with that error. Returning (true, nil)
// continues to the next page (or terminates if the current page has no
// nextToken).
type SearchPageHandler func(page *SearchPoll) (keepGoing bool, err error)

// searchBaseURL returns the URL prefix for /v1/search calls, honoring an
// already-qualified override (used by unit tests to point at an httptest
// server) or wrapping a bare hostname with https:// for production.
func (o *Organization) searchBaseURL() (string, error) {
	urls, err := o.GetURLs()
	if err != nil {
		return "", fmt.Errorf("failed to get organization URLs: %v", err)
	}
	host, ok := urls["search"]
	if !ok || host == "" {
		return "", fmt.Errorf("search URL not found in organization URLs")
	}
	// Tests preload urls["search"] with a full httptest URL (http://...);
	// production values are hostname-only and always use https.
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/") + "/v1/search", nil
	}
	return "https://" + host + "/v1/search", nil
}

// doSearchRequest is the single entry point that every Search HTTP call
// goes through. It attaches the current JWT, executes the request, and
// transparently refreshes+retries once on a 401. A second 401 (or a
// refresh failure) is surfaced to the caller.
//
// Mirrors the Python SDK's per-request 401 handling (Manager.py:490).
// It is deliberately narrower than the older reliableRequest — no 5xx /
// 429 retries — to match the synchronous call shape of query.go.
func (o *Organization) doSearchRequest(ctx context.Context, method, u string, body []byte) (int, []byte, error) {
	attempt := func(jwt string) (int, []byte, error) {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to create search request: %v", err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", "limacharlie-sdk")
		if jwt != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
		}
		client := &http.Client{Timeout: searchHTTPTimeout}
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to execute search request: %v", err)
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp.StatusCode, nil, fmt.Errorf("failed to read search response: %v", err)
		}
		return resp.StatusCode, respBody, nil
	}

	status, respBody, err := attempt(o.GetCurrentJWT())
	if err != nil || status != http.StatusUnauthorized {
		return status, respBody, err
	}

	// 401: refresh the JWT once and retry. Uses the client's configured
	// JWTExpiryTime so a call-site refresh matches what reliableRequest
	// does for /v1 endpoints (see client.go:265).
	fresh := o.RefreshJWT(o.client.options.JWTExpiryTime)
	if fresh == "" {
		return status, respBody, fmt.Errorf("search request returned 401 and JWT refresh failed")
	}
	return attempt(fresh)
}

// applyDefaults normalizes a SearchRequest for dispatch: default Stream
// to "event" and Paginated to true unless the caller set them explicitly.
func (r SearchRequest) applyDefaults() SearchRequest {
	if r.Stream == "" {
		r.Stream = "event"
	}
	if !r.paginatedSet {
		r.Paginated = true
	}
	return r
}

// buildSearchBody encodes the POST body shared by InitiateSearch and
// ValidateSearch. Times go out as strings to match the Python SDK; the
// server rejects numeric types on this endpoint.
func buildSearchBody(oid string, r SearchRequest) ([]byte, error) {
	r = r.applyDefaults()
	body := map[string]interface{}{
		"oid":       oid,
		"query":     r.Query,
		"startTime": strconv.FormatInt(r.StartTime, 10),
		"endTime":   strconv.FormatInt(r.EndTime, 10),
		"paginated": r.Paginated,
		"stream":    r.Stream,
	}
	return json.Marshal(body)
}

// InitiateSearch starts a replay search and returns the server-assigned
// queryId. The caller is then responsible for polling (see PollSearch) or
// cancelling (see CancelSearch). For an all-in-one flow use ExecuteSearch.
func (org *Organization) InitiateSearch(req SearchRequest) (string, error) {
	return org.InitiateSearchWithContext(context.Background(), req)
}

// InitiateSearchWithContext is InitiateSearch with a cancellable context.
func (org *Organization) InitiateSearchWithContext(ctx context.Context, req SearchRequest) (string, error) {
	baseURL, err := org.searchBaseURL()
	if err != nil {
		return "", err
	}
	bodyBytes, err := buildSearchBody(org.GetOID(), req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal search request body: %v", err)
	}

	status, respBody, err := org.doSearchRequest(ctx, http.MethodPost, baseURL, bodyBytes)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("search initiate failed with status %d: %s", status, string(respBody))
	}

	var parsed struct {
		QueryID string `json:"queryId"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse search initiate response: %v", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("search initiate error: %s", parsed.Error)
	}
	if parsed.QueryID == "" {
		return "", fmt.Errorf("search initiate returned no queryId: %s", string(respBody))
	}
	return parsed.QueryID, nil
}

// PollSearch retrieves the current state of a search identified by
// queryID. Pass an empty token on the first call; thereafter pass the
// nextToken observed in a prior poll's last Results item to page forward.
func (org *Organization) PollSearch(queryID, token string) (*SearchPoll, error) {
	return org.PollSearchWithContext(context.Background(), queryID, token)
}

// PollSearchWithContext is PollSearch with a cancellable context.
func (org *Organization) PollSearchWithContext(ctx context.Context, queryID, token string) (*SearchPoll, error) {
	if queryID == "" {
		return nil, fmt.Errorf("queryID is required")
	}
	baseURL, err := org.searchBaseURL()
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/%s", baseURL, queryID)
	if token != "" {
		u = fmt.Sprintf("%s?token=%s", u, url.QueryEscape(token))
	}

	status, respBody, err := org.doSearchRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("search poll failed with status %d: %s", status, string(respBody))
	}

	var parsed SearchPoll
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse search poll response: %v", err)
	}
	return &parsed, nil
}

// CancelSearch asks the server to abort a running search. Returns nil on
// success; errors surfaced include network failures and non-200 responses.
func (org *Organization) CancelSearch(queryID string) error {
	return org.CancelSearchWithContext(context.Background(), queryID)
}

// CancelSearchWithContext is CancelSearch with a cancellable context.
func (org *Organization) CancelSearchWithContext(ctx context.Context, queryID string) error {
	if queryID == "" {
		return fmt.Errorf("queryID is required")
	}
	baseURL, err := org.searchBaseURL()
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/%s", baseURL, queryID)
	status, respBody, err := org.doSearchRequest(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("search cancel failed with status %d: %s", status, string(respBody))
	}
	return nil
}

// ValidateSearch checks a SearchRequest for syntactic validity and, when
// the server supports it, returns a pricing estimate. No search is run.
func (org *Organization) ValidateSearch(req SearchRequest) (*SearchValidation, error) {
	return org.ValidateSearchWithContext(context.Background(), req)
}

// ValidateSearchWithContext is ValidateSearch with a cancellable context.
func (org *Organization) ValidateSearchWithContext(ctx context.Context, req SearchRequest) (*SearchValidation, error) {
	baseURL, err := org.searchBaseURL()
	if err != nil {
		return nil, err
	}
	bodyBytes, err := buildSearchBody(org.GetOID(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search validate body: %v", err)
	}

	status, respBody, err := org.doSearchRequest(ctx, http.MethodPost, baseURL+"/validate", bodyBytes)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("search validate failed with status %d: %s", status, string(respBody))
	}

	var parsed SearchValidation
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse search validate response: %v", err)
	}
	return &parsed, nil
}

// ExecuteSearch drives the full replay-search flow: initiate, poll until
// each page completes, invoke handler with the page, and follow any
// nextToken to the next page. Returns nil when a page reports
// completed=true with no more pages, or when handler returns (false, nil).
//
// Cancellation: if ctx is cancelled mid-search, ExecuteSearch issues a
// best-effort DELETE /v1/search/{queryId} on a fresh 5s context so the
// server can stop work immediately. The cancel error is swallowed; the
// return value is always the original ctx.Err().
func (org *Organization) ExecuteSearch(ctx context.Context, req SearchRequest, opts SearchExecuteOptions, handler SearchPageHandler) error {
	return org.ExecuteSearchWithContext(ctx, req, opts, handler)
}

// ExecuteSearchWithContext is the implementation of ExecuteSearch; the
// two forms are equivalent but we expose both for symmetry with the rest
// of the SDK.
func (org *Organization) ExecuteSearchWithContext(ctx context.Context, req SearchRequest, opts SearchExecuteOptions, handler SearchPageHandler) error {
	if handler == nil {
		return fmt.Errorf("handler is required")
	}
	if opts.MaxPollAttempts <= 0 {
		opts.MaxPollAttempts = defaultSearchMaxPollAttempts
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultSearchPollInterval
	}

	queryID, err := org.InitiateSearchWithContext(ctx, req)
	if err != nil {
		return err
	}
	if opts.OnQueryInitiated != nil {
		opts.OnQueryInitiated(queryID)
	}

	token := ""
	pageNum := 0

	for {
		// Poll this page until it reports Completed=true or we hit
		// MaxPollAttempts. Between polls, honor NextPollInMs but clamp
		// up to opts.PollInterval so a low hint can't flood the server.
		var page *SearchPoll
		for attempt := 0; attempt < opts.MaxPollAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				org.bestEffortCancelSearch(queryID)
				return err
			}
			page, err = org.PollSearchWithContext(ctx, queryID, token)
			if err != nil {
				if ctx.Err() != nil {
					org.bestEffortCancelSearch(queryID)
				}
				return err
			}
			if page.Error != "" {
				return fmt.Errorf("search error: %s", page.Error)
			}
			if page.Completed {
				break
			}

			wait := time.Duration(page.NextPollInMs) * time.Millisecond
			if wait < opts.PollInterval {
				wait = opts.PollInterval
			}
			select {
			case <-ctx.Done():
				org.bestEffortCancelSearch(queryID)
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		if page == nil || !page.Completed {
			return fmt.Errorf("search did not complete within %d poll attempts", opts.MaxPollAttempts)
		}

		pageNum++

		// nextToken lives on the last non-empty Results entry per the
		// server contract; scan the slice to be robust to ordering.
		nextToken := ""
		for _, r := range page.Results {
			if r.NextToken != "" {
				nextToken = r.NextToken
			}
		}

		if opts.OnPageCompleted != nil {
			opts.OnPageCompleted(pageNum, nextToken)
		}

		keepGoing, hErr := handler(page)
		if hErr != nil {
			return hErr
		}
		if !keepGoing || nextToken == "" {
			return nil
		}
		token = nextToken
	}
}

// bestEffortCancelSearch issues a DELETE on a fresh context so a caller's
// cancelled ctx doesn't prevent the server from being told. Errors are
// swallowed — we've already decided to stop.
func (org *Organization) bestEffortCancelSearch(queryID string) {
	ctx, cancel := context.WithTimeout(context.Background(), cancelSearchTimeout)
	defer cancel()
	_ = org.CancelSearchWithContext(ctx, queryID)
}
