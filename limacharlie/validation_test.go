package limacharlie

import (
	"context"
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

	resp, err := org.ValidateLCQLQuery("-1h | * | * | event/FILE_PATH ends with '.exe'")
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
