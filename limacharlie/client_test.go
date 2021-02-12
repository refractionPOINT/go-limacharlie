package limacharlie

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientAndJWT(t *testing.T) {
	a := assert.New(t)
	c := getTestClientFromEnv(a)
	err := c.refreshJWT(60 * 30 * time.Second)
	a.NoError(err)
}

func TestWho(t *testing.T) {
	a := assert.New(t)
	c := getTestClientFromEnv(a)
	who, err := c.whoAmI()
	a.NoError(err)
	a.NotEmpty(*who.Identity)
}
