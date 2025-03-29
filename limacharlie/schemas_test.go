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
	eventType := schemas.EventTypes[0] // Use the first event type from the list
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
