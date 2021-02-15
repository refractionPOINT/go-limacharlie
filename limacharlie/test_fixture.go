package limacharlie

import (
	"os"

	"github.com/stretchr/testify/assert"
)

func getTestClientOpts(a *assert.Assertions) ClientOptions {
	oid := os.Getenv("LC_OID")
	key := os.Getenv("LC_API_KEY")
	a.NotEmpty(key)
	a.NotEmpty(oid)
	return ClientOptions{
		OID:    oid,
		APIKey: key,
	}
}

func getTestClientFromEnv(a *assert.Assertions) Client {
	c, err := NewClient(getTestClientOpts(a))
	a.NoError(err)
	return *c
}

func getTestOrgFromEnv(a *assert.Assertions) *Organization {
	org, err := NewOrganization(getTestClientOpts(a))
	a.NoError(err)
	return org
}
