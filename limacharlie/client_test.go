package limacharlie

import (
	"testing"
	"time"
)

func TestClientAndJWT(t *testing.T) {
	c := getTestClientFromEnv(t)
	err := c.refreshJWT(60 * 30 * time.Second)
	assertIsNotError(t, err, "failed to get jwt")
}

func TestWho(t *testing.T) {
	c := getTestClientFromEnv(t)
	who, err := c.whoAmI()
	assertIsNotError(t, err, "failed to get WhoAmI response")
	assert(t, *who.Identity == "", "error getting basic JWT info")
}
