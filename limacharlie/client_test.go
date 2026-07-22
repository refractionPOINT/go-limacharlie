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
	// With no loaders and empty options, the client should surface the actual
	// validation failure (missing minimum requirements) rather than a generic
	// "no loader" error.
	c, err := NewClientFromLoader(ClientOptions{}, nil)
	s.EqualError(err, newLCError(lcErrClientMissingRequirements).Error())
	s.Nil(c)
}

func (s *ClientTestSuite) TestNoLoaderInvalidAPIKey() {
	// Minimum requirements are met (a valid OID is set) but the APIKey is not a
	// valid UUID. With no loaders to fall back on, the underlying validation
	// error must be surfaced instead of being masked.
	c, err := NewClientFromLoader(ClientOptions{
		OID:    "9416bc29-2bae-47d7-ac8c-63210f3a22e3",
		APIKey: "not-a-valid-uuid",
	}, nil)
	s.Error(err)
	s.Contains(err.Error(), "invalid APIKey")
	s.Nil(c)
}

func (s *ClientTestSuite) TestEnvironmentLoader() {
	c, err := NewClientFromLoader(ClientOptions{}, nil, &EnvironmentClientOptionLoader{})
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
	c, err := NewClientFromLoader(ClientOptions{}, nil, &FileClientOptionLoader{os.Getenv("LC_CREDS_FILE_NO_ENV")})
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
	c, err := NewClientFromLoader(ClientOptions{}, nil, &FileClientOptionLoader{os.Getenv("LC_CREDS_FILE")})
	if s.NoError(err) {
		s.Equal(c.options, ClientOptions{
			Environment: "",
			OID:         "9416bc29-2bae-47d7-ac8c-63210f3a22e3",
			UID:         "708034c9-38d9-4603-8b9d-16e2bbc5cf97",
			APIKey:      "8daf363c-88a2-4a8e-b375-99aeb236fbd0",
		})
	}
}

func (s *ClientTestSuite) TestDefaultURLs() {
	c, err := NewClientFromLoader(ClientOptions{
		OID: "9416bc29-2bae-47d7-ac8c-63210f3a22e3",
		JWT: "fake",
	}, nil)
	if s.NoError(err) {
		s.Equal(rootURL, c.baseURL)
		s.Equal(getJWTURL, c.jwtURL)
	}
}

func (s *ClientTestSuite) TestURLOverrides() {
	c, err := NewClientFromLoader(ClientOptions{
		OID:    "9416bc29-2bae-47d7-ac8c-63210f3a22e3",
		JWT:    "fake",
		URL:    "https://api.example.test",
		JWTURL: "https://jwt.example.test",
	}, nil)
	if s.NoError(err) {
		s.Equal("https://api.example.test", c.baseURL)
		s.Equal("https://jwt.example.test", c.jwtURL)
	}
}
