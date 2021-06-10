package limacharlie

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"gopkg.in/yaml.v2"
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

func TestOutputMarshalingYAML(t *testing.T) {
	testOutput := OutputConfig{
		Name:   "test-lc-go-sdk-out",
		Module: OutputTypes.Syslog,
		Type:   OutputType.Event,

		DestinationHost: "1.1.1.1:22",
		TLS:             true,
		StrictTLS:       true,
		NoHeader:        true,
	}
	expected := `dest_host: 1.1.1.1:22
is_no_header: "true"
is_strict_tls: "true"
is_tls: "true"
module: syslog
name: test-lc-go-sdk-out
type: event
`

	y, err := yaml.Marshal(testOutput)
	if err != nil {
		t.Errorf("failed to marshal output to yaml: %v", err)
	}
	if string(y) != expected {
		t.Errorf("mismatch: %s != %s", y, expected)
	}
}

func TestOutputUnMarshalingYAML(t *testing.T) {
	testOutput := `dest_host: 1.1.1.1:22
is_no_header: "true"
is_strict_tls: "true"
is_tls: "true"
module: syslog
name: test-lc-go-sdk-out
type: event
`
	expected := OutputConfig{
		Name:   "test-lc-go-sdk-out",
		Module: OutputTypes.Syslog,
		Type:   OutputType.Event,

		DestinationHost: "1.1.1.1:22",
		TLS:             true,
		StrictTLS:       true,
		NoHeader:        true,
	}

	y := OutputConfig{}
	err := yaml.Unmarshal([]byte(testOutput), &y)
	if err != nil {
		t.Errorf("failed to marshal output to yaml: %v", err)
	}
	if fmt.Sprintf("%#v", y) != fmt.Sprintf("%#v", expected) {
		t.Errorf("mismatch: %#v != %#v", y, expected)
	}
}
