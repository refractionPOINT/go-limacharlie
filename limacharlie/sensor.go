package limacharlie

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
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

type rawSensorListPage struct {
	ContinuationToken string `json:"continuation_token"`
	Sensors           string `json:"sensors"`
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
	IdempotentKey        string
}

type SimpleRequestOptions struct {
	Timeout         time.Duration
	UntilCompletion bool
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
	resp := Dict{}
	if err := s.Organization.client.reliableRequest(http.MethodPost, fmt.Sprintf("%s/isolation", s.SID), makeDefaultRequest(&resp)); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) RejoinNetwork() error {
	resp := Dict{}
	if err := s.Organization.client.reliableRequest(http.MethodDelete, fmt.Sprintf("%s/isolation", s.SID), makeDefaultRequest(&resp)); err != nil {
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
	resp := Dict{}
	req := makeDefaultRequest(&resp).withFormData(Dict{
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
	resp := Dict{}
	req := makeDefaultRequest(&resp).withFormData(Dict{
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
	idempotentKey := ""
	if len(options) != 0 {
		opt := options[0]
		if opt.InvestigationID != "" {
			effectiveInvestigationID = opt.InvestigationID
		}
		if effectiveInvestigationID != "" && opt.InvestigationContext != "" {
			effectiveInvestigationID = fmt.Sprintf("%s/%s", effectiveInvestigationID, opt.InvestigationContext)
		}
		if opt.IdempotentKey != "" {
			idempotentKey = opt.IdempotentKey
		}
	}
	if effectiveInvestigationID != "" {
		data["investigation_id"] = effectiveInvestigationID
	}
	resp := Dict{}
	req := makeDefaultRequest(&resp).withFormData(data)
	if idempotentKey != "" {
		req = req.withIdempotentKey(idempotentKey)
	}
	if err := s.Organization.client.reliableRequest(http.MethodPost, s.SID, req); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) Delete() error {
	resp := Dict{}
	req := makeDefaultRequest(&resp)
	if err := s.Organization.client.reliableRequest(http.MethodDelete, s.SID, req); err != nil {
		s.LastError = err
		return err
	}
	return nil
}

func (s *Sensor) IsOnline() (bool, error) {
	resp, err := s.Organization.ActiveSensors([]string{s.SID})
	if err != nil {
		return false, err
	}
	return resp[s.SID], nil
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

type ListSensorsOptions struct {
	Selector string
	Limit    int
}

func (org *Organization) ListSensors(options ...ListSensorsOptions) (map[string]*Sensor, error) {
	m := map[string]*Sensor{}
	lastToken := ""
	effectiveOptions := ListSensorsOptions{}
	if len(options) != 0 {
		effectiveOptions = options[0]
	}
	for {
		page := rawSensorListPage{}
		q := makeDefaultRequest(&page)
		if lastToken != "" {
			q = q.withQueryData(Dict{
				"continuation_token": lastToken,
				"is_compressed":      "true",
				"selector":           effectiveOptions.Selector,
				"limit":              effectiveOptions.Limit,
			})
		} else {
			q = q.withQueryData(Dict{
				"is_compressed": "true",
				"selector":      effectiveOptions.Selector,
				"limit":         effectiveOptions.Limit,
			})
		}
		if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("sensors/%s", org.client.options.OID), q); err != nil {
			return nil, err
		}

		sensors := []*Sensor{}
		if err := decompressPayload(page.Sensors, &sensors); err != nil {
			return nil, err
		}

		for _, s := range sensors {
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

func (org *Organization) ListSensorsFromSelector(selector string) (map[string]*Sensor, error) {
	m := map[string]*Sensor{}
	lastToken := ""

	for {
		page := rawSensorListPage{}
		q := makeDefaultRequest(&page)
		if lastToken != "" {
			q = q.withQueryData(Dict{
				"continuation_token": lastToken,
				"selector":           selector,
				"is_compressed":      "true",
			})
		} else {
			q = q.withQueryData(Dict{
				"selector":      selector,
				"is_compressed": "true",
			})
		}
		if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("sensors/%s", org.client.options.OID), q); err != nil {
			return nil, err
		}

		sensors := []*Sensor{}
		if err := decompressPayload(page.Sensors, &sensors); err != nil {
			return nil, err
		}

		for _, s := range sensors {
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

func (org *Organization) ListSensorsFromSelectorIteratively(selector string, continuationToken string) (map[string]*Sensor, string, error) {
	m := map[string]*Sensor{}
	lastToken := continuationToken

	page := rawSensorListPage{}
	q := makeDefaultRequest(&page)
	if lastToken != "" {
		q = q.withQueryData(Dict{
			"continuation_token": lastToken,
			"selector":           selector,
			"is_compressed":      "true",
		})
	} else {
		q = q.withQueryData(Dict{
			"selector":      selector,
			"is_compressed": "true",
		})
	}
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("sensors/%s", org.client.options.OID), q); err != nil {
		return nil, "", err
	}

	sensors := []*Sensor{}
	if err := decompressPayload(page.Sensors, &sensors); err != nil {
		return nil, "", err
	}

	for _, s := range sensors {
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
	lastToken = page.ContinuationToken

	return m, lastToken, nil
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

func (org *Organization) ActiveSensors(sids []string) (map[string]bool, error) {
	list := map[string]bool{}
	q := makeDefaultRequest(&list).withFormData(Dict{
		"sids": sids,
	})
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("online/%s", org.client.options.OID), q); err != nil {
		return nil, err
	}
	return list, nil
}

func (org *Organization) GetSensorsWithTag(tag string) (map[string][]string, error) {
	data := map[string][]string{}
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("tags/%s/%s", org.client.options.OID, url.QueryEscape(tag)), makeDefaultRequest(&data)); err != nil {
		return nil, err
	}
	return data, nil
}

func decompressPayload(data string, out interface{}) error {
	// We decode the base64 string.
	b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(data)))
	// We decompress the data.
	z, err := gzip.NewReader(b64)
	if err != nil {
		return err
	}
	defer z.Close()
	// We decode the JSON.
	if err := json.NewDecoder(z).Decode(out); err != nil {
		return err
	}
	return nil
}

// SimpleRequest makes a request to the sensor assuming a single response.
// It uses the Organization's Spout to track responses and provides a simpler interface for making requests.
func (s *Sensor) SimpleRequest(tasks interface{}, options ...SimpleRequestOptions) (interface{}, error) {
	// Convert tasks to string slice if needed
	var taskList []string
	switch t := tasks.(type) {
	case string:
		taskList = []string{t}
	case []string:
		taskList = t
	default:
		return nil, fmt.Errorf("tasks must be string or []string")
	}

	// Set default options
	opts := SimpleRequestOptions{
		Timeout:         30 * time.Second,
		UntilCompletion: false,
	}
	if len(options) > 0 {
		opts = options[0]
	}

	// Create a unique tracking ID
	trackingID := fmt.Sprintf("%s/%s", s.InvestigationID, uuid.New().String())

	// Create a channel to receive responses
	responseChan := make(chan interface{}, len(taskList))
	errorChan := make(chan error, 1)

	// Check if organization has an active spout
	err := s.Organization.MakeInteractive()
	if err != nil {
		return nil, fmt.Errorf("failed to make organization interactive: %v", err)
	}

	// Start a goroutine to read responses
	go func() {
		deadline := time.Now().Add(opts.Timeout)
		responses := []interface{}{}
		completionCount := 0

		for {
			// Check if we've hit the deadline
			if time.Now().After(deadline) {
				errorChan <- fmt.Errorf("timeout waiting for responses")
				return
			}

			// Get next message with timeout
			msg, err := s.Organization.spout.Get()
			if err != nil {
				if err.Error() == "spout stopped" {
					break
				}
				errorChan <- err
				return
			}

			// Process the message
			if m, ok := msg.(map[string]interface{}); ok {
				routing, ok := m["routing"].(map[string]interface{})
				if !ok {
					continue
				}
				// Check if this message belongs to our tracking ID
				if invID, ok := routing["investigation_id"].(string); !ok || invID != trackingID {
					continue
				}

				// Check for completion receipt
				if errMsg, ok := m["ERROR_MESSAGE"].(string); ok && errMsg == "done" {
					completionCount++
					if completionCount >= len(taskList) {
						break
					}
					continue
				}

				// Add to responses
				responses = append(responses, msg)

				// If not waiting for completion and we have all responses, we're done
				if !opts.UntilCompletion && len(responses) >= len(taskList) {
					break
				}
			}
		}

		// Return the appropriate response
		if len(taskList) == 1 && len(responses) > 0 {
			responseChan <- responses[0]
		} else {
			responseChan <- responses
		}
	}()
	// Send the tasks
	if err := s.Task(taskList[0], TaskingOptions{InvestigationID: trackingID}); err != nil {
		return nil, fmt.Errorf("failed to send task: %v", err)
	}

	// Wait for response
	select {
	case resp := <-responseChan:
		return resp, nil
	case err := <-errorChan:
		return nil, err
	case <-time.After(opts.Timeout):
		return nil, fmt.Errorf("timeout waiting for responses")
	}
}
