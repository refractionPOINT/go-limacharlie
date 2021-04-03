package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSensorInfo(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	s, err := org.ListSensors()
	if err != nil {
		t.Errorf("ListSensors: %v", err)
	}
	if len(s) == 0 {
		t.Error("no sensors listed")
	}
}
