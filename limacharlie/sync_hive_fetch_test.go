package limacharlie

import (
	"bytes"
	"errors"
	"fmt"
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

	_, _, err = org.syncFetchHive(map[string]bool{
		"dr-general":       true,
		"fp":               true,
		"yara":             true,
		"lookup":           true,
		"extension_config": true,
		"cloud_sensor":     true,
	}, false)
	r.Error(err, "a denied hive must be reported to the caller")
	r.Contains(err.Error(), "PERMISSION_DENIED")

	remaining := waitForNoGoroutinesIn("syncFetchHive", 5*time.Second)
	r.Equal(before, remaining, "every fetch goroutine must finish; %d were left blocked reporting their error", remaining-before)
}

// hostPostureDenied is the body the API gateway answers with when the key lacks
// the permission to read a hive (status 400 today).
const hostPostureDenied = `{"error":"lc_error_code:UNAUTHORIZED - access to 00000000-0000-0000-0000-0000000000ff requires host_posture.get"}`

// newHiveFetchOrg returns an org whose hive endpoint serves one record for every
// hive except those in denied, which answer with the given status and body.
type hiveReply struct {
	status int
	body   string
}

func newHiveFetchOrg(t *testing.T, denied map[string]hiveReply) *Organization {
	t.Helper()
	ms := NewMockServer("00000000-0000-0000-0000-0000000000ff")
	t.Cleanup(ms.Close)
	ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		hive := ""
		if len(parts) >= 3 {
			hive = parts[2]
		}
		w.Header().Set("Content-Type", "application/json")
		if reply, ok := denied[hive]; ok {
			w.WriteHeader(reply.status)
			w.Write([]byte(reply.body))
			return
		}
		w.Write([]byte(`{"records":{"rec":{"data":{"hive":"` + hive + `"},"usr_mtd":{"enabled":true}}},"next_cursor":""}`))
	}
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	return org
}

func TestSyncFetchWithReportSkipsUnauthorizedHives(t *testing.T) {
	r := require.New(t)
	org := newHiveFetchOrg(t, map[string]hiveReply{
		"host_posture_rule": {http.StatusBadRequest, hostPostureDenied},
		"cloudsec_policy":   {http.StatusBadRequest, `{"error":"lc_error_code:UNAUTHORIZED - access to x requires cloudsec_policy.get"}`},
	})

	cfg, report, err := org.SyncFetchWithReport(SyncOptions{
		SyncHives: map[string]bool{
			"yara": true, "host_posture_rule": true, "cloudsec_policy": true, "lookup": true, "fp": false,
		},
		SkipUnauthorizedHives: true,
	})
	r.NoError(err)

	r.Len(cfg.Hives, 2)
	r.Contains(cfg.Hives, "yara")
	r.Contains(cfg.Hives, "lookup")
	r.NotContains(cfg.Hives, "host_posture_rule", "a skipped hive has no key, not an empty map")
	r.NotContains(cfg.Hives, "cloudsec_policy")
	r.NotContains(cfg.Hives, "fp", "a hive that was not asked for is not fetched")
	r.Equal(map[string]interface{}{"hive": "yara"}, cfg.Hives["yara"]["rec"].Data)

	r.Len(report.SkippedHives, 2)
	r.Equal("cloudsec_policy", report.SkippedHives[0].Hive, "sorted by hive name")
	r.Equal("cloudsec_policy.get", report.SkippedHives[0].Permission)
	r.Equal("host_posture_rule", report.SkippedHives[1].Hive)
	r.Equal("host_posture.get", report.SkippedHives[1].Permission)
	r.Contains(report.SkippedHives[1].Error, "lc_error_code:UNAUTHORIZED")
}

func TestSyncFetchWithReportNothingSkippedIsEmptyList(t *testing.T) {
	r := require.New(t)
	org := newHiveFetchOrg(t, nil)
	cfg, report, err := org.SyncFetchWithReport(SyncOptions{
		SyncHives:             map[string]bool{"yara": true},
		SkipUnauthorizedHives: true,
	})
	r.NoError(err)
	r.Contains(cfg.Hives, "yara")
	r.NotNil(report.SkippedHives)
	r.Empty(report.SkippedHives)
}

