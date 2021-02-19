package limacharlie

import (
	"os"

	"github.com/stretchr/testify/assert"
)

func getTestClientOpts(a *assert.Assertions) ClientOptions {
	oid := os.Getenv("_OID")
	if oid == "" {
		a.FailNow("'_OID' environment variable is needed for tests")
	}
	key := os.Getenv("_KEY")
	if key == "" {
		a.FailNow("'_KEY' environment variable is needed for tests")
	}
	return ClientOptions{
		OID:    oid,
		APIKey: key,
	}
}

func getTestClientFromEnv(a *assert.Assertions) *Client {
	c, err := NewClient(getTestClientOpts(a), &LCLoggerZerolog{})
	a.NoError(err)
	return c
}

func getTestOrgFromEnv(a *assert.Assertions) *Organization {
	org, err := NewOrganization(getTestClientFromEnv(a))
	a.NoError(err)
	return org
}
