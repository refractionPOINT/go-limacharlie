package limacharlie

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type lcErrorCode = string

const (
	lcErrClientMissingRequirements = "CLIENT_MISSING_REQUIREMENTS"
)

type lcError struct {
	code lcErrorCode
}

func newLCError(code lcErrorCode) *lcError {
	return &lcError{
		code: code,
	}
}

func (e *lcError) Error() string {
	return fmt.Sprintf("limacharlie client: %s", e.code)
}

// InvalidClientOptionsError is the error type returned by Client
type InvalidClientOptionsError struct {
	s string
}

// NewInvalidClientOptionsError makes a new error
func NewInvalidClientOptionsError(err string) InvalidClientOptionsError {
	return InvalidClientOptionsError{s: err}
}

func (e InvalidClientOptionsError) Error() string {
	return fmt.Sprintf("invalid client options: %s", e.s)
}

// ErrorNoAPIKeyConfigured is returned when no api key is given to a client
var ErrorNoAPIKeyConfigured = errors.New("no api key configured")

// RESTError is a generic rest error
type RESTError struct {
	s string
	// status is the HTTP status code of the response, or 0 when the error was
	// not built from an HTTP response.
	status int
	// body is the raw response body of the failed HTTP response, when the
	// error was built from one.
	body string
}

// StatusCode returns the HTTP status code that produced the error, or 0 when
// it is unknown.
func (e RESTError) StatusCode() int {
	return e.status
}

// unauthorizedErrorCode is the error code the API gateway puts at the start of
// the "error" field when the credentials in use lack a permission.
const unauthorizedErrorCode = "lc_error_code:UNAUTHORIZED"

// unauthorizedPermissionRe extracts the missing permission from an
// "access to <oid> requires <permission>" message.
var unauthorizedPermissionRe = regexp.MustCompile(`\brequires\s+([A-Za-z0-9_.\-]+)`)

// unauthorizedAPIError reports whether err is the API refusing a call because
// the credentials lack a permission, as opposed to any other failure. It
// relies on the structured error code of the API response ({"error":
// "lc_error_code:UNAUTHORIZED - access to <oid> requires <perm>"}), never on
// loose text matching, and returns the missing permission when the message
// names one.
func unauthorizedAPIError(err error) (permission string, ok bool) {
	var restErr RESTError
	if !errors.As(err, &restErr) {
		return "", false
	}
	if restErr.status < 400 || restErr.status >= 500 {
		return "", false
	}
	var body struct {
		Error string `json:"error"`
	}
	if jerr := json.Unmarshal([]byte(restErr.body), &body); jerr != nil {
		return "", false
	}
	msg := strings.TrimSpace(body.Error)
	if !strings.HasPrefix(msg, unauthorizedErrorCode) {
		return "", false
	}
	if m := unauthorizedPermissionRe.FindStringSubmatch(msg); m != nil {
		permission = m[1]
	}
	return permission, true
}

// NewRESTError makes a new RESTError
func NewRESTError(err string) RESTError {
	return RESTError{s: err}
}

func (e RESTError) Error() string {
	return fmt.Sprintf("api error: %s", e.s)
}

// ErrorResourceNotFound is returned when querying for a resource that does not exist or that the client does not have the permission to see
var ErrorResourceNotFound = errors.New("resource not found")

// Returned for a feature that is not yet implemented to parity with the Python SDK.
var ErrorNotImplemented = errors.New("not implemented")

func IsServiceNotRegisteredError(err error) bool {
	return strings.Contains(err.Error(), "org not registered to service")
}
