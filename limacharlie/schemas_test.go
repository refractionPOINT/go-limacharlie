package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemas(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test GetSchemas
	schemas, err := org.GetSchemas()
	if err != nil {
		t.Errorf("GetSchemas: %v", err)
	}
	if schemas == nil {
		t.Error("no schemas returned")
		return
	}
	if len(schemas.EventTypes) == 0 {
		t.Error("no event types listed in schemas")
		return
	}

	// Test GetSchema with a known event type
	eventType := "evt:CODE_IDENTITY"
	schema, err := org.GetSchema(eventType)
	if err != nil {
		t.Errorf("GetSchema(%s): %v", eventType, err)
	}
	if schema == nil {
		t.Errorf("no schema returned for event type: %s", eventType)
		return
	}
	if schema.Schema.EventType == "" {
		t.Errorf("missing event type in schema: %+v", schema)
	}
	if len(schema.Schema.Elements) == 0 {
		t.Errorf("no elements in schema: %+v", schema)
	}

	// Test GetSchema with an invalid event type
	_, err = org.GetSchema("invalid_event_type")
	if err == nil {
		t.Error("expected error for invalid event type, got nil")
	}
}

// TestGetSchemasForPlatform tests retrieving schemas filtered by platform
func TestGetSchemasForPlatform(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test with Linux platform
	schemasLinux, err := org.GetSchemasForPlatform("linux")
	a.NoError(err)
	a.NotNil(schemasLinux, "schemas should not be nil")
	a.Greater(len(schemasLinux.EventTypes), 0, "should have event types for linux platform")
	t.Logf("Linux platform has %d event types", len(schemasLinux.EventTypes))
}

// TestGetPlatformNames tests retrieving the list of platform names
func TestGetPlatformNames(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	platforms, err := org.GetPlatformNames()
	a.NoError(err, "GetPlatformNames should not return an error")
	a.NotNil(platforms, "platforms should not be nil")
	a.Greater(len(platforms), 0, "should have at least one platform")

	// Log all platforms
	t.Logf("Available platforms: %v", platforms)

	// Check for expected platforms
	expectedPlatforms := []string{"windows", "linux", "macos", "chrome"}
	for _, expected := range expectedPlatforms {
		found := false
		for _, platform := range platforms {
			if platform == expected {
				found = true
				break
			}
		}
		if found {
			t.Logf("Found expected platform: %s", expected)
		}
	}
}
