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

// ValidationResponse contains the result of a validation request.
type ValidationResponse struct {
	// Error contains the validation error message if validation failed
	Error string `json:"error,omitempty"`
	// Success indicates if the validation succeeded
	Success bool `json:"success,omitempty"`
	// NumEvals contains the number of evaluation operations (for D&R validation)
	NumEvals int `json:"num_evals,omitempty"`
	// NumEvents contains the number of events evaluated (for D&R validation)
	NumEvents int `json:"num_events,omitempty"`
	// EvalTime contains the evaluation time in seconds (for D&R validation)
	EvalTime float64 `json:"eval_time,omitempty"`
}

// BillingEstimate contains estimated billing information for a query.
type BillingEstimate struct {
	// BilledEvents is the estimated number of events that would be billed
	BilledEvents uint64 `json:"billed_events"`
	// FreeEvents is the estimated number of events that would be free (not billed)
	FreeEvents uint64 `json:"free_events"`
}

// lcqlValidationRawResponse is the raw response structure from the replay endpoint.
// Used internally to parse the billing stats from LCQL validation responses.
// The Stats field uses the same ReplayStats structure as the replay endpoint.
type lcqlValidationRawResponse struct {
	// Error contains any error message from validation
	Error string `json:"error,omitempty"`
	// Stats contains statistics from the replay service, including billing information.
	// Uses the same ReplayStats structure as ReplayDRRuleResponse.
	Stats ReplayStats `json:"stats"`
}

// USPMappingValidationRequest contains the parameters for validating USP adapter mappings.
type USPMappingValidationRequest struct {
	// Platform is the parser platform type (e.g., 'text', 'json', 'cef', 'gcp', 'aws')
	Platform string `json:"platform"`
	// Hostname is the default hostname for sensors (optional, defaults to 'validation-test')
	Hostname string `json:"hostname,omitempty"`
	// Mapping is a single mapping descriptor (optional)
	Mapping Dict `json:"mapping,omitempty"`
	// Mappings is a list of mapping descriptors for multi-mapping selection (optional)
	Mappings []Dict `json:"mappings,omitempty"`
	// Indexing is a list of indexing rules (optional)
	Indexing []Dict `json:"indexing,omitempty"`
	// TextInput is newline-separated text input (mutually exclusive with JSONInput)
	TextInput string `json:"text_input,omitempty"`
	// JSONInput is pre-parsed JSON input array (mutually exclusive with TextInput)
	JSONInput []Dict `json:"json_input,omitempty"`
}

// USPMappingValidationResponse contains the result of a USP mapping validation.
type USPMappingValidationResponse struct {
	// Results contains the list of successfully parsed events
	Results []Dict `json:"results,omitempty"`
	// Errors contains the list of errors encountered during validation
	Errors []string `json:"errors,omitempty"`
}

// ValidateLCQLQuery validates an LCQL query syntax without executing it.
// This method sends the query to the replay service with the is_validation flag set to true.
//
// Note: This only validates query syntax. To get billing estimates, use EstimateLCQLQueryBilling.
//
// Parameters:
//   - query: The LCQL query string to validate.
//
// Returns:
//   - *ValidationResponse: Contains validation result (success/error).
//   - error: An error if the validation request fails.
//
// Example:
//
//	result, err := org.ValidateLCQLQuery("2025-01-01 to 2025-01-15 | * | * | event/FILE_PATH ends with '.exe'")
//	if err != nil {
//	    return err
//	}
//	if result.Error != "" {
//	    fmt.Printf("Query validation failed: %s\n", result.Error)
//	}
func (org *Organization) ValidateLCQLQuery(query string) (*ValidationResponse, error) {
	return org.ValidateLCQLQueryWithContext(context.Background(), query)
}

