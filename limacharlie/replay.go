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

// ReplayDRRuleRequest contains the parameters for replaying a D&R rule.
type ReplayDRRuleRequest struct {
	// RuleName is the name of an existing rule to replay (optional if Rule is provided)
	RuleName string `json:"rule_name,omitempty"`
	// Namespace is the rule namespace (general, managed, service). Default: general
	Namespace string `json:"namespace,omitempty"`
	// Rule is an inline D&R rule to replay (optional if RuleName is provided)
	// Should contain "detect" and optionally "respond" keys
	Rule Dict `json:"rule,omitempty"`

	// Events is a list of inline events to test against (mutually exclusive with SensorEvents)
	Events []Dict `json:"events,omitempty"`

	// SID is a specific sensor ID to replay events from
	SID string `json:"sid,omitempty"`
	// SIDs is a list of sensor IDs to replay events from
	SIDs []string `json:"sids,omitempty"`
	// Selector is a sensor selector expression (bexpr syntax)
	Selector string `json:"selector,omitempty"`
	// StartTime is the start timestamp in epoch seconds
	StartTime int64 `json:"start_time,omitempty"`
	// EndTime is the end timestamp in epoch seconds
	EndTime int64 `json:"end_time,omitempty"`

	// Stream is the data stream to replay from: "event", "audit", or "detect". Default: "event"
	Stream string `json:"stream,omitempty"`

	// LimitEvent is the maximum number of events to process (0 = no limit)
	LimitEvent uint64 `json:"limit_event,omitempty"`
	// LimitEval is the maximum number of evaluations to perform (0 = no limit)
	LimitEval uint64 `json:"limit_eval,omitempty"`

	// Trace enables detailed trace output for debugging rule evaluation
	Trace bool `json:"trace,omitempty"`
	// DryRun returns cost estimates without actually processing events
	DryRun bool `json:"dry_run,omitempty"`
}

// ReplayDRRuleResponse contains the result of a D&R rule replay.
type ReplayDRRuleResponse struct {
	// Error contains any error message from the replay
	Error string `json:"error,omitempty"`
	// Stats contains statistics about the replay execution
	Stats ReplayStats `json:"stats"`
	// Results contains the actions that would have been taken by the rule
	Results []ReplayResult `json:"results"`
	// DidMatch indicates if the rule matched any events
	DidMatch bool `json:"did_match"`
	// Traces contains detailed evaluation traces (only if Trace was enabled)
	Traces [][]string `json:"traces,omitempty"`
	// IsDryRun indicates if this was a dry run
	IsDryRun bool `json:"is_dry_run"`
}

// ReplayStats contains statistics about a replay execution.
type ReplayStats struct {
	// NumScanned is the number of events scanned
	NumScanned uint64 `json:"n_scan"`
	// NumBytesScanned is the number of bytes scanned
	NumBytesScanned uint64 `json:"n_bytes_scan"`
	// NumEventsProcessed is the number of events processed
	NumEventsProcessed uint64 `json:"n_proc"`
	// NumEventsMatched is the number of events that matched the rule
	NumEventsMatched uint64 `json:"n_matched"`
	// NumShards is the number of shards the replay was split into
	NumShards uint64 `json:"n_shard"`
	// NumEvals is the number of rule evaluations performed
	NumEvals uint64 `json:"n_eval"`
	// WallTime is the wall clock time in seconds
	WallTime float64 `json:"wall_time,omitempty"`
	// CumulativeTime is the cumulative processing time in seconds
	CumulativeTime float64 `json:"cummulative_time,omitempty"`
	// NumBatches is the number of batch accesses
	NumBatches uint64 `json:"n_batch_access"`
	// BilledFor is the number of events billed
	BilledFor uint64 `json:"n_billed"`
	// NotBilledFor is the number of events not billed (free tier)
	NotBilledFor uint64 `json:"n_free"`
	// EstimatedPrice contains the calculated billing estimate based on BilledFor events
	EstimatedPrice EstimatedPrice `json:"estimated_price,omitempty"`
}

// EstimatedPrice represents the calculated billing estimate for a query.
type EstimatedPrice struct {
	// Price is the estimated cost in the specified currency
	Price float64 `json:"value"`
	// Currency is the unit for the price (e.g., "USD cents")
	Currency string `json:"currency"`
}

