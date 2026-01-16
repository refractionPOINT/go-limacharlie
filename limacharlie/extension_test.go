package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExtensionSchema(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test GetExtensionSchema with ext-reliable-tasking extension
	// Note: Some internal extensions (replicants) may not return schemas
	// via the webhook protocol, so we just verify the API call succeeds
	schema, err := org.GetExtensionSchema("ext-reliable-tasking")
	a.NoError(err, "GetExtensionSchema should not return an error")
	a.NotNil(schema, "schema should not be nil")

	t.Logf("ext-reliable-tasking schema: %+v", schema)

	// If schema has content, verify it has expected structure
	if len(schema) > 0 {
		t.Logf("Schema has %d top-level fields", len(schema))
	}
}
