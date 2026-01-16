package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExtensionSchema(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test GetExtensionSchema with ext-exfil extension
	schema, err := org.GetExtensionSchema("ext-exfil")
	a.NoError(err, "GetExtensionSchema should not return an error")
	a.NotNil(schema, "schema should not be nil")
	a.NotEmpty(schema, "schema should not be empty")

	t.Logf("ext-exfil schema: %+v", schema)
}
