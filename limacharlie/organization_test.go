package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthorize(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	_, _, err := org.Authorize([]string{"org.get"})
	a.EqualError(err, "Org should have 'org.get' permission")
}

func TestAuthorizeMissingPermission(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	_, _, err := org.Authorize([]string{"org.get", "foo.bar"})
	a.EqualError(err, "Unauthorized, missing permissions: 'foo.bar'")
}
