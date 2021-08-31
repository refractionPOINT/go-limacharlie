package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/stretchr/testify/assert"
)

type OrgTestSuite struct {
	suite.Suite
}

func TestOrgSuite(t *testing.T) {
	suite.Run(t, new(OrgTestSuite))
}

func (s *OrgTestSuite) TestAuthorize() {
	org := getTestOrgFromEnv(s.Assertions)
	_, _, err := org.Authorize([]string{"org.get"})
	s.NoError(err)
}

func (s *OrgTestSuite) TestAuthorizeMissingPermission() {
	org := getTestOrgFromEnv(s.Assertions)
	_, _, err := org.Authorize([]string{"org.get", "foo.bar"})
	s.EqualError(err, "unauthorized, missing permissions: '[\"foo.bar\"]'")
}

func (s *OrgTestSuite) TestOrgURLs() {
	a := assert.New(s)
	org := getTestOrgFromEnv(a)

	urls, err := org.GetURLs()
	a.NoError(err)
	if len(urls) != 0 {
		s.Errorf("not enough URLs found: %+v", urls)
	}
}
