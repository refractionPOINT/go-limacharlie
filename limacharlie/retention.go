package limacharlie

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
type Event = Dict
type IteratedEvent struct {
	Error string `json:"error"`
	Data  Dict   `json:"data"`
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

func (org *Organization) HistoricalDetections(detectionReq HistoricalDetectionsRequest) (HistoricalDetectionsResponse, error) {

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

type HistoricEventsRequest struct {
	// Start is the start unix (seconds) timestamp to fetch events from
	Start int64 `json:"start"`
	// End is the end unix (seconds) timestamp to fetch events to
	End int64 `json:"end"`
	// Limit is the maximum number of events to return (optional)
	Limit *int `json:"limit,omitempty"`
	// EventType returns events only of this type (optional)
	EventType string `json:"event_type,omitempty"`
	// IsForward returns events in ascending order
	IsForward bool `json:"is_forward"`
	// OutputName sends data to a named output instead (optional)
	OutputName string `json:"output_name,omitempty"`
	// IsCompressed indicates if the response should be compressed
	IsCompressed bool `json:"is_compressed"`
	// Cursor is used for pagination
	Cursor string `json:"cursor,omitempty"`
}

type historicEventsResponse struct {
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor"`
}

type historicEventsResponseCompressed struct {
	Events     string `json:"events"`
	NextCursor string `json:"next_cursor"`
}

// GetHistoricEvents gets the events for a sensor between two times, requires Insight (retention) enabled.
// If outputName is specified, it will return a single response. Otherwise, it will handle pagination internally.
// The returned close function can be called to abort the operation.
func (org *Organization) GetHistoricEvents(sensorID string, req HistoricEventsRequest) (chan IteratedEvent, func(), error) {
	if req.Cursor == "" {
		req.Cursor = "-"
	}
	req.IsCompressed = true

	// Create a channel to stream events and a done channel for cancellation
	eventChan := make(chan IteratedEvent)
	done := make(chan struct{})

	// Convert request to Dict using JSON marshaling
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	var reqDict Dict
	if err := json.Unmarshal(reqBytes, &reqDict); err != nil {
		return nil, nil, err
	}

	// Create close function
	closeFunc := func() {
		close(done)
	}

	// If outputName is specified, make a single request and close the channel
	if req.OutputName != "" {
		response := Dict{}
		err := org.GenericGETRequest(fmt.Sprintf("insight/%s/%s", org.client.options.OID, sensorID), reqDict, &response)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}

	// Handle pagination
	go func() {
		defer close(eventChan)
		nReturned := 0
		for req.Cursor != "" {
			select {
			case <-done:
				return
			default:
			}

			events := []Event{}
			respCursor := ""

			if req.IsCompressed {
				response := historicEventsResponseCompressed{}
				// Update cursor in Dict
				reqDict["cursor"] = req.Cursor
				err = org.GenericGETRequest(fmt.Sprintf("insight/%s/%s", org.client.options.OID, sensorID), reqDict, &response)
				if err == nil {
					respCursor = response.NextCursor
					// We need to decompress the events
					r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(response.Events))
					if err != nil {
						eventChan <- IteratedEvent{Error: err.Error()}
						return
					}
					z, err := gzip.NewReader(r)
					if err != nil {
						eventChan <- IteratedEvent{Error: err.Error()}
						return
					}
					if err := json.NewDecoder(z).Decode(&events); err != nil {
						eventChan <- IteratedEvent{Error: err.Error()}
						return
					}
				}
			} else {
				response := historicEventsResponse{}
				// Update cursor in Dict
				reqDict["cursor"] = req.Cursor
				err = org.GenericGETRequest(fmt.Sprintf("insight/%s/%s", org.client.options.OID, sensorID), reqDict, &response)
				if err == nil {
					events = response.Events
					respCursor = response.NextCursor
				}
			}
			if err != nil {
				eventChan <- IteratedEvent{Error: err.Error()}
				return
			}

			for _, event := range events {
				select {
				case <-done:
					return
				case eventChan <- IteratedEvent{Data: event}:
					nReturned++
					if req.Limit != nil && nReturned >= *req.Limit {
						return
					}
				}
			}

			req.Cursor = respCursor
		}
	}()

	return eventChan, closeFunc, nil
}
