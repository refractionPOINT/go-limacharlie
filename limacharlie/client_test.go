package limacharlie

import (
	"os"
	"testing"
	"time"
)

func getTestOptions(t *testing.T) ClientOptions {
	testOID := os.Getenv("_OID")
	testKey := os.Getenv("_KEY")

	if testKey == "" || testOID == "" {
		// Looks like test credentials
		// are not configured.
		t.Errorf("test credentials not provided")
		t.FailNow()
	}

	return ClientOptions{
		OID:    testOID,
		APIKey: testKey,
	}
}

func getTestClient(t *testing.T) *Client {
	c, err := NewClient(getTestOptions(t))
	if err != nil {
		t.Errorf("failed to create client: %v", err)
		t.FailNow()
	}
	return c
}

func TestClientAndJWT(t *testing.T) {
	c := getTestClient(t)

	if err := c.refreshJWT(60 * 30 * time.Second); err != nil {
		t.Errorf("failed to get jwt: %v", err)
	}
}

func TestWho(t *testing.T) {
	c := getTestClient(t)

	data, err := c.whoAmI()
	if err != nil {
		t.Errorf("failed to get JWT info: %v", err)
	}
	if ident, ok := data["ident"]; !ok || ident == "" {
		t.Errorf("error getting basic JWT info: %+v", data)
	}
}
