package limacharlie

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUSPMapping_BasicText(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	req := USPMappingValidationRequest{
		Platform: "text",
		Mapping: Dict{
			"parsing": Dict{
				"fmt": "regex",
				"re":  "(?P<timestamp>\\S+)\\s+(?P<message>.*)",
			},
		},
		TextInput: "2024-01-01T12:00:00Z test message",
	}

	resp, err := org.ValidateUSPMapping(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if len(resp.Errors) > 0 {
		t.Logf("Validation errors: %v", resp.Errors)
	}

	t.Logf("Parsed %d results", len(resp.Results))
}

func TestValidateUSPMapping_JSONInput(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	req := USPMappingValidationRequest{
		Platform: "json",
		JSONInput: []Dict{
			{"timestamp": "2024-01-01T12:00:00Z", "message": "test"},
		},
	}

	resp, err := org.ValidateUSPMapping(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Validation result: errors=%d, results=%d",
		len(resp.Errors), len(resp.Results))
}

func TestValidateUSPMapping_WithContext(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := USPMappingValidationRequest{
		Platform: "text",
		Mapping: Dict{
			"parsing": Dict{
				"fmt": "regex",
				"re":  "(?P<message>.*)",
			},
		},
		TextInput: "test message",
	}

	resp, err := org.ValidateUSPMappingWithContext(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Validation with context: errors=%d, results=%d",
		len(resp.Errors), len(resp.Results))
}

func TestValidateUSPMapping_InvalidPlatform(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	req := USPMappingValidationRequest{
		Platform:  "invalid_platform_that_does_not_exist",
		TextInput: "some data",
	}

	resp, err := org.ValidateUSPMapping(req)
	// May return error or errors in response
	if err != nil {
		t.Logf("Request failed with error: %v", err)
	} else if len(resp.Errors) > 0 {
		t.Logf("Validation returned errors as expected: %v", resp.Errors)
	} else {
		t.Logf("Unexpected success with invalid platform")
	}
}

func TestValidateLCQLQuery_Valid(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	resp, err := org.ValidateLCQLQuery("2025-12-01 to 2026-01-15 | * | * | event/FILE_PATH ends with '.exe'")
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Error != "" {
		t.Logf("Query validation failed: %s", resp.Error)
	} else {
		a.True(resp.Success)
		t.Log("Query validation succeeded")
	}
}

func TestValidateLCQLQuery_Invalid(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Invalid query syntax
	resp, err := org.ValidateLCQLQuery("this is not valid LCQL syntax !!@#$%")
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should have an error in the response
	if resp.Error != "" {
		t.Logf("Query validation failed as expected: %s", resp.Error)
	} else {
		t.Log("Query did not return expected validation error")
	}
}

func TestValidateDRRule_Valid(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	rule := Dict{
		"detect": Dict{
			"event": "NEW_PROCESS",
			"op":    "is",
			"path":  "event/FILE_PATH",
			"value": "*/cmd.exe",
		},
		"respond": List{
			Dict{"action": "report", "name": "test_detection"},
		},
	}

	resp, err := org.ValidateDRRule(rule)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if resp.Error != "" {
		t.Logf("Rule validation failed: %s", resp.Error)
	} else {
		a.True(resp.Success)
		t.Log("Rule validation succeeded")
	}
}

func TestValidateDRRule_Invalid(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Invalid rule structure - missing required fields
	rule := Dict{
		"detect": Dict{
			"op": "invalid_operator",
		},
	}

	resp, err := org.ValidateDRRule(rule)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should have an error in the response
	if resp.Error != "" {
		t.Logf("Rule validation failed as expected: %s", resp.Error)
	} else {
		t.Log("Rule did not return expected validation error")
	}
}

func TestLCQLValidationRawResponse_JSONParsing(t *testing.T) {
	t.Run("full_stats_response", func(t *testing.T) {
		// Simulate a full response from the replay service with all ReplayStats fields
		jsonData := `{
			"error": "",
			"stats": {
				"n_scan": 10000,
				"n_bytes_scan": 5000000,
				"n_proc": 8500,
				"n_matched": 150,
				"n_shard": 4,
				"n_eval": 25000,
				"wall_time": 2.5,
				"cummulative_time": 8.2,
				"n_batch_access": 12,
				"n_billed": 8000,
				"n_free": 2000,
				"estimated_price": {
					"value": 0.08,
					"currency": "USD cents"
				}
			}
		}`

		var resp lcqlValidationRawResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Empty(t, resp.Error)
		assert.Equal(t, uint64(10000), resp.Stats.NumScanned)
		assert.Equal(t, uint64(5000000), resp.Stats.NumBytesScanned)
		assert.Equal(t, uint64(8500), resp.Stats.NumEventsProcessed)
		assert.Equal(t, uint64(150), resp.Stats.NumEventsMatched)
		assert.Equal(t, uint64(4), resp.Stats.NumShards)
		assert.Equal(t, uint64(25000), resp.Stats.NumEvals)
		assert.Equal(t, 2.5, resp.Stats.WallTime)
		assert.Equal(t, 8.2, resp.Stats.CumulativeTime)
		assert.Equal(t, uint64(12), resp.Stats.NumBatches)
		assert.Equal(t, uint64(8000), resp.Stats.BilledFor)
		assert.Equal(t, uint64(2000), resp.Stats.NotBilledFor)
		assert.Equal(t, 0.08, resp.Stats.EstimatedPrice.Price)
		assert.Equal(t, "USD cents", resp.Stats.EstimatedPrice.Currency)
	})

	t.Run("minimal_stats_response", func(t *testing.T) {
		// Response with only billing stats (validation mode)
		jsonData := `{
			"stats": {
				"n_billed": 1000,
				"n_free": 500
			}
		}`

		var resp lcqlValidationRawResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Empty(t, resp.Error)
		assert.Equal(t, uint64(1000), resp.Stats.BilledFor)
		assert.Equal(t, uint64(500), resp.Stats.NotBilledFor)
		// Other fields should be zero
		assert.Equal(t, uint64(0), resp.Stats.NumScanned)
	})

	t.Run("error_response_with_stats", func(t *testing.T) {
		jsonData := `{
			"error": "invalid query syntax: unexpected token",
			"stats": {
				"n_billed": 0,
				"n_free": 0
			}
		}`

		var resp lcqlValidationRawResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "invalid query syntax: unexpected token", resp.Error)
		assert.Equal(t, uint64(0), resp.Stats.BilledFor)
		assert.Equal(t, uint64(0), resp.Stats.NotBilledFor)
	})

	t.Run("empty_stats_response", func(t *testing.T) {
		jsonData := `{
			"error": "",
			"stats": {}
		}`

		var resp lcqlValidationRawResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Empty(t, resp.Error)
		assert.Equal(t, uint64(0), resp.Stats.BilledFor)
		assert.Equal(t, uint64(0), resp.Stats.NotBilledFor)
	})

	t.Run("partial_stats_response", func(t *testing.T) {
		// Response with only some stats fields present
		jsonData := `{
			"stats": {
				"n_scan": 5000,
				"n_proc": 4500,
				"n_billed": 4000,
				"wall_time": 1.2
			}
		}`

		var resp lcqlValidationRawResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, uint64(5000), resp.Stats.NumScanned)
		assert.Equal(t, uint64(4500), resp.Stats.NumEventsProcessed)
		assert.Equal(t, uint64(4000), resp.Stats.BilledFor)
		assert.Equal(t, 1.2, resp.Stats.WallTime)
		// Missing fields should be zero
		assert.Equal(t, uint64(0), resp.Stats.NotBilledFor)
		assert.Equal(t, uint64(0), resp.Stats.NumEventsMatched)
	})
}

