package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetGroupStruct tests the GetGroup constructor
// This test does not require API credentials as it only tests the struct creation
func TestGetGroupStruct(t *testing.T) {
	a := assert.New(t)

	// Create a minimal client for testing the GetGroup constructor
	c := &Client{
		options: ClientOptions{
			OID:    "test-oid",
			APIKey: "test-key",
		},
	}

	testGID := "test-gid-12345"
	group := c.GetGroup(testGID)

	a.NotNil(group)
	a.Equal(testGID, group.GID)
	a.NotNil(group.client)
	a.Equal(c, group.client)

	t.Log("GetGroup constructor works correctly")
}
