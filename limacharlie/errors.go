package limacharlie

import (
	"errors"
	"fmt"
)

type lcErrorCode = string

const (
	lcErrClientNoOptionsLoader     = "CLIENT_NO_OPTION_LOADER"
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
