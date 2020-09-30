package limacharlie

import (
	"testing"
)

func TestOutputList(t *testing.T) {
	c := getTestClient(t)
	outputs, err := c.Outputs()
	if err != nil {
		t.Errorf("failed to get outputs: %v", err)
	}
	if len(outputs) != 0 {
		t.Errorf("unexpected preexisting outputs: %+v", outputs)
	}
}

func TestOutputAddDelete(t *testing.T) {
	c := getTestClient(t)
	outputs, err := c.Outputs()
	if err != nil {
		t.Errorf("failed to get outputs: %v", err)
	}
	if len(outputs) != 0 {
		t.Errorf("unexpected preexisting outputs: %+v", outputs)
	}

	testOutputName := "test-lc-go-sdk-out"

	testOutput := GenericOutputConfig{
		Name:   testOutputName,
		Module: OutputTypes.Syslog,
		Stream: OutputStreams.Event,

		DestinationHost: "1.1.1.1:22",
		TLS:             true,
		StrictTLS:       true,
		NoHeader:        true,
	}

	_, err = c.OutputAdd(testOutput)
	if err != nil {
		t.Errorf("error adding output: %v", err)
	}

	outputs, err = c.Outputs()
	if err != nil {
		t.Errorf("failed to get outputs: %v", err)
	}
	if len(outputs) != 1 {
		t.Errorf("unexpected preexisting outputs: %+v", outputs)
	} else {
		output, ok := outputs[testOutputName]
		if !ok || len(output.(map[string]interface{})) == 0 {
			t.Errorf("test output not found: %+v", outputs)
		}
	}

	if _, err = c.OutputDel(testOutputName); err != nil {
		t.Errorf("error deleting output: %v", err)
	}
}
