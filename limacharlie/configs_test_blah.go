package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	testConfig = `
oid: 11111111-2222-3333-4444-555555555555
api_key: 31111111-2222-3333-4444-555555555555
env:
  ttt:
    oid: 41111111-2222-3333-4444-555555555555
    uid: 51111111-2222-3333-4444-555555555555
    api_key: 61111111-2222-3333-4444-555555555555
  vvv:
    oid: 71111111-2222-3333-4444-555555555555
    uid: 81111111-2222-3333-4444-555555555555
    api_key: 91111111-2222-3333-4444-555555555555`
)

type ConfigsTestSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigsTestSuite))
}

func (s *ConfigsTestSuite) TestFromConfigStringDefault() {
	o := ClientOptions{}

	s.NoError(o.FromConfigString([]byte(testConfig), ""))
	s.Equal(o.OID, "11111111-2222-3333-4444-555555555555")
	s.Equal(o.UID, "")
	s.Equal(o.APIKey, "31111111-2222-3333-4444-555555555555")

	s.NoError(o.FromConfigString([]byte(testConfig), "default"))
	s.Equal(o.OID, "11111111-2222-3333-4444-555555555555")
	s.Equal(o.UID, "")
	s.Equal(o.APIKey, "31111111-2222-3333-4444-555555555555")
}

func (s *ConfigsTestSuite) TestFromConfigStringFromEnvironment() {
	o := ClientOptions{}
	s.NoError(o.FromConfigString([]byte(testConfig), "vvv"))
	s.Equal(o.OID, "71111111-2222-3333-4444-555555555555")
	s.Equal(o.UID, "81111111-2222-3333-4444-555555555555")
	s.Equal(o.APIKey, "91111111-2222-3333-4444-555555555555")
}
