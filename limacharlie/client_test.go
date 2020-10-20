package limacharlie

import (
	"os"
	"testing"
	"time"
)

func assert(t *testing.T, message string) {
	t.Errorf(message)
	t.FailNow()
}

func AssertIsNotError(t *testing.T, err error, message string) {
	if err != nil {
		t.Errorf("%s: %v", message, err)
		t.FailNow()
	}
}

func getTestOptions(t *testing.T) ClientOptions {
	testOID := os.Getenv("_OID")
	testKey := os.Getenv("_KEY")

	if testKey == "" || testOID == "" {
		// Looks like test credentials
		// are not configured.
		assert(t, "test credentials not provided")
	}

	return ClientOptions{
		OID:    testOID,
		APIKey: testKey,
	}
}

func getTestClient(t *testing.T) *Client {
	c, err := NewClient(getTestOptions(t))
	AssertIsNotError(t, err, "failed to create client")
	return c
}

func TestClientAndJWT(t *testing.T) {
	c := getTestClient(t)
	err := c.refreshJWT(60 * 30 * time.Second)
	AssertIsNotError(t, err, "failed to get jwt")
}

func TestWho(t *testing.T) {
	c := getTestClient(t)
	who, err := c.whoAmI()
	AssertIsNotError(t, err, "failed to get WhoAmI response")

	if *who.Identity == "" {
		t.Errorf("error getting basic JWT info")
	}
}
