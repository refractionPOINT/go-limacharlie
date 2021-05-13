package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutputList(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	outputs, err := org.Outputs()
	a.NoError(err)
	if len(outputs) != 0 {
		t.Errorf("unexpected preexisting outputs in list: %+v", outputs)
	}
}

func TestOutputAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	outputs, err := org.Outputs()
	a.NoError(err)
	if len(outputs) != 0 {
		t.Errorf("unexpected preexisting outputs in add/delete: %+v", outputs)
	}

	testOutputName := "test-lc-go-sdk-out"

	testOutput := OutputConfig{
		Name:   testOutputName,
		Module: OutputTypes.Syslog,
		Type:   OutputType.Event,

		DestinationHost: "1.1.1.1:22",
		TLS:             true,
		StrictTLS:       true,
		NoHeader:        true,
	}

	_, err = org.OutputAdd(testOutput)
	a.NoError(err)

	var output OutputConfig
	var ok bool

	outputs, err = org.Outputs()
	a.NoError(err)
	if len(outputs) == 0 {
		t.Errorf("outputs is empty")
	} else if output, ok = outputs[testOutputName]; !ok {
		t.Errorf("test output not found: %+v", outputs)
	} else if output.Type != OutputType.Event {
		t.Errorf("output type is wrong: %s", output.Type)
	}

	var rawJSON GenericJSON
	err = org.OutputsGeneric(&rawJSON)
	a.NoError(err)
	if len(rawJSON) == 0 {
		t.Errorf("generic outputs is empty")
	}

	_, err = org.OutputDel(testOutputName)
	a.NoError(err)
}