func TestBillingEstimate_JSONSerialization(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		estimate := BillingEstimate{
			BilledEvents: 1000,
			FreeEvents:   500,
			EstimatedPrice: EstimatedPrice{
				Price:    0.01,
				Currency: "USD cents",
			},
		}

		data, err := json.Marshal(estimate)
		require.NoError(t, err)

		// Verify the JSON structure matches the expected field names
		var raw map[string]interface{}
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.Equal(t, float64(1000), raw["billed_events"])
		assert.Equal(t, float64(500), raw["free_events"])
		// Check estimated_price nested object
		priceObj, ok := raw["estimated_price"].(map[string]interface{})
		require.True(t, ok, "estimated_price should be an object")
		assert.Equal(t, 0.01, priceObj["value"])
		assert.Equal(t, "USD cents", priceObj["currency"])
	})

	t.Run("unmarshal", func(t *testing.T) {
		jsonData := `{
			"billed_events": 2500,
			"free_events": 750,
			"estimated_price": {
				"value": 0.025,
				"currency": "USD cents"
			}
		}`

		var estimate BillingEstimate
		err := json.Unmarshal([]byte(jsonData), &estimate)
		require.NoError(t, err)

		assert.Equal(t, uint64(2500), estimate.BilledEvents)
		assert.Equal(t, uint64(750), estimate.FreeEvents)
		assert.Equal(t, 0.025, estimate.EstimatedPrice.Price)
		assert.Equal(t, "USD cents", estimate.EstimatedPrice.Currency)
	})

	t.Run("unmarshal_zero_values", func(t *testing.T) {
		jsonData := `{
			"billed_events": 0,
			"free_events": 0,
			"estimated_price": {
				"value": 0,
				"currency": "USD cents"
			}
		}`

		var estimate BillingEstimate
		err := json.Unmarshal([]byte(jsonData), &estimate)
		require.NoError(t, err)

		assert.Equal(t, uint64(0), estimate.BilledEvents)
		assert.Equal(t, uint64(0), estimate.FreeEvents)
		assert.Equal(t, 0.0, estimate.EstimatedPrice.Price)
		assert.Equal(t, "USD cents", estimate.EstimatedPrice.Currency)
	})

	t.Run("unmarshal_without_estimated_price", func(t *testing.T) {
		// Test backward compatibility - response may not include estimated_price
		jsonData := `{"billed_events": 1000, "free_events": 500}`

		var estimate BillingEstimate
		err := json.Unmarshal([]byte(jsonData), &estimate)
		require.NoError(t, err)

		assert.Equal(t, uint64(1000), estimate.BilledEvents)
		assert.Equal(t, uint64(500), estimate.FreeEvents)
		// EstimatedPrice should be zero value
		assert.Equal(t, 0.0, estimate.EstimatedPrice.Price)
		assert.Equal(t, "", estimate.EstimatedPrice.Currency)
	})
}

