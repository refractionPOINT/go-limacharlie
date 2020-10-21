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

func assertNotNil(t *testing.T, ptr *interface{}) {
	assert(t, ptr != nil, "pointer is nil")
}

func assertNotEmptyString(t *testing.T, str string) {
	assert(t, len(str) == 0, "")
}
