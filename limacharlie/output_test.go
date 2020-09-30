package limacharlie

import "testing"

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
