package limacharlie

import (
	"os"

	"github.com/stretchr/testify/assert"
)

func getClientOptionsFromEnv(a *assert.Assertions) ClientOptions {
	oid := os.Getenv("_OID")
	key := os.Getenv("_KEY")
	return getClientOptions(a, oid, key)
}

func getClientOptions(a *assert.Assertions, oid string, key string) ClientOptions {
	// Looks like test credentials are not configured.
	a.NotEmpty(key)
	a.NotEmpty(oid)

	return ClientOptions{
		OID:    oid,
		APIKey: key,
	}
}

func getTestClientFromEnv(a *assert.Assertions) Client {
	c, err := NewClient(getClientOptionsFromEnv(a))
	a.NoError(err)
	return *c
}

func getTestOrgFromEnv(a *assert.Assertions) *Organization {
	org, err := NewOrganization(getClientOptionsFromEnv(a))
	a.NoError(err)
	return org
}
