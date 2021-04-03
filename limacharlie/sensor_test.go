package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSensorInfo(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// List all sensors.
	sensors, err := org.ListSensors()
	if err != nil {
		t.Errorf("ListSensors: %v", err)
	}
	if len(sensors) == 0 {
		t.Error("no sensors listed")
		return
	}
	sid := ""
	for _, s := range sensors {
		sid = s.SID
		if s.Hostname == "" {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.OID == "" {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.IID == "" {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.SID == "" {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.Platform == 0 {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.Architecture == 0 {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.Organization == nil {
			t.Errorf("missing sensor info: %+v", s)
		}
		if s.LastError != nil {
			t.Errorf("missing sensor info: %+v", s)
		}
	}

	// Get a single sensor.
	s := org.GetSensor(sid).Update()
	if s.Hostname == "" {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.OID == "" {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.IID == "" {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.SID == "" {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.Platform == 0 {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.Architecture == 0 {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.Organization == nil {
		t.Errorf("missing sensor info: %+v", s)
	}
	if s.LastError != nil {
		t.Errorf("missing sensor info: %+v", s)
	}
}
