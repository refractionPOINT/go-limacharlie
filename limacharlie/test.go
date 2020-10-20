package limacharlie

import (
	"fmt"
	"testing"
)

func assert(t *testing.T, value bool, message string) {
	if value {
		t.Errorf(message)
		t.FailNow()
	}
}

func assertIsNotError(t *testing.T, err error, message string) {
	assert(t, err != nil, fmt.Sprintf("%s: %v", message, err))
}
