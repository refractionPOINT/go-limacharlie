package limacharlie

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	OID           string
	APIKey        string
	UID           string
	JWT           string
	Environment   string
	JWTExpiryTime time.Duration
}

type jwtResponse struct {
	JWT string `json:"jwt"`
}

type restRequest struct {
	nRetries  int
	timeout   time.Duration
	queryData interface{}
	formData  interface{}
	response  interface{}
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

func getHttpClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func (c *Client) reliableRequest(verb string, path string, request restRequest) error {
	request.nRetries++
	var err error
	for request.nRetries > 0 {
		var statusCode int
		statusCode, err = c.request(verb, path, request)
		if err == nil && statusCode == 200 {
			break
		}
		request.nRetries--

		if statusCode == 401 {
			// Unauthorized, the JWT may have expired, refresh
			// it and retry.
			if err := c.refreshJWT(c.options.JWTExpiryTime); err != nil {
				// If we cannot get a new JWT there is no point in
				// retrying with bad creds.
				return err
			}
		}
	}
	return err
}

func getStringKV(d interface{}) (map[string]string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	o := map[string]interface{}{}
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, err
	}
	m := map[string]string{}
	for k, v := range o {
		m[k] = fmt.Sprintf("%v", v)
	}
	return m, nil
}

func (c *Client) request(verb string, path string, request restRequest) (int, error) {
	headers := map[string]string{}
	var body io.Reader
	rawQuery := ""

	fData, err := getStringKV(request.formData)
	if err != nil {
		return 0, err
	}

	if len(fData) != 0 {
		vals := url.Values{}
		for k, v := range fData {
			vals.Set(k, v)
		}
		body = strings.NewReader(vals.Encode())
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}

	qData, err := getStringKV(request.queryData)
	if err != nil {
		return 0, err
	}
	if len(qData) != 0 {
		vals := url.Values{}
		for k, v := range qData {
			vals.Set(k, v)
		}
		rawQuery = vals.Encode()
	}

	r, err := http.NewRequest(verb, fmt.Sprintf("%s/%s/%s", rootURL, currentAPIVersion, path), body)
	if err != nil {
		return 0, err
	}

	r.Header.Set("User-Agent", "limacharlie-sdk")
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.options.JWT))
	for k, v := range headers {
		r.Header.Set(k, v)
	}

	if rawQuery != "" {
		r.URL.RawQuery = rawQuery
	}

	resp, err := getHttpClient(request.timeout).Do(r)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// The API gateway returns error details in the body.
		errorStr := ""
		if errorDetails, err := ioutil.ReadAll(resp.Body); err == nil {
			errorStr = string(errorDetails)
		}
		return resp.StatusCode, NewRESTError(fmt.Sprintf("%s: %s", resp.Status, errorStr))
	}

	respData := bytes.Buffer{}
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return resp.StatusCode, err
	}
	if err := json.Unmarshal(respData.Bytes(), request.response); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

type whoAmIJsonResponse struct {
	UserPermissions *map[string][]string `json:"user_perms"`
	Organizations   *map[string][]string `json:"orgs"`
	Permissions     *[]string            `json:"perms"`
}

func (c *Client) whoAmI() (whoAmIJsonResponse, error) {
	who := whoAmIJsonResponse{}
	if err := c.reliableRequest(http.MethodGet, "who", restRequest{
		nRetries: 3,
		timeout:  5 * time.Second,
		response: &who,
	}); err != nil {
		return whoAmIJsonResponse{}, err
	}
	return who, nil
}
