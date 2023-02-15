package limacharlie

import (
	"fmt"
	"net/http"
)

type Stats struct {
	Totals map[string]uint `json:"totals"`
}
type DetStats struct {
	Totals map[string]map[string]int `json:"totals"`
}
type EventContainer struct {
	Event Event `json:"event"`
}
type Event struct {
	Event     interface{} `json:"event"`
	Routing   Routing     `json:"routing"`
	TimeStamp string      `json:"ts"`
}
type Routing struct {
	Arch      int      `json:"arch"`
	DID       string   `json:"did"`
	EventID   string   `json:"event_id"`
	EventTime int64    `json:"event_time"`
	EventType string   `json:"event_type"`
	ExtIP     string   `json:"ext_ip"`
	Hostname  string   `json:"hostname"`
	IID       string   `json:"iid"`
	IntIP     string   `json:"int_ip"`
	ModuleID  int      `json:"moduleid"`
	OID       string   `json:"oid"`
	Parent    string   `json:"parent"`
	Plat      int      `json:"plat"`
	SID       string   `json:"sid"`
	Tags      []string `json:"tags"`
	This      string   `json:"this"`
}

type Detect struct {
	Author    string  `json:"author"`
	Cat       string  `json:"cat"`
	Detect    Dict    `json:"detect"`
	DetectID  string  `json:"detect_id"`
	DetectMtd Dict    `json:"detect_mtd"`
	Link      string  `json:"link"`
	Namespace string  `json:"namespace"`
	Routing   Routing `json:"routing"`
	Source    string  `json:"source"`
	Ts        int64   `json:"ts"`
}

type HistoricalDetectionsResponse struct {
	Detects    []Detect `json:"detects"`
	NextCursor string   `json:"next_cursor"`
}

func (org *Organization) OnlineStats(start int64, end int64) (Stats, error) {
	stats := Stats{}
	q := makeDefaultRequest(&stats)
	q = q.withQueryData(Dict{
		"start": start,
		"end":   end,
	})
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/online/stats", org.client.options.OID), q); err != nil {
		return stats, err
	}
	return stats, nil
}

func (org *Organization) TrafficStats(start int64, end int64) (Stats, error) {
	stats := Stats{}
	q := makeDefaultRequest(&stats)
	q = q.withQueryData(Dict{
		"start": start,
		"end":   end,
	})
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/traffic/stats", org.client.options.OID), q); err != nil {
		return stats, err
	}
	return stats, nil
}

func (org *Organization) DetectionStats(start int64, end int64) (DetStats, error) {
	stats := DetStats{}
	q := makeDefaultRequest(&stats)
	q = q.withQueryData(Dict{
		"start": start,
		"end":   end,
	})
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/detections/stats", org.client.options.OID), q); err != nil {
		return stats, err
	}
	return stats, nil
}

func (org *Organization) GenericGETRequest(path string, query Dict, response interface{}) error {
	q := makeDefaultRequest(response)
	q = q.withQueryData(query)
	return org.client.reliableRequest(http.MethodGet, path, q)
}

func (org *Organization) EventByAtom(sensorID, atom string) (EventContainer, error) {
	event := EventContainer{}
	q := makeDefaultRequest(&event)
	err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/%s/%s", org.client.options.OID, sensorID, atom), q)
	return event, err
}

type HistoricalDetectionsRequest struct {
	// Cat is the category of the detections to fetch
	Cat string `json:"cat"`
	// Cursor is optional for paginated access, set to '-' for first query
	Cursor string `json:"cursor"`
	// Start is the required timestamp in seconds where to stop fetching detections
	Start int `json:"start"`
	// End is the required timestamp in seconds where to stop fetching detections
	End int `json:"end"`
	// Limit maximum number of detections to return
	Limit int `json:"limit"`
}

func (org Organization) HistoricalDetections(detectionReq HistoricalDetectionsRequest) (HistoricalDetectionsResponse, error) {

	var results HistoricalDetectionsResponse

	if detectionReq.Cursor == "" {
		detectionReq.Cursor = "-"
	}

	q := makeDefaultRequest(&results)
	q = q.withQueryData(detectionReq)

	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/detections", org.client.options.OID), q); err != nil {
		return results, err
	}

	return results, nil
}