// ValidateLCQLQueryWithContext validates an LCQL query with a context for cancellation.
// See ValidateLCQLQuery for full documentation.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - query: The LCQL query string to validate.
//
// Returns:
//   - *ValidationResponse: Contains validation result and billing estimate if available.
//   - error: An error if the validation request fails.
func (org *Organization) ValidateLCQLQueryWithContext(ctx context.Context, query string) (*ValidationResponse, error) {
	// Get replay URL from organization
	urls, err := org.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization URLs: %v", err)
	}

	replayURL, ok := urls["replay"]
	if !ok {
		return nil, fmt.Errorf("replay URL not found in organization URLs")
	}

	// Build the request body for LCQL validation
	requestBody := map[string]interface{}{
		"oid":           org.GetOID(),
		"query":         query,
		"is_validation": true,
		"limit_event":   0,
		"limit_eval":    0,
		"event_source": map[string]interface{}{
			"stream": "event",
			"sensor_events": map[string]interface{}{
				"cursor": "",
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
		Timeout: 30 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute validation request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response using the raw response structure that includes stats
	// The replay endpoint returns stats with billing information
	var rawResponse lcqlValidationRawResponse
	if err := json.Unmarshal(respBody, &rawResponse); err != nil {
		// If we can't parse the response, check if it's an HTTP error
		if httpResp.StatusCode != http.StatusOK {
			return &ValidationResponse{
				Error: fmt.Sprintf("validation request failed with status %d: %s", httpResp.StatusCode, string(respBody)),
			}, nil
		}
		return nil, fmt.Errorf("failed to parse validation response: %v", err)
	}

	// Build the validation response
	// Note: is_validation: true only validates syntax, it does not return billing stats.
	// Use EstimateLCQLQueryBilling for billing estimates.
	response := &ValidationResponse{
		Error: rawResponse.Error,
	}

	// Check for errors in the response
	if response.Error != "" {
		return response, nil
	}

	// Success
	response.Success = true
	return response, nil
}

// EstimateLCQLQueryBilling returns the billing estimate for an LCQL query without executing it.
// This method sends the query to the replay service with is_dry_run: true to get the estimated
// number of events that would be billed vs free, based on the query's time range.
//
// Note: This is separate from ValidateLCQLQuery because billing estimation requires
// is_validation: false (to actually count events) while validation uses is_validation: true
// (which only checks syntax).
//
// Parameters:
//   - query: The LCQL query string to estimate billing for.
//
// Returns:
//   - *BillingEstimate: Contains estimated billed and free event counts.
//   - error: An error if the estimation request fails.
//
// Example:
//
//	estimate, err := org.EstimateLCQLQueryBilling("2025-01-01 to 2025-06-01 | * | * | *")
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Estimated billed events: %d, free events: %d\n",
//	    estimate.BilledEvents, estimate.FreeEvents)
func (org *Organization) EstimateLCQLQueryBilling(query string) (*BillingEstimate, error) {
	return org.EstimateLCQLQueryBillingWithContext(context.Background(), query)
}

// EstimateLCQLQueryBillingWithContext returns billing estimate with a context for cancellation.
// See EstimateLCQLQueryBilling for full documentation.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - query: The LCQL query string to estimate billing for.
//
// Returns:
//   - *BillingEstimate: Contains estimated billed and free event counts.
//   - error: An error if the estimation request fails.
func (org *Organization) EstimateLCQLQueryBillingWithContext(ctx context.Context, query string) (*BillingEstimate, error) {
	// Get replay URL from organization
	urls, err := org.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization URLs: %v", err)
	}

	replayURL, ok := urls["replay"]
	if !ok {
		return nil, fmt.Errorf("replay URL not found in organization URLs")
	}

	// Build the request body for billing estimation
	// Key difference from validation: is_validation: false, is_dry_run: true
	// This makes the service count events without actually processing them
	requestBody := map[string]interface{}{
		"oid":           org.GetOID(),
		"query":         query,
		"is_validation": false,
		"is_dry_run":    true,
		"limit_event":   0,
		"limit_eval":    0,
		"event_source": map[string]interface{}{
			"stream": "event",
			"sensor_events": map[string]interface{}{
				"cursor": "",
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

	// Execute the request with longer timeout for billing estimation
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute billing estimate request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response
	var rawResponse lcqlValidationRawResponse
	if err := json.Unmarshal(respBody, &rawResponse); err != nil {
		if httpResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("billing estimate request failed with status %d: %s", httpResp.StatusCode, string(respBody))
		}
		return nil, fmt.Errorf("failed to parse billing estimate response: %v", err)
	}

	// Check for errors in the response
	if rawResponse.Error != "" {
		return nil, fmt.Errorf("billing estimate failed: %s", rawResponse.Error)
	}

	// Return the billing estimate from stats
	return &BillingEstimate{
		BilledEvents: rawResponse.Stats.BilledFor,
		FreeEvents:   rawResponse.Stats.NotBilledFor,
	}, nil
}

// ValidateDRRule validates a Detection & Response rule without executing it.
// This method sends the rule to the replay service with a minimal dummy event to validate the rule structure.
//
// The rule parameter should be a Dict containing "detect" and/or "respond" keys.
//
// Example:
//
//	rule := lc.Dict{
//	    "detect": lc.Dict{
//	        "event": "NEW_PROCESS",
//	        "op": "is",
//	        "path": "event/FILE_PATH",
//	        "value": "*/cmd.exe",
//	    },
//	    "respond": lc.List{
//	        lc.Dict{"action": "report", "name": "suspicious_process"},
//	    },
//	}
//	result, err := org.ValidateDRRule(rule)
//	if err != nil {
//	    return err
//	}
//	if result.Error != "" {
//	    fmt.Printf("Rule validation failed: %s\n", result.Error)
//	}
func (org *Organization) ValidateDRRule(rule Dict) (*ValidationResponse, error) {
	return org.ValidateDRRuleWithContext(context.Background(), rule)
}

// ValidateDRRuleWithContext validates a D&R rule with a context for cancellation.
func (org *Organization) ValidateDRRuleWithContext(ctx context.Context, rule Dict) (*ValidationResponse, error) {
	// Get replay URL from organization
	urls, err := org.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization URLs: %v", err)
	}

	replayURL, ok := urls["replay"]
	if !ok {
		return nil, fmt.Errorf("replay URL not found in organization URLs")
	}

	// Build the request body for D&R validation
	// Use a minimal dummy event to validate the rule structure
	requestBody := map[string]interface{}{
		"oid": org.GetOID(),
		"rule_source": map[string]interface{}{
			"rule_name": "",
			"namespace": "",
			"rule":      rule,
		},
		"event_source": map[string]interface{}{
			"stream": "event",
			"sensor_events": map[string]interface{}{
				"sid":        "",
				"start_time": 0,
				"end_time":   0,
			},
			"events": []interface{}{
				map[string]interface{}{
					"event":   map[string]interface{}{},
					"routing": map[string]interface{}{},
				},
			},
		},
		"trace":       false,
		"limit_event": 0,
		"limit_eval":  0,
		"is_dry_run":  false,
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
		Timeout: 30 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute validation request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response
	var response ValidationResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		// If we can't parse the response, check if it's an HTTP error
		if httpResp.StatusCode != http.StatusOK {
			return &ValidationResponse{
				Error: fmt.Sprintf("validation request failed with status %d: %s", httpResp.StatusCode, string(respBody)),
			}, nil
		}
		return nil, fmt.Errorf("failed to parse validation response: %v", err)
	}

	// Check for errors in the response
	if response.Error != "" {
		return &response, nil
	}

	// Success
	response.Success = true
	return &response, nil
}

// ValidateUSPMapping validates a USP adapter mapping configuration.
// This method sends the mapping to the API to validate that it can correctly parse input data.
//
// Example:
//
//	req := lc.USPMappingValidationRequest{
//	    Platform: "text",
//	    Mapping: lc.Dict{
//	        "parsing": lc.Dict{
//	            "fmt": "regex",
//	            "re": "(?P<timestamp>\\S+)\\s+(?P<message>.*)",
//	        },
//	    },
//	    TextInput: "2024-01-01T12:00:00Z test message",
//	}
//	result, err := org.ValidateUSPMapping(req)
//	if err != nil {
//	    return err
//	}
//	if len(result.Errors) > 0 {
//	    fmt.Printf("Mapping validation failed: %v\n", result.Errors)
//	}
func (org *Organization) ValidateUSPMapping(req USPMappingValidationRequest) (*USPMappingValidationResponse, error) {
	return org.ValidateUSPMappingWithContext(context.Background(), req)
}

// ValidateUSPMappingWithContext validates a USP adapter mapping with a context for cancellation.
func (org *Organization) ValidateUSPMappingWithContext(ctx context.Context, req USPMappingValidationRequest) (*USPMappingValidationResponse, error) {
	// Marshal the request body
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Build the URL
	url := fmt.Sprintf("%s/%s/usp/validate/%s", rootURL, currentAPIVersion, org.GetOID())

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
		Timeout: 30 * time.Second,
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute validation request: %v", err)
	}
	defer httpResp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response
	var response USPMappingValidationResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		// If we can't parse the response, check if it's an HTTP error
		if httpResp.StatusCode != http.StatusOK {
			return &USPMappingValidationResponse{
				Errors: []string{fmt.Sprintf("validation request failed with status %d: %s", httpResp.StatusCode, string(respBody))},
			}, nil
		}
		return nil, fmt.Errorf("failed to parse validation response: %v", err)
	}

	return &response, nil
}
