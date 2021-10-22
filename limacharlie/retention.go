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
