package limacharlie

import (
	"bytes"
	"context"
	"encoding/base64"
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
	getJWTURL         = "https://jwt.limacharlie.io"

	restRetries          = 3
	restTimeout          = 30 * time.Second
	restCreateOrgTimeout = 35 * time.Second
)

// Client makes raw request to LC cloud
type Client struct {
	options    ClientOptions
	logger     LCLogger
	httpClient *http.Client
}

// ClientOptions holds all options for Client
type ClientOptions struct {
	OID           string
	APIKey        string
	UID           string
	JWT           string
	Environment   string
	Permissions   []string
	JWTExpiryTime time.Duration
}

type jwtResponse struct {
	JWT string `json:"jwt"`
}

type restRequest struct {
	nRetries      int
	timeout       time.Duration
	queryData     interface{}
	formData      interface{}
	urlValues     url.Values
	response      interface{}
	urlRoot       string
	idempotentKey string
}

func makeDefaultRequest(response interface{}) restRequest {
	return restRequest{
		nRetries: restRetries,
		timeout:  restTimeout,
		response: response,
		urlRoot:  fmt.Sprintf("/%s/", currentAPIVersion),
	}
}

func (r restRequest) withTimeout(timeout time.Duration) restRequest {
	r.timeout = timeout
	return r
}

func (r restRequest) withFormData(formData interface{}) restRequest {
	r.formData = formData
	return r
}

func (r restRequest) withQueryData(queryData interface{}) restRequest {
	r.queryData = queryData
	return r
}

func (r restRequest) withURLValues(urlValues url.Values) restRequest {
	r.urlValues = urlValues
	return r
}

func (r restRequest) withURLRoot(root string) restRequest {
	r.urlRoot = root
	return r
}

func (r restRequest) withIdempotentKey(idempotentKey string) restRequest {
	r.idempotentKey = idempotentKey
	return r
}

func isEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// NewClientFromLoader initialize a client from options loaders.
// Will return a valid client as soon as one loader returns valid requirements
func NewClientFromLoader(inOpt ClientOptions, logger LCLogger, optsLoaders ...ClientOptionLoader) (*Client, error) {
	if inOpt.validateMinimumRequirements() == nil && inOpt.validate() == nil {
		return &Client{options: inOpt, logger: logger, httpClient: getHTTPClient()}, nil
	}

	loaderCount := len(optsLoaders)
	if loaderCount == 0 {
		return nil, newLCError(lcErrClientNoOptionsLoader)
	}

	var opt ClientOptions
	var err error
	for _, loader := range optsLoaders {
		if opt, err = loader.Load(inOpt); err != nil {
			return nil, err
		}
		if err = opt.validateMinimumRequirements(); err == nil {
			break
		}
	}

	if err = opt.validateMinimumRequirements(); err != nil {
		return nil, err
	}
	if err = opt.validate(); err != nil {
		return nil, err
	}

	return &Client{
		options:    opt,
		logger:     logger,
		httpClient: getHTTPClient(),
	}, nil
}

// NewClient loads client options from
// first, environment varibles;
// then from a file specified by the environment variable LC_CREDS_FILE;
// then from .limacharlie in home directory
func NewClient(opt ClientOptions, logger LCLogger) (*Client, error) {
	if logger == nil {
		logger = &LCLoggerEmpty{}
	}
	return NewClientFromLoader(opt,
		logger,
		&EnvironmentClientOptionLoader{},
		NewFileClientOptionLoader(os.Getenv("LC_CREDS_FILE")),
		NewFileClientOptionLoader("~/.limacharlie"),
	)
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

func (c *Client) RefreshJWT(expiry time.Duration) (string, error) {
	if c.options.APIKey == "" {
		return "", ErrorNoAPIKeyConfigured
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
	if c.options.Permissions != nil {
		authData.Set("perms", strings.Join(c.options.Permissions, ","))
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, getJWTURL, strings.NewReader(authData.Encode()))
	if err != nil {
		return "", err
	}

	r.Header.Set("User-Agent", "limacharlie-sdk")
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", NewRESTError(resp.Status)
	}

	respData := bytes.Buffer{}
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return "", err
	}

	// We should have a valid JWT.
	jwtData := jwtResponse{}
	if err := json.Unmarshal(respData.Bytes(), &jwtData); err != nil {
		return "", err
	}

	c.options.JWT = jwtData.JWT
	return c.options.JWT, nil
}

func getHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func (c *Client) reliableRequest(verb string, path string, request restRequest) (err error) {
	// If no JWT is ready and we have an API key, prime it (similar to Python SDK behavior).
	// This prevents sending empty JWT on first request which causes billing server to complain.
	if c.options.JWT == "" && c.options.APIKey != "" {
		if _, err = c.RefreshJWT(c.options.JWTExpiryTime); err != nil {
			return err
		}
	}

	request.nRetries++
	for request.nRetries > 0 {
		var statusCode int
		statusCode, err = c.request(verb, path, request)
		if err == nil && statusCode == http.StatusOK {
			break
		}
		request.nRetries--

		if statusCode == http.StatusUnauthorized {
			// Unauthorized, the JWT may have expired, refresh
			// it and retry.
			// If there is no API Key configured, provide the
			// previous error instead of the refresh.
			if c.options.APIKey == "" {
				return err
			}
			if _, err = c.RefreshJWT(c.options.JWTExpiryTime); err != nil {
				// If we cannot get a new JWT there is no point in
				// retrying with bad creds.
				return err
			}
		} else if statusCode == http.StatusTooManyRequests {
			// Out of quota, wait a bit and retry.
			time.Sleep(10 * time.Second)
		} else if statusCode == http.StatusGatewayTimeout {
			// Looks like the API might be under load.
			time.Sleep(5 * time.Second)
		} else if err == nil {
			// If no errors, any other status code other than those
			// above will not be retried.
			break
		}
	}
	return err
}

func (c *Client) serviceRequest(responseData interface{}, serviceName string, serviceData Dict, isAsync bool) error {
	bytes, err := json.Marshal(serviceData)
	if err != nil {
		return err
	}
	encodedData := base64.StdEncoding.EncodeToString(bytes)

	req := makeDefaultRequest(responseData).withFormData(Dict{
		"request_data": encodedData,
		"is_async":     isAsync,
	}).withTimeout(10 * time.Minute)
	return c.reliableRequest(http.MethodPost, fmt.Sprintf("service/%s/%s", c.options.OID, serviceName), req)
}

func (c *Client) extensionRequest(responseData interface{}, extensionName string, action string, serviceData Dict, isImpersonate bool) error {
	bytes, err := json.Marshal(serviceData)
	if err != nil {
		return err
	}
	reqData := Dict{
		"oid":    c.options.OID,
		"data":   string(bytes),
		"action": action,
	}
	if isImpersonate {
		reqData["impersonator_jwt"] = c.options.JWT
	}

	req := makeDefaultRequest(responseData).withFormData(reqData).withTimeout(10 * time.Minute)
	return c.reliableRequest(http.MethodPost, fmt.Sprintf("extension/request/%s", extensionName), req)
}

func getStringKV(d interface{}) (*url.Values, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	o := map[string]interface{}{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&o); err != nil {
		return nil, err
	}
	m := &url.Values{}
	for k, v := range o {
		if _, ok := v.(Dict); ok {
			// If the value is a dict, assume
			// we want to ship its JSON string value.
			s, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			m.Set(k, string(s))
		} else if _, ok := v.(map[string]interface{}); ok {
			// If the value is a dict, assume
			// we want to ship its JSON string value.
			s, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			m.Set(k, string(s))
		} else if l, ok := v.([]string); ok {
			for _, e := range l {
				m.Add(k, e)
			}
		} else if l, ok := v.([]interface{}); ok {
			for _, e := range l {
				m.Add(k, fmt.Sprintf("%v", e))
			}
		} else {
			// Just the normal value itself.
			m.Set(k, fmt.Sprintf("%v", v))
		}
	}
	if len(o) == 0 {
		return nil, nil
	}
	return m, nil
}

