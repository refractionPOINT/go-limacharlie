package limacharlie

import (
	"os"
	"testing"
)

func getTestOptionsFromEnv(t *testing.T) ClientOptions {
	oid := os.Getenv("_OID")
	key := os.Getenv("_KEY")
	return getTestOptions(t, oid, key)
}

func getTestOptions(t *testing.T, oid string, key string) ClientOptions {
	// Looks like test credentials are not configured.
	assert(t, key == "" || oid == "", "test credentials not provided")

	return ClientOptions{
		OID:    oid,
		APIKey: key,
	}
}

func getTestClientFromEnv(t *testing.T) *Client {
	c, err := NewClient(getTestOptionsFromEnv(t))
	assertIsNotError(t, err, "failed to create client")
	return c
}

func getTestOrgFromEnv(t *testing.T) Organization {
	org, err := MakeOrganization(getTestOptionsFromEnv(t))
	assertIsNotError(t, err, "failed to make organization")
	return org
}
