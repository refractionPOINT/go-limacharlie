package limacharlie

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
)

const (
	rootURL           = "https://api.limacharlie.io"
	currentAPIVersion = "v1"
	getJWTURL         = "https://app.limacharlie.io/jwt"

	defaultConfigFileLocation = "~/.limacharlie"
	environmentNameEnvVar     = "LC_CURRENT_ENV"
	oidEnvVar                 = "LC_OID"
	uidEnvVar                 = "LC_UID"
	keyEnvVar                 = "LC_API_KEY"
	credsEnvVar               = "LC_CREDS_FILE"
)

type Client struct {
	options ClientOptions
}

type ClientOptions struct {
	OID         string
	APIKey      string
	UID         string
	JWT         string
	Environment string
}

type jwtResponse struct {
	JWT string `json:"jwt"`
}

func NewClient(opts ...ClientOptions) (*Client, error) {
	c := &Client{}

	if len(opts) > 1 {
		return nil, NewInvalidClientOptionsError("too many options specified")
	} else if len(opts) == 1 {
		c.options = opts[0]
	}

	// If any value is missing from the config file
	// look for it in the environment.
	if c.options.OID == "" {
		c.options.OID = os.Getenv(oidEnvVar)
	}
	if c.options.UID == "" {
		c.options.UID = os.Getenv(uidEnvVar)
	}
	if c.options.APIKey == "" {
		c.options.APIKey = os.Getenv(keyEnvVar)
	}
	if c.options.Environment == "" {
		c.options.Environment = os.Getenv(environmentNameEnvVar)
	}

	// If neither OrgID or UserID is specified
	// we need to parse the config to auto-detect.
	if c.options.OID == "" && c.options.UID == "" {
		configFile := defaultConfigFileLocation
		if globalEnv := os.Getenv(credsEnvVar); globalEnv != "" {
			configFile = globalEnv
		}
		if err := c.options.FromConfigFile(configFile, c.options.Environment); err != nil {
			return nil, err
		}
	}

	// Validate the minimum requirements.
	if c.options.OID == "" && c.options.UID == "" {
		return nil, NewInvalidClientOptionsError("OID or UID is required")
	}

	// Validate all the options we ended up with.
	if err := validateUUID(c.options.OID); err != nil {
		return nil, NewInvalidClientOptionsError(fmt.Sprintf("invalid OID: %v", err))
	}
	if err := validateUUID(c.options.UID); err != nil {
		return nil, NewInvalidClientOptionsError(fmt.Sprintf("invalid UID: %v", err))
	}
	if err := validateUUID(c.options.APIKey); err != nil {
		return nil, NewInvalidClientOptionsError(fmt.Sprintf("invalid APIKey: %v", err))
	}

	return c, nil
}

func validateUUID(s string) error {
	if s == "" {
		return nil
	}
	if _, err := uuid.Parse(s); err != nil {
		return NewInvalidClientOptionsError(err.Error())
	}
	return nil
}

func (c *Client) refreshJWT(expiry time.Duration) error {
	if c.options.APIKey == "" {
		return NoAPIKeyConfiguredError
	}
	authData := url.Values{}
	authData.Set("secret", c.options.APIKey)
	if c.options.UID != "" {
		authData.Set("uid", c.options.UID)
	}
	if c.options.OID != "" {
		authData.Set("oid", c.options.OID)
	}
	if expiry != 0 {
		authData.Set("expiry", fmt.Sprintf("%d", int64(expiry.Seconds())))
	}

	resp, err := http.PostForm(getJWTURL, authData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return NewRESTError(resp.Status)
	}

	respData := bytes.Buffer{}
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return err
	}

	// We should have a valid JWT.
	jwtData := jwtResponse{}
	if err := json.Unmarshal(respData.Bytes(), &jwtData); err != nil {
		return err
	}

	c.options.JWT = jwtData.JWT

	return nil
}