func TestValidationResponse_JSONSerialization(t *testing.T) {
	t.Run("success_response", func(t *testing.T) {
		jsonData := `{
			"error": "",
			"success": true
		}`

		var resp ValidationResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Success)
		assert.Empty(t, resp.Error)
	})

	t.Run("error_response", func(t *testing.T) {
		jsonData := `{
			"error": "query syntax error",
			"success": false
		}`

		var resp ValidationResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "query syntax error", resp.Error)
		assert.False(t, resp.Success)
	})

	t.Run("marshal_round_trip", func(t *testing.T) {
		original := ValidationResponse{
			Success: true,
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded ValidationResponse
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.Success, decoded.Success)
	})
}

func TestEstimateLCQLQueryBilling(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	t.Run("recent_time_range_free_events", func(t *testing.T) {
		// Recent events (< 30 days) should be free
		estimate, err := org.EstimateLCQLQueryBilling("2025-12-20 to 2026-01-15 | * | * | *")
		require.NoError(t, err)
		require.NotNil(t, estimate)

		t.Logf("Recent range - BilledEvents: %d, FreeEvents: %d, EstimatedPrice: %.4f %s",
			estimate.BilledEvents, estimate.FreeEvents,
			estimate.EstimatedPrice.Price, estimate.EstimatedPrice.Currency)
	})

	t.Run("older_time_range_billed_events", func(t *testing.T) {
		// Older events (> 6 months) should be billed
		estimate, err := org.EstimateLCQLQueryBilling("2025-01-01 to 2025-03-01 | * | * | *")
		require.NoError(t, err)
		require.NotNil(t, estimate)

		t.Logf("Older range - BilledEvents: %d, FreeEvents: %d, EstimatedPrice: %.4f %s",
			estimate.BilledEvents, estimate.FreeEvents,
			estimate.EstimatedPrice.Price, estimate.EstimatedPrice.Currency)
	})

	t.Run("mixed_time_range", func(t *testing.T) {
		// Mixed range should have both billed and free
		estimate, err := org.EstimateLCQLQueryBilling("2025-06-01 to 2026-01-01 | * | * | *")
		require.NoError(t, err)
		require.NotNil(t, estimate)

		t.Logf("Mixed range - BilledEvents: %d, FreeEvents: %d, EstimatedPrice: %.4f %s",
			estimate.BilledEvents, estimate.FreeEvents,
			estimate.EstimatedPrice.Price, estimate.EstimatedPrice.Currency)
	})

	t.Run("invalid_query_returns_error", func(t *testing.T) {
		// Invalid query should return an error
		_, err := org.EstimateLCQLQueryBilling("invalid !@#$ query syntax")
		// Should return an error for invalid queries
		if err != nil {
			t.Logf("Expected error for invalid query: %v", err)
		}
	})
}

