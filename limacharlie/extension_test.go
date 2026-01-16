package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExtensionSchema(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	schema, err := org.GetExtensionSchema("ext-exfil")
	a.NoError(err, "GetExtensionSchema should not return an error")
	a.NotNil(schema, "schema should not be nil")
	a.NotEmpty(schema, "schema should not be empty")

	// Verify the schema has expected fields
	_, hasConfigSchema := schema["config_schema"]
	a.True(hasConfigSchema, "schema should have config_schema field")
}
