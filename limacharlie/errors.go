package limacharlie

import (
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