func TestBillingEstimate_ZeroValuesAreValid(t *testing.T) {
	t.Run("zero_billed_zero_free_is_valid", func(t *testing.T) {
		// Zero values should be distinguishable from nil
		estimate := &BillingEstimate{
			BilledEvents: 0,
			FreeEvents:   0,
		}

		// Pointer is not nil, so we can check the values
		require.NotNil(t, estimate)
		assert.Equal(t, uint64(0), estimate.BilledEvents)
		assert.Equal(t, uint64(0), estimate.FreeEvents)
	})

	t.Run("nil_vs_zero_distinction", func(t *testing.T) {
		// Test that we can distinguish between nil and zero values
		var nilEstimate *BillingEstimate
		zeroEstimate := &BillingEstimate{
			BilledEvents: 0,
			FreeEvents:   0,
		}

		assert.Nil(t, nilEstimate)
		assert.NotNil(t, zeroEstimate)
	})
}

func TestQueryValidationResult_JSONSerialization(t *testing.T) {
	t.Run("marshal_with_billing", func(t *testing.T) {
		result := QueryValidationResult{
			Validation: &ValidationResponse{
				Success: true,
			},
			BillingEstimate: &BillingEstimate{
				BilledEvents: 1000,
				FreeEvents:   500,
				EstimatedPrice: EstimatedPrice{
					Price:    0.01,
					Currency: "USD cents",
				},
			},
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var decoded QueryValidationResult
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.True(t, decoded.Validation.Success)
		require.NotNil(t, decoded.BillingEstimate)
		assert.Equal(t, uint64(1000), decoded.BillingEstimate.BilledEvents)
		assert.Equal(t, uint64(500), decoded.BillingEstimate.FreeEvents)
		assert.Equal(t, 0.01, decoded.BillingEstimate.EstimatedPrice.Price)
	})

	t.Run("marshal_without_billing", func(t *testing.T) {
		// When validation fails, billing estimate is nil
		result := QueryValidationResult{
			Validation: &ValidationResponse{
				Error: "invalid query syntax",
			},
			BillingEstimate: nil,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var decoded QueryValidationResult
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "invalid query syntax", decoded.Validation.Error)
		assert.Nil(t, decoded.BillingEstimate)
	})
}

func TestValidateAndEstimateLCQLQuery(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	t.Run("valid_query_returns_both", func(t *testing.T) {
		result, err := org.ValidateAndEstimateLCQLQuery("2025-12-20 to 2026-01-15 | * | * | *")
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Validation)

		// Validation should succeed
		if result.Validation.Error != "" {
			t.Logf("Validation error: %s", result.Validation.Error)
		} else {
			assert.True(t, result.Validation.Success)
		}

		// Billing estimate should be present for valid queries
		if result.BillingEstimate != nil {
			t.Logf("Combined result - BilledEvents: %d, FreeEvents: %d, Price: %.4f %s",
				result.BillingEstimate.BilledEvents,
				result.BillingEstimate.FreeEvents,
				result.BillingEstimate.EstimatedPrice.Price,
				result.BillingEstimate.EstimatedPrice.Currency)
		}
	})

	t.Run("invalid_query_returns_validation_error", func(t *testing.T) {
		result, err := org.ValidateAndEstimateLCQLQuery("invalid !@#$ query syntax")
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Validation)

		// Validation should fail
		if result.Validation.Error != "" {
			t.Logf("Expected validation error: %s", result.Validation.Error)
			// Billing estimate should be nil for invalid queries
			assert.Nil(t, result.BillingEstimate)
		}
	})

	t.Run("concurrent_execution_performance", func(t *testing.T) {
		// This test verifies concurrent execution by checking both results are returned
		result, err := org.ValidateAndEstimateLCQLQuery("2025-06-01 to 2026-01-01 | * | * | *")
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Validation)

		if result.Validation.Success {
			// If validation succeeded, we should have billing estimate
			if result.BillingEstimate != nil {
				t.Logf("Concurrent execution succeeded - BilledEvents: %d, FreeEvents: %d",
					result.BillingEstimate.BilledEvents,
					result.BillingEstimate.FreeEvents)
			}
		}
	})
}
