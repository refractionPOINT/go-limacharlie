package limacharlie

import (
	"os"
	"testing"
	"time"
)

func getTestOptions() *ClientOptions {
	testOID := os.Getenv("_OID")
	testKey := os.Getenv("_KEY")

	if testKey == "" || testOID == "" {
		// Looks like test credentials
		// are not configured.
		return nil
	}

	return &ClientOptions{
		OID:    testOID,
		APIKey: testKey,
	}
}

func TestClientAndJWT(t *testing.T) {
	o := getTestOptions()
	if o == nil {
		t.Errorf("test credentials not provided")
		return
	}

	c, err := NewClient(*o)
	if err != nil {
		t.Errorf("failed to create client: %v", err)
	}

	if err := c.refreshJWT(60 * 30 * time.Second); err != nil {
		t.Errorf("failed to get jwt: %v", err)
	}
}
