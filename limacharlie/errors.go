package limacharlie

import (
	"errors"
	"fmt"
)

type InvalidClientOptionsError struct {
	s string
}

func NewInvalidClientOptionsError(err string) InvalidClientOptionsError {
	return InvalidClientOptionsError{s: err}
}

func (e InvalidClientOptionsError) Error() string {
	return fmt.Sprintf("invalid client options: %s", e.s)
}

var NoAPIKeyConfiguredError = errors.New("no api key configured")

type RESTError struct {
	s string
}

func NewRESTError(err string) RESTError {
	return RESTError{s: err}
}

func (e RESTError) Error() string {
	return fmt.Sprintf("api error: %s", e.s)
}
