package limacharlie

import (
	"testing"
	"time"

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

func TestSensorIsolation(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	sensors, err := org.ListSensors()
	if err != nil {
		t.Errorf("ListSensors: %v", err)
	}
	if len(sensors) == 0 {
		t.Error("no sensors listed")
		return
	}
	var sensor *Sensor
	for _, s := range sensors {
		sensor = s
		break
	}

	if err := sensor.IsolateFromNetwork(); err != nil {
		t.Errorf("failed isolating: %v", err)
	}
	if err := sensor.RejoinNetwork(); err != nil {
		t.Errorf("failed rejoining: %v", err)
	}
}

func TestSensorTags(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	sensors, err := org.ListSensors()
	if err != nil {
		t.Errorf("ListSensors: %v", err)
	}
	if len(sensors) == 0 {
		t.Error("no sensors listed")
		return
	}
	var sensor *Sensor
	for _, s := range sensors {
		sensor = s
		break
	}

	tags, err := sensor.GetTags()
	if err != nil {
		t.Errorf("GetTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("test expects no default tags: %v", tags)
		return
	}

	if err := sensor.AddTag("ttt", 30*time.Second); err != nil {
		t.Errorf("AddTag: %v", err)
	}

	time.Sleep(2 * time.Second)

	tags, err = sensor.GetTags()
	if err != nil {
		t.Errorf("GetTags: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("unexpected tags: %v", tags)
		return
	}
	if tags[0].Tag != "ttt" || tags[0].By == "" || tags[0].AddedTS == "" {
		t.Errorf("unexpected tags: %v", tags)
	}

	if err := sensor.RemoveTag("ttt"); err != nil {
		t.Errorf("RemoveTag: %v", err)
	}

	time.Sleep(2 * time.Second)

	tags, err = sensor.GetTags()
	if err != nil {
		t.Errorf("GetTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("unexpected tags: %v", tags)
		return
	}
}

func TestSensorTask(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	org = org.WithInvestigationID("testinv")

	sensors, err := org.ListSensors()
	if err != nil {
		t.Errorf("ListSensors: %v", err)
	}
	if len(sensors) == 0 {
		t.Error("no sensors listed")
		return
	}
	var sensor *Sensor
	for _, s := range sensors {
		sensor = s
		break
	}

	if sensor.InvestigationID != "testinv" {
		t.Errorf("InvID not propagated: %s", sensor.InvestigationID)
	}

	if err := sensor.Task("os_version"); err != nil {
		t.Errorf("Task: %v", err)
	}
}