// Without the option, an UNAUTHORIZED hive fails the fetch exactly as before.
func TestSyncFetchUnauthorizedHiveIsFatalByDefault(t *testing.T) {
	r := require.New(t)
	org := newHiveFetchOrg(t, map[string]hiveReply{"host_posture_rule": {http.StatusBadRequest, hostPostureDenied}})
	opts := SyncOptions{SyncHives: map[string]bool{"yara": true, "host_posture_rule": true}}

	_, err := org.SyncFetch(opts)
	r.Error(err)
	r.Contains(err.Error(), "sync_hives:")
	r.Contains(err.Error(), "requires host_posture.get")

	_, report, err := org.SyncFetchWithReport(opts)
	r.Error(err)
	r.Empty(report.SkippedHives)
}

// With the option, only the UNAUTHORIZED error is forgiven.
func TestSyncFetchSkipUnauthorizedStillFailsOnOtherErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
	}{
		"server error":                     {http.StatusInternalServerError, `{"error":"boom"}`},
		"forbidden with another code":      {http.StatusForbidden, `{"error":"PERMISSION_DENIED"}`},
		"bad request other code":           {http.StatusBadRequest, `{"error":"lc_error_code:INVALID_ARGUMENT - bad"}`},
		"unauthorized text not error code": {http.StatusBadRequest, `{"error":"the call was unauthorized"}`},
		"not json":                         {http.StatusBadRequest, `lc_error_code:UNAUTHORIZED - access requires x.get`},
	} {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			org := newHiveFetchOrg(t, map[string]hiveReply{"host_posture_rule": {tc.status, tc.body}})
			_, report, err := org.SyncFetchWithReport(SyncOptions{
				SyncHives:             map[string]bool{"yara": true, "host_posture_rule": true},
				SkipUnauthorizedHives: true,
			})
			r.Error(err)
			r.Empty(report.SkippedHives)
		})
	}
}

// A real failure next to an unauthorized hive is still reported, not hidden by
// the skip.
func TestSyncFetchSkipUnauthorizedMixedWithFatal(t *testing.T) {
	r := require.New(t)
	org := newHiveFetchOrg(t, map[string]hiveReply{
		"host_posture_rule": {http.StatusBadRequest, hostPostureDenied},
		"fp":                {http.StatusInternalServerError, `{"error":"boom"}`},
	})
	_, report, err := org.SyncFetchWithReport(SyncOptions{
		SyncHives:             map[string]bool{"yara": true, "host_posture_rule": true, "fp": true},
		SkipUnauthorizedHives: true,
	})
	r.Error(err)
	r.Empty(report.SkippedHives)

	// No goroutine is left behind either way.
	r.Zero(waitForNoGoroutinesIn("syncFetchHive", 5*time.Second))
}

func TestUnauthorizedAPIError(t *testing.T) {
	r := require.New(t)
	mk := func(status int, body string) error {
		e := NewRESTError("x: " + body)
		e.status = status
		e.body = body
		return fmt.Errorf("wrapped: %w", e)
	}
	perm, ok := unauthorizedAPIError(mk(400, hostPostureDenied))
	r.True(ok)
	r.Equal("host_posture.get", perm)

	perm, ok = unauthorizedAPIError(mk(403, `{"error":"lc_error_code:UNAUTHORIZED - nope"}`))
	r.True(ok, "status other than 400 is accepted when the error code is UNAUTHORIZED")
	r.Empty(perm)

	_, ok = unauthorizedAPIError(mk(500, hostPostureDenied))
	r.False(ok)
	_, ok = unauthorizedAPIError(errors.New("lc_error_code:UNAUTHORIZED - access requires a.b"))
	r.False(ok, "not a REST error")
	_, ok = unauthorizedAPIError(nil)
	r.False(ok)
}
