package limacharlie

import (
	"os"
	"testing"
	"time"
)

func getTestOptions(t *testing.T) ClientOptions {
	testOID := os.Getenv("_OID")
	testKey := os.Getenv("_KEY")

	// Looks like test credentials are not configured.
	assert(t, testKey == "" || testOID == "", "test credentials not provided")

	return ClientOptions{
		OID:    testOID,
		APIKey: testKey,
	}
}

func getTestClient(t *testing.T) *Client {
	c, err := NewClient(getTestOptions(t))
	assertIsNotError(t, err, "failed to create client")
	return c
}

func TestClientAndJWT(t *testing.T) {
	c := getTestClient(t)
	err := c.refreshJWT(60 * 30 * time.Second)
	assertIsNotError(t, err, "failed to get jwt")
}

func TestWho(t *testing.T) {
	c := getTestClient(t)
	who, err := c.whoAmI()
	assertIsNotError(t, err, "failed to get WhoAmI response")
	assert(t, *who.Identity == "", "error getting basic JWT info")
}