// ReplayResult contains a single action result from a replay.
type ReplayResult struct {
	// Action is the action type (e.g., "report", "task", "tag")
	Action string `json:"action"`
	// Data contains the action data
	Data Dict `json:"data"`
}

// ReplayDRRule replays a D&R rule against events.
// This can test rules against inline events or historical sensor data.
//
// For inline event testing (unit testing style):
//
//	req := lc.ReplayDRRuleRequest{
//	    Rule: lc.Dict{
//	        "detect": lc.Dict{
//	            "event": "NEW_PROCESS",
//	            "op": "contains",
//	            "path": "event/FILE_PATH",
//	            "value": "powershell",
//	        },
//	        "respond": lc.List{
//	            lc.Dict{"action": "report", "name": "suspicious-process"},
//	        },
//	    },
//	    Events: []lc.Dict{
//	        {
//	            "routing": lc.Dict{"event_type": "NEW_PROCESS"},
//	            "event": lc.Dict{"FILE_PATH": "C:\\Windows\\powershell.exe"},
//	        },
//	    },
//	    Trace: true,
//	}
//	result, err := org.ReplayDRRule(req)
//
// For historical replay:
//
//	req := lc.ReplayDRRuleRequest{
//	    RuleName: "my-detection-rule",
//	    SID: "9cbed57a-6d6a-4af0-b881-803a99b177d9",
//	    StartTime: time.Now().Add(-1 * time.Hour).Unix(),
//	    EndTime: time.Now().Unix(),
//	    LimitEvent: 10000,
//	}
//	result, err := org.ReplayDRRule(req)
func (org *Organization) ReplayDRRule(req ReplayDRRuleRequest) (*ReplayDRRuleResponse, error) {
	return org.ReplayDRRuleWithContext(context.Background(), req)
}

// ReplayDRRuleWithContext replays a D&R rule with a context for cancellation.
func (org *Organization) ReplayDRRuleWithContext(ctx context.Context, req ReplayDRRuleRequest) (*ReplayDRRuleResponse, error) {
	// Get replay URL from organization
	urls, err := org.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization URLs: %v", err)
	}

	replayURL, ok := urls["replay"]
	if !ok {
		return nil, fmt.Errorf("replay URL not found in organization URLs")
	}

	// Set default stream
	stream := req.Stream
	if stream == "" {
		stream = "event"
	}

	// Build the rule source
	ruleSource := map[string]interface{}{}
	if req.RuleName != "" {
		ruleSource["rule_name"] = req.RuleName
	}
	if req.Namespace != "" {
		ruleSource["namespace"] = req.Namespace
	}
	if req.Rule != nil {
		ruleSource["rule"] = req.Rule
	}

	// Build the event source
	eventSource := map[string]interface{}{
		"stream": stream,
	}

	// If inline events are provided, use them
	if len(req.Events) > 0 {
		eventSource["events"] = req.Events
	} else {
		// Use sensor events (historical replay)
		sensorEvents := map[string]interface{}{}
		if req.SID != "" {
			sensorEvents["sid"] = req.SID
		}
		if len(req.SIDs) > 0 {
			sensorEvents["sids"] = req.SIDs
		}
		if req.Selector != "" {
			sensorEvents["selector"] = req.Selector
		}
		if req.StartTime > 0 {
			sensorEvents["start_time"] = req.StartTime
		}
		if req.EndTime > 0 {
			sensorEvents["end_time"] = req.EndTime
		}
		eventSource["sensor_events"] = sensorEvents
	}

	// Build the request body
	requestBody := map[string]interface{}{
		"oid":          org.GetOID(),
		"rule_source":  ruleSource,
		"event_source": eventSource,
		"trace":        req.Trace,
		"is_dry_run":   req.DryRun,
	}

	if req.LimitEvent > 0 {
		requestBody["limit_event"] = req.LimitEvent
	}
	if req.LimitEval > 0 {
		requestBody["limit_eval"] = req.LimitEval
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

	// Execute the request with a longer timeout for replay operations
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute replay request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response
	var response ReplayDRRuleResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		// If we can't parse the response, check if it's an HTTP error
		if httpResp.StatusCode != http.StatusOK {
			return &ReplayDRRuleResponse{
				Error: fmt.Sprintf("replay request failed with status %d: %s", httpResp.StatusCode, string(respBody)),
			}, nil
		}
		return nil, fmt.Errorf("failed to parse replay response: %v", err)
	}

	return &response, nil
}
