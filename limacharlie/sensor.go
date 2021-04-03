package limacharlie

import (
	"fmt"
	"net/http"
)

type Sensor struct {
	OID          string `json:"oid"`
	IID          string `json:"iid"`
	SID          string `json:"sid"`
	DID          string `json:"did,omitempty"`
	Platform     uint32 `json:"plat"`
	Architecture uint32 `json:"arch"`

	EnrollTS string `json:"enroll"`
	AliveTS  string `json:"alive"`

	InternalIP string `json:"int_ip"`
	ExternalIP string `json:"ext_ip"`

	Hostname string `json:"hostname"`

	IsIsolated        bool `json:"isolated"`
	ShouldIsolate     bool `json:"should_isolate"`
	IsKernelAvailable bool `json:"kernel"`

	Organization *Organization `json:"-"`

	LastError error `json:"-"`
}

type sensorListPage struct {
	ContinuationToken string    `json:"continuation_token"`
	Sensors           []*Sensor `json:"sensors"`
}

func (s *Sensor) Update() *Sensor {
	if err := s.Organization.client.reliableRequest(http.MethodGet, s.SID, makeDefaultRequest(s)); err != nil {
		s.LastError = err
		return s
	}
	return s
}

func (org *Organization) GetSensor(SID string) *Sensor {
	s := &Sensor{
		OID:          org.client.options.OID,
		SID:          SID,
		Organization: org,
	}
	return s.Update()
}

func (org *Organization) GetSensors(SIDs []string) map[string]*Sensor {
	m := map[string]*Sensor{}
	for _, s := range SIDs {
		m[s] = org.GetSensor(s).Update()
	}
	return m
}

func (org *Organization) ListSensors() (map[string]*Sensor, error) {
	m := map[string]*Sensor{}
	lastToken := ""

	for {
		page := sensorListPage{}
		q := makeDefaultRequest(&page)
		if lastToken != "" {
			q = q.withQueryData(Dict{
				"continuation_token": lastToken,
			})
		}
		if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("sensors/%s", org.client.options.OID), q); err != nil {
			return nil, err
		}
		for _, s := range page.Sensors {
			s.Organization = org
			m[s.SID] = s
		}
		if page.ContinuationToken == "" {
			break
		}
		lastToken = page.ContinuationToken
	}

	return m, nil
}
