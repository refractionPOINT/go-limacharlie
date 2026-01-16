package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExtensionSchema(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test GetExtensionSchema with ext-exfil extension which has a known schema
	schema, err := org.GetExtensionSchema("ext-exfil")
	a.NoError(err, "GetExtensionSchema should not return an error")
	a.NotNil(schema, "schema should not be nil")
	a.NotEmpty(schema, "schema should not be empty")

	t.Logf("ext-exfil schema keys: %v", getKeys(schema))

	// Verify the schema has expected fields
	_, hasConfigSchema := schema["config_schema"]
	a.True(hasConfigSchema, "schema should have config_schema field")
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
