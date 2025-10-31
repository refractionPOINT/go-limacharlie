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

	// Test with Windows platform
	schemas, err := org.GetSchemasForPlatform("windows")
	a.NoError(err, "GetSchemasForPlatform(windows) should succeed")
	a.NotNil(schemas)
	a.Greater(len(schemas.EventTypes), 0, "should have event types for windows platform")
	t.Logf("Windows platform has %d event types", len(schemas.EventTypes))

	// Test with Linux platform
	schemasLinux, err := org.GetSchemasForPlatform("linux")
	if err != nil {
		t.Errorf("GetSchemasForPlatform(linux): %v", err)
		return
	}
	a.NotNil(schemasLinux)
	t.Logf("Linux platform has %d event types", len(schemasLinux.EventTypes))

	// Test with macOS platform
	schemasMac, err := org.GetSchemasForPlatform("macos")
	if err != nil {
		t.Errorf("GetSchemasForPlatform(macos): %v", err)
		return
	}
	a.NotNil(schemasMac)
	t.Logf("macOS platform has %d event types", len(schemasMac.EventTypes))

	// Test with Chrome platform
	schemasChrome, err := org.GetSchemasForPlatform("chrome")
	if err != nil {
		t.Errorf("GetSchemasForPlatform(chrome): %v", err)
		return
	}
	a.NotNil(schemasChrome)
	t.Logf("Chrome platform has %d event types", len(schemasChrome.EventTypes))
}

// TestGetPlatformNames tests retrieving the list of platform names
func TestGetPlatformNames(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	platforms, err := org.GetPlatformNames()
	a.NoError(err, "GetPlatformNames should succeed")
	a.NotNil(platforms)
	if len(platforms) == 0 {
		t.Log("GetPlatformNames() returned empty list - may not be available in test environment")
		return
	}

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
