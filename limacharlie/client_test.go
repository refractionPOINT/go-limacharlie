package limacharlie

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

func (s *ClientTestSuite) TestNoLoader() {
	c, err := NewClientFromLoader(ClientOptions{})
	s.EqualError(err, newLCError(lcErrClientNoOptionsLoader).Error())
	s.Nil(c)
}

func (s *ClientTestSuite) TestEnvironmentLoader() {
	c, err := NewClientFromLoader(ClientOptions{}, &EnvironmentClientOptionLoader{})
	if s.NoError(err) {
		s.Equal(c.options, ClientOptions{
			Environment: "test_env",
			OID:         "fba6e992-ce4f-4d9e-99dc-b548f00df7f9",
			UID:         "af4ddec0-c2e8-4db2-ba3f-f5e9a1aff3fd",
			APIKey:      "843e80c8-e273-4b3e-93bd-41151b4b933a",
		})
	}
}

func (s *ClientTestSuite) TestFileLoaderNoEnvironment() {
	c, err := NewClientFromLoader(ClientOptions{}, &FileClientOptionLoader{os.Getenv("LC_CREDS_FILE_NO_ENV")})
	if s.NoError(err) {
		s.Equal(c.options, ClientOptions{
			Environment: "",
			OID:         "c67941b8-8f1b-444c-9dd3-e2790f880a01",
			UID:         "09da8f03-92cf-425c-9df3-da4dd206c25a",
			APIKey:      "45ea660f-99ca-4663-a27e-764d4fbde119",
		})
	}
}
func (s *ClientTestSuite) TestFileLoader() {
	c, err := NewClientFromLoader(ClientOptions{}, &FileClientOptionLoader{os.Getenv("LC_CREDS_FILE")})
	if s.NoError(err) {
		s.Equal(c.options, ClientOptions{
			Environment: "",
			OID:         "9416bc29-2bae-47d7-ac8c-63210f3a22e3",
			UID:         "708034c9-38d9-4603-8b9d-16e2bbc5cf97",
			APIKey:      "8daf363c-88a2-4a8e-b375-99aeb236fbd0",
		})
	}
}
