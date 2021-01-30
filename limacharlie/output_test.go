package limacharlie

import (
	"testing"
)

func TestOutputList(t *testing.T) {
	org := getTestOrgFromEnv(t)
	outputs, err := org.Outputs()
	assertIsNotError(t, err, "failed to get outputs")
	if len(outputs) != 0 {
		t.Errorf("unexpected preexisting outputs in list: %+v", outputs)
	}
}

func TestOutputAddDelete(t *testing.T) {
	org := getTestOrgFromEnv(t)
	outputs, err := org.Outputs()
	assertIsNotError(t, err, "failed to get outputs")
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
	assertIsNotError(t, err, "error adding output")

	outputs, err = org.Outputs()
	assertIsNotError(t, err, "failed to get outputs")
	if len(outputs) == 0 {
		t.Errorf("outputs is empty")
	} else if _, ok := outputs[testOutputName]; !ok {
		t.Errorf("test output not found: %+v", outputs)
	}

	var rawJSON GenericJSON
	err = org.OutputsGeneric(&rawJSON)
	assertIsNotError(t, err, "failed to get generic ouputs")
	if len(rawJSON) == 0 {
		t.Errorf("generic outputs is empty")
	}

	_, err = org.OutputDel(testOutputName)
	assertIsNotError(t, err, "error deleting output")
}
