package limacharlie

import (
	"bytes"
	"net/http"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// goroutinesIn counts the goroutines currently running with fn on their stack.
func goroutinesIn(fn string) int {
	buf := bytes.Buffer{}
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 2); err != nil {
		return -1
	}
	n := 0
	for _, stack := range strings.Split(buf.String(), "\n\n") {
		if strings.Contains(stack, fn) {
			n++
		}
	}
	return n
}

// waitForNoGoroutinesIn waits (briefly) for every goroutine running fn to exit.
func waitForNoGoroutinesIn(fn string, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for {
		n := goroutinesIn(fn)
		if n == 0 || time.Now().After(deadline) {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// syncFetchHive fans out one goroutine per hive and returns as soon as any of
// them reports an error. Every other failing fetch must still be able to report
// without blocking: an API key that is refused on several hives at once (the
// common partially-permissioned key) would otherwise strand those goroutines
// for the lifetime of the process.
func TestSyncFetchHiveDoesNotLeakOnError(t *testing.T) {
	r := require.New(t)

	ms := NewMockServer("00000000-0000-0000-0000-0000000000ff")
	defer ms.Close()

	denied := map[string]bool{
		"dr-general": true,
		"fp":         true,
		"yara":       true,
		"lookup":     true,
	}
	ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
		// Path is /v1/hive/<hive name>/<partition key>.
		parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		hive := ""
		if len(parts) >= 3 {
			hive = parts[2]
		}
		w.Header().Set("Content-Type", "application/json")
		if denied[hive] {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"PERMISSION_DENIED"}`))
			return
		}
		w.Write([]byte(`{}`))
	}

	org, err := ms.NewOrganization()
	r.NoError(err)

	before := goroutinesIn("syncFetchHive")

	_, err = org.syncFetchHive(map[string]bool{
		"dr-general":       true,
		"fp":               true,
		"yara":             true,
		"lookup":           true,
		"extension_config": true,
		"cloud_sensor":     true,
	})
	r.Error(err, "a denied hive must be reported to the caller")
	r.Contains(err.Error(), "PERMISSION_DENIED")

	remaining := waitForNoGoroutinesIn("syncFetchHive", 5*time.Second)
	r.Equal(before, remaining, "every fetch goroutine must finish; %d were left blocked reporting their error", remaining-before)
}
