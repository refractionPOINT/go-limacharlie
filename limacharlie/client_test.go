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
		OID:    "00000000-0000-4000-8000-000000000004",
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
			OID:         "00000000-0000-4000-8000-000000000001",
			UID:         "00000000-0000-4000-8000-000000000002",
			APIKey:      "00000000-0000-4000-8000-000000000003",
		})
	}
}

func (s *ClientTestSuite) TestFileLoaderNoEnvironment() {
	c, err := NewClientFromLoader(ClientOptions{}, nil, &FileClientOptionLoader{os.Getenv("LC_CREDS_FILE_NO_ENV")})
	if s.NoError(err) {
		s.Equal(c.options, ClientOptions{
			Environment: "",
			OID:         "00000000-0000-4000-8000-000000000010",
			UID:         "00000000-0000-4000-8000-000000000011",
			APIKey:      "00000000-0000-4000-8000-000000000012",
		})
	}
}
func (s *ClientTestSuite) TestFileLoader() {
	c, err := NewClientFromLoader(ClientOptions{}, nil, &FileClientOptionLoader{os.Getenv("LC_CREDS_FILE")})
	if s.NoError(err) {
		s.Equal(c.options, ClientOptions{
			Environment: "",
			OID:         "00000000-0000-4000-8000-000000000004",
			UID:         "00000000-0000-4000-8000-000000000005",
			APIKey:      "00000000-0000-4000-8000-000000000006",
		})
	}
}

func (s *ClientTestSuite) TestDefaultURLs() {
	c, err := NewClientFromLoader(ClientOptions{
		OID: "00000000-0000-4000-8000-000000000004",
		JWT: "fake",
	}, nil)
	if s.NoError(err) {
		s.Equal(rootURL, c.baseURL)
		s.Equal(getJWTURL, c.jwtURL)
	}
}

func (s *ClientTestSuite) TestURLOverrides() {
	c, err := NewClientFromLoader(ClientOptions{
		OID:    "00000000-0000-4000-8000-000000000004",
		JWT:    "fake",
		URL:    "https://api.example.test",
		JWTURL: "https://jwt.example.test",
	}, nil)
	if s.NoError(err) {
		s.Equal("https://api.example.test", c.baseURL)
		s.Equal("https://jwt.example.test", c.jwtURL)
	}
}
