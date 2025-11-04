package limacharlie

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// QueryRequest contains parameters for an LCQL query.
type QueryRequest struct {
	// Query is the LCQL query string (e.g., "timeframe | sensors | events | query_logic")
	Query string
	// Stream specifies which stream to query: "event", "detect", or "audit"
	Stream string
	// LimitEvent approximately limits the number of events processed
	LimitEvent int
	// LimitEval approximately limits the number of rule evaluations
	LimitEval int
	// Cursor is used for pagination ("-" for first page, empty to fetch all in one request)
	Cursor string
}

// QueryResponse contains the results of an LCQL query.
type QueryResponse struct {
	// Results contains the query results
	Results []Dict `json:"results"`
	// Cursor is the pagination cursor for the next page (empty if no more results)
	Cursor string `json:"cursor"`
	// Stats contains query statistics
	Stats Dict `json:"stats,omitempty"`
}

// QueryIterator provides an iterator interface for paginated query results.
type QueryIterator struct {
	org       *Organization
	request   QueryRequest
	replayURL string
	hasMore   bool
	ctx       context.Context
}

// Query executes an LCQL query and returns the first page of results.
// For cursor-based pagination, use QueryAll() to get an iterator.
//
// Example:
//
//	resp, err := org.Query(QueryRequest{
//	    Query: "-1h | * | * | event.FILE_PATH ends with '.exe'",
//	    Stream: "event",
//	    LimitEvent: 1000,
//	})
func (org *Organization) Query(req QueryRequest) (*QueryResponse, error) {
	return org.QueryWithContext(context.Background(), req)
}

// QueryWithContext executes an LCQL query with a context for cancellation.
func (org *Organization) QueryWithContext(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	// Get replay URL from organization
	urls, err := org.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization URLs: %v", err)
	}

	replayURL, ok := urls["replay"]
	if !ok {
		return nil, fmt.Errorf("replay URL not found in organization URLs")
	}

	// Set default stream if not specified
	if req.Stream == "" {
		req.Stream = "event"
	}

	// Set cursor to empty string if not specified (non-paginated)
	// Use "-" to start cursor-based pagination
	cursor := req.Cursor
	if cursor == "" {
		cursor = ""
	}

	// Build the request body
	requestBody := map[string]interface{}{
		"oid":         org.GetOID(),
		"query":       req.Query,
		"limit_event": req.LimitEvent,
		"limit_eval":  req.LimitEval,
		"event_source": map[string]interface{}{
			"stream": req.Stream,
			"sensor_events": map[string]interface{}{
				"cursor": cursor,
			},
		},
	}

	// Marshal the request body
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Build the URL
	url := fmt.Sprintf("https://%s/", replayURL)

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "limacharlie-sdk")

	// Add authentication
	jwt := org.GetCurrentJWT()
	if jwt != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
	}

	// Execute the request
	client := &http.Client{
		Timeout: 120 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query request failed with status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse the response
	var response QueryResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse query response: %v", err)
	}

	return &response, nil
}

// QueryAll returns an iterator for paginated query results.
// The iterator will automatically fetch subsequent pages as you call Next().
//
// Example:
//
//	iter, err := org.QueryAll(QueryRequest{
//	    Query: "-1h | * | * | event.FILE_PATH ends with '.exe'",
//	    Stream: "event",
//	    Cursor: "-", // Start pagination
//	})
//	if err != nil {
//	    return err
//	}
//
//	for iter.HasMore() {
//	    resp, err := iter.Next()
//	    if err != nil {
//	        return err
//	    }
//	    // Process resp.Results...
//	}
func (org *Organization) QueryAll(req QueryRequest) (*QueryIterator, error) {
	return org.QueryAllWithContext(context.Background(), req)
}

// QueryAllWithContext returns an iterator with context for cancellation.
func (org *Organization) QueryAllWithContext(ctx context.Context, req QueryRequest) (*QueryIterator, error) {
	// Get replay URL from organization
	urls, err := org.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization URLs: %v", err)
	}

	replayURL, ok := urls["replay"]
	if !ok {
		return nil, fmt.Errorf("replay URL not found in organization URLs")
	}

	// Set default stream if not specified
	if req.Stream == "" {
		req.Stream = "event"
	}

	// Set cursor to "-" to start pagination if not already set
	if req.Cursor == "" {
		req.Cursor = "-"
	}

	return &QueryIterator{
		org:       org,
		request:   req,
		replayURL: replayURL,
		hasMore:   true,
		ctx:       ctx,
	}, nil
}

// Next fetches the next page of query results.
// Returns nil when there are no more results.
func (qi *QueryIterator) Next() (*QueryResponse, error) {
	if !qi.hasMore {
		return nil, nil
	}

	// Build the request body
	requestBody := map[string]interface{}{
		"oid":         qi.org.GetOID(),
		"query":       qi.request.Query,
		"limit_event": qi.request.LimitEvent,
		"limit_eval":  qi.request.LimitEval,
		"event_source": map[string]interface{}{
			"stream": qi.request.Stream,
			"sensor_events": map[string]interface{}{
				"cursor": qi.request.Cursor,
			},
		},
	}

	// Marshal the request body
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Build the URL
	url := fmt.Sprintf("https://%s/", qi.replayURL)

	// Create the HTTP request
	httpReq, err := http.NewRequestWithContext(qi.ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "limacharlie-sdk")

	// Add authentication
	jwt := qi.org.GetCurrentJWT()
	if jwt != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
	}

	// Execute the request
	client := &http.Client{
		Timeout: 120 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check for HTTP errors
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query request failed with status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse the response
	var response QueryResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse query response: %v", err)
	}

	// Update cursor and hasMore flag
	if response.Cursor == "" {
		qi.hasMore = false
	} else {
		qi.request.Cursor = response.Cursor
	}

	return &response, nil
}

// HasMore returns true if there are more pages of results to fetch.
func (qi *QueryIterator) HasMore() bool {
	return qi.hasMore
}
