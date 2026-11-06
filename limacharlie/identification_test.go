package limacharlie

import (
	"encoding/json"
	"testing"
)

func TestDetonationPlatformCatalogue(t *testing.T) {
	var sensor struct {
		Platform uint32 `json:"plat"`
	}
	if err := json.Unmarshal([]byte(`{"plat":922746880}`), &sensor); err != nil {
		t.Fatal(err)
	}
	if sensor.Platform != Platforms.Detonation || PlatformStrings[sensor.Platform] != "detonation" || StringToPlatform["detonation"] != sensor.Platform {
		t.Fatal("detonation sensor platform does not round-trip through public catalogues")
	}
	for _, name := range []string{"windows", "linux", "macos", "lc_event"} {
		if StringToPlatform[name] == sensor.Platform {
			t.Fatalf("detonation aliases %s", name)
		}
	}
	for id, name := range PlatformStrings {
		if id == sensor.Platform && name != "detonation" {
			t.Fatalf("platform collision with %s", name)
		}
		if StringToPlatform[name] != id {
			t.Fatalf("existing platform %s lost its reverse mapping", name)
		}
	}
}
