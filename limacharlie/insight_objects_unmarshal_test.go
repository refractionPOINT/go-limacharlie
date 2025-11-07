package limacharlie

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIOCLocationsResponseUnmarshal(t *testing.T) {
	a := assert.New(t)

	// Mock JSON response with known fields and dynamic sensor ID keys
	jsonData := `{
		"from_cache": true,
		"type": "domain",
		"name": "google.com",
		"sensor-id-1": {
			"sid": "sensor-id-1",
			"hostname": "host1",
			"first_ts": 1234567890,
			"last_ts": 1234567900
		},
		"sensor-id-2": {
			"sid": "sensor-id-2",
			"hostname": "host2",
			"first_ts": 1234567800,
			"last_ts": 1234567850
		}
	}`

	var resp IOCLocationsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	a.NoError(err)

	// Test known fields
	a.True(resp.FromCache, "FromCache should be true")
	a.Equal("domain", string(resp.Type), "Type should be 'domain'")
	a.Equal("google.com", resp.Name, "Name should be 'google.com'")

	// Test locations
	a.NotNil(resp.Locations, "Locations should not be nil")
	a.Equal(2, len(resp.Locations), "Should have 2 locations")

	loc1, ok := resp.Locations["sensor-id-1"]
	a.True(ok, "Should have sensor-id-1")
	a.Equal("sensor-id-1", loc1.SID)
	a.Equal("host1", loc1.Hostname)
	a.Equal(int64(1234567890), loc1.FirstTS)
	a.Equal(int64(1234567900), loc1.LastTS)

	loc2, ok := resp.Locations["sensor-id-2"]
	a.True(ok, "Should have sensor-id-2")
	a.Equal("sensor-id-2", loc2.SID)
	a.Equal("host2", loc2.Hostname)
}