func (c *Client) request(verb string, path string, request restRequest) (int, error) {
	headers := map[string]string{}
	var body io.Reader
	rawQuery := ""

	var fData *url.Values
	var err error
	if request.urlValues != nil {
		fData = &request.urlValues
	} else {
		fData, err = getStringKV(request.formData)
		if err != nil {
			return 0, err
		}
	}

	if fData != nil {
		body = strings.NewReader(fData.Encode())
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}

	qData, err := getStringKV(request.queryData)
	if err != nil {
		return 0, err
	}
	if qData != nil {
		rawQuery = qData.Encode()
	}

	ctx, _ := context.WithTimeout(context.Background(), request.timeout)

	// Build the URL - if urlRoot is a full URL (starts with http), use it as base, otherwise concatenate
	var fullURL string
	if strings.HasPrefix(request.urlRoot, "http://") || strings.HasPrefix(request.urlRoot, "https://") {
		fullURL = fmt.Sprintf("%s%s", request.urlRoot, path)
	} else {
		fullURL = fmt.Sprintf("%s%s%s", rootURL, request.urlRoot, path)
	}

	r, err := http.NewRequestWithContext(ctx, verb, fullURL, body)
	if err != nil {
		return 0, err
	}

	r.Header.Set("User-Agent", "limacharlie-sdk")
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.options.JWT))
	if request.idempotentKey != "" {
		r.Header.Set("x-idempotent-key", request.idempotentKey)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}

	if rawQuery != "" {
		r.URL.RawQuery = rawQuery
	}

	resp, err := c.httpClient.Do(r)
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

	// Prior to enforcement of rate limits, we return the headers
	// in the response. If the request is over quota and the headers
	// are present, display a warning in stderr.
	rateLimitQuota := resp.Header.Get("X-RateLimit-Quota")
	rateLimitPeriod := resp.Header.Get("X-RateLimit-Period")
	if rateLimitQuota != "" && rateLimitPeriod != "" {
		os.Stderr.WriteString(fmt.Sprintf("Warning: Rate limit hit, quota limit: %s, quota period: %s seconds, see https://docs.limacharlie.io/v2/docs/en/api-keys?highlight=bulk\n", rateLimitQuota, rateLimitPeriod))
	}

	respData := bytes.Buffer{}
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return resp.StatusCode, err
	}

	// If no response target is provided, we're done
	if request.response == nil {
		return resp.StatusCode, nil
	}

	// If response body is empty, don't try to unmarshal
	if respData.Len() == 0 {
		return resp.StatusCode, nil
	}

	// If the response is not a well structured
	// datatype (and is a map[]interface{} instead)
	// we will perform a CleanUnmarshal to better
	// interpret large integers to int64 whenever
	// possible instead of the json's default float64.
	if originalResponse, ok := request.response.(*map[string]interface{}); ok {
		tmpResp, err := UnmarshalCleanJSON(respData.String())
		if err != nil {
			return resp.StatusCode, fmt.Errorf("error parsing response: %v", err)
		}
		for k, v := range tmpResp {
			(*originalResponse)[k] = v
		}
		return resp.StatusCode, nil
	}

	// Looks like it is not a map[string]interface{}, let json do its thing.
	if err := json.Unmarshal(respData.Bytes(), request.response); err != nil {
		return resp.StatusCode, fmt.Errorf("error parsing response: %v", err)
	}
	return resp.StatusCode, nil
}

type WhoAmIJsonResponse struct {
	UserPermissions *map[string][]string `json:"user_perms:omitempty"`
	Organizations   *[]string            `json:"orgs"`
	Permissions     *[]string            `json:"perms"`
	Identity        *string              `json:"ident"`
}

// GenericJSON is the default format for json data
type GenericJSON = map[string]interface{}

func (c *Client) WhoAmI() (WhoAmIJsonResponse, error) {
	who := WhoAmIJsonResponse{}
	if err := c.reliableRequest(http.MethodGet, "who", makeDefaultRequest(&who)); err != nil {
		return WhoAmIJsonResponse{}, err
	}
	return who, nil
}

// GetCurrentJWT returns the JWT from the client options
func (c *Client) GetCurrentJWT() string {
	return c.options.JWT
}

func (w WhoAmIJsonResponse) HasPermissionForOrg(oid string, permName string) bool {
	if w.UserPermissions != nil {
		if p, ok := (*w.UserPermissions)[oid]; ok {
			for _, v := range p {
				if permName == v {
					return true
				}
			}
		}
	}
	if w.Organizations == nil || w.Permissions == nil {
		return false
	}
	isOrgFound := false
	for _, o := range *w.Organizations {
		if o == oid {
			isOrgFound = true
			break
		}
	}
	if !isOrgFound {
		return false
	}
	for _, p := range *w.Permissions {
		if p == permName {
			return true
		}
	}
	return false
}

func (w WhoAmIJsonResponse) HasAccessToOrg(oid string) bool {
	if w.UserPermissions != nil {
		if _, ok := (*w.UserPermissions)[oid]; ok {
			return true
		}
	}
	if w.Organizations == nil || w.Permissions == nil {
		return false
	}
	isOrgFound := false
	for _, o := range *w.Organizations {
		if o == oid {
			isOrgFound = true
			break
		}
	}
	return isOrgFound
}
