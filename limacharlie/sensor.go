package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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

	Device *Device `json:"-"`

	LastError error `json:"-"`

	InvestigationID string `json:"-"`
}

type sensorInfo struct {
	Info     *Sensor `json:"info"`
	IsOnline bool    `json:"is_online"`
}

type sensorListPage struct {
	ContinuationToken string    `json:"continuation_token"`
	Sensors           []*Sensor `json:"sensors"`
}

type sensorTagsList struct {
	Tags map[string]map[string]TagInfo `json:"tags"`
}

type TagInfo struct {
	Tag     string
	By      string
	AddedTS string
}

type TaskingOptions struct {
	InvestigationID      string
	InvestigationContext string
}

func (t *TagInfo) UnmarshalJSON(b []byte) error {
	l := []interface{}{}
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	if len(l) < 4 {
		return fmt.Errorf("tag info missing elements: %v", l)
	}
	ok := false
	t.Tag, ok = l[1].(string)
	if !ok {
		return fmt.Errorf("tag wrong datatype: %v (%T)", l[1], l[1])
	}
	t.By, ok = l[2].(string)
	if !ok {
		return fmt.Errorf("by wrong datatype: %v (%T)", l[1], l[1])
	}
	t.AddedTS, ok = l[3].(string)
	if !ok {
		return fmt.Errorf("added wrong datatype: %v (%T)", l[1], l[1])
	}
	return nil
}

func (s *Sensor) Update() *Sensor {
	si := sensorInfo{
		Info: s,
	}
	if err := s.Organization.client.reliableRequest(http.MethodGet, s.SID, makeDefaultRequest(&si)); err != nil {
		s.LastError = err
		return s
	}
	if s.DID != "" {
		s.Device = &Device{
			DID:          s.DID,
			Organization: s.Organization,
		}
	}
	return s
}

func (s *Sensor) IsolateFromNetwork() error {
	if err := s.Organization.client.reliableRequest(http.MethodPost, fmt.Sprintf("%s/isolation", s.SID), makeDefaultRequest(s)); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) RejoinNetwork() error {
	if err := s.Organization.client.reliableRequest(http.MethodDelete, fmt.Sprintf("%s/isolation", s.SID), makeDefaultRequest(s)); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) GetTags() ([]TagInfo, error) {
	ti := sensorTagsList{}
	if err := s.Organization.client.reliableRequest(http.MethodGet, fmt.Sprintf("%s/tags", s.SID), makeDefaultRequest(&ti)); err != nil {
		s.LastError = err
		return nil, err
	}
	sTags, ok := ti.Tags[s.SID]
	if !ok {
		return nil, fmt.Errorf("missing sid tags: %+v", ti)
	}
	tags := []TagInfo{}
	for _, t := range sTags {
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Sensor) AddTag(tag string, ttl time.Duration) error {
	req := makeDefaultRequest(s).withFormData(Dict{
		"tags": tag,
		"ttl":  ttl / time.Second,
	})
	if err := s.Organization.client.reliableRequest(http.MethodPost, fmt.Sprintf("%s/tags", s.SID), req); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) RemoveTag(tag string) error {
	req := makeDefaultRequest(s).withFormData(Dict{
		"tags": tag,
	})
	if err := s.Organization.client.reliableRequest(http.MethodDelete, fmt.Sprintf("%s/tags", s.SID), req); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) Task(task string, options ...TaskingOptions) error {
	data := Dict{
		"tasks": task,
	}
	effectiveInvestigationID := s.InvestigationID
	if len(options) != 0 {
		opt := options[0]
		if opt.InvestigationID != "" {
			effectiveInvestigationID = opt.InvestigationID
		}
		if effectiveInvestigationID != "" && opt.InvestigationContext != "" {
			effectiveInvestigationID = fmt.Sprintf("%s/%s", effectiveInvestigationID, opt.InvestigationContext)
		}
	}
	if effectiveInvestigationID != "" {
		data["investigation_id"] = effectiveInvestigationID
	}
	req := makeDefaultRequest(s).withFormData(data)
	if err := s.Organization.client.reliableRequest(http.MethodPost, s.SID, req); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (org *Organization) GetSensor(SID string) *Sensor {
	s := &Sensor{
		OID:             org.client.options.OID,
		SID:             SID,
		Organization:    org,
		InvestigationID: org.invID,
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
			s.InvestigationID = org.invID
			if s.DID != "" {
				s.Device = &Device{
					DID:          s.DID,
					Organization: org,
				}
			}
			m[s.SID] = s
		}
		if page.ContinuationToken == "" {
			break
		}
		lastToken = page.ContinuationToken
	}

	return m, nil
}

func (org *Organization) GetAllTags() ([]string, error) {
	tags := struct {
		Tags []string `json:"tags"`
	}{}
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("tags/%s", org.client.options.OID), makeDefaultRequest(&tags)); err != nil {
		return nil, err
	}
	return tags.Tags, nil
}
