package limacharlie

import (
	"testing"
)

func TestAuthorize(t *testing.T) {
	org := getTestOrgFromEnv(t)
	_, err := org.Authorize([]string{"org.get"})
	assertIsNotError(t, err, "permissions missing")
}
