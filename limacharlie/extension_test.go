package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExtensionSchema(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test GetExtensionSchema with ext-reliable-tasking extension
	// This is a core extension that has a well-defined schema
	schema, err := org.GetExtensionSchema("ext-reliable-tasking")
	a.NoError(err, "GetExtensionSchema should not return an error")
	a.NotNil(schema, "schema should not be nil")
	a.NotEmpty(schema, "schema should not be empty")

	t.Logf("ext-reliable-tasking schema: %+v", schema)
}
