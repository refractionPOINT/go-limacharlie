package limacharlie

import (
	"testing"
)

func TestAuthorize(t *testing.T) {
	org := getTestOrgFromEnv(t)
	_, err := org.Authorize([]string{"org.get"})
	assertIsNotError(t, err, "Org should have 'org.get' permission")
}

func TestAuthorizeMissingPermission(t *testing.T) {
	org := getTestOrgFromEnv(t)
	_, err := org.Authorize([]string{"org.get", "foo.bar"})
	assertIsErrorMessage(t, err, "Unauthorized, missing permissions: 'foo.bar'")
}
