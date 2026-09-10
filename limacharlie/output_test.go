package limacharlie

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gopkg.in/yaml.v3"
)

func TestOutputList(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	deleteAllOutputs(org)

	outputs, err := org.Outputs()
	a.NoError(err)
	if len(outputs) != 0 {
		t.Errorf("unexpected preexisting outputs in list: %+v", outputs)
	}
}

func TestOutputAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	deleteAllOutputs(org)
	defer deleteAllOutputs(org)

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

// TestOutputElasticCreateActionYAML round-trips the elastic data stream toggle.
// Bool fields carry the ",string" JSON option, so a field declared without it
// marshals to an unquoted value the backend then rejects — and one missing from
// the struct is dropped silently, which is the failure this pins.
func TestOutputElasticCreateActionYAML(t *testing.T) {
	testOutput := OutputConfig{
		Name:   "test-lc-go-sdk-out",
		Module: OutputTypes.Elastic,
		Type:   OutputType.Event,

		Addresses:         "https://elastic.example.com:9200",
		Index:             "logs-limacharlie-default",
		IsCreateAction:    true,
		IsCompressRequest: true,
	}

	y, err := yaml.Marshal(testOutput)
	if err != nil {
		t.Fatalf("failed to marshal output to yaml: %v", err)
	}
	for _, want := range []string{`is_create_action: "true"`, `is_compress_request: "true"`} {
		if !strings.Contains(string(y), want) {
			t.Errorf("%s missing or unquoted in:\n%s", want, y)
		}
	}

	roundTripped := OutputConfig{}
	if err := yaml.Unmarshal(y, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal output from yaml: %v", err)
	}
	if fmt.Sprintf("%#v", roundTripped) != fmt.Sprintf("%#v", testOutput) {
		t.Errorf("mismatch: %#v != %#v", roundTripped, testOutput)
	}

	// Omitted, the toggle must not appear at all: an output that never set it
	// keeps the "index" action.
	testOutput.IsCreateAction = false
	testOutput.IsCompressRequest = false
	y, err = yaml.Marshal(testOutput)
	if err != nil {
		t.Fatalf("failed to marshal output to yaml: %v", err)
	}
	for _, unwanted := range []string{"is_create_action", "is_compress_request"} {
		if strings.Contains(string(y), unwanted) {
			t.Errorf("%s emitted while unset:\n%s", unwanted, y)
		}
	}
}
