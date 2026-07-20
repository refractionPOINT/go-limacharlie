package limacharlie

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lastSensorsListCall returns the most recent recorded GET sensors/{oid} call
// on the mock server, failing the test if none was recorded.
func lastSensorsListCall(t *testing.T, ms *MockServer) MockCall {
	t.Helper()
	calls := ms.Calls()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Method == "GET" && strings.Contains(calls[i].Path, "sensors/") {
			return calls[i]
		}
	}
	t.Fatalf("no GET sensors/ call recorded; calls=%+v", calls)
	return MockCall{}
}

// TestListSensorsFromSelectorIterativelyOnlineOnly pins the wire contract of the
// OnlineOnly option: it maps to the sensors-API is_online_only query parameter
// only when explicitly enabled, and composes with Limit. Not providing the
// option (or setting it false) leaves is_online_only OFF THE WIRE entirely, so
// the request is byte-identical to before the option existed - fully backward
// compatible.
func TestListSensorsFromSelectorIterativelyOnlineOnly(t *testing.T) {
	t.Run("OnlineOnly true sends is_online_only=true", func(t *testing.T) {
		ms, org := setupMock(t)
		_, _, err := org.ListSensorsFromSelectorIteratively(
			context.Background(), "plat == windows", "",
			ListSensorsIterativeOptions{OnlineOnly: true},
		)
		require.NoError(t, err)
		q := lastSensorsListCall(t, ms).Query
		require.Contains(t, q, "is_online_only=true")
		require.Contains(t, q, "selector=")
	})

	t.Run("no options omits is_online_only (backward compatible)", func(t *testing.T) {
		ms, org := setupMock(t)
		_, _, err := org.ListSensorsFromSelectorIteratively(
			context.Background(), "plat == windows", "",
		)
		require.NoError(t, err)
		q := lastSensorsListCall(t, ms).Query
		require.NotContains(t, q, "is_online_only")
		// The rest of the request is unchanged from before the option existed.
		require.Contains(t, q, "selector=")
		require.Contains(t, q, "is_compressed=true")
	})

	t.Run("explicit OnlineOnly false also omits is_online_only", func(t *testing.T) {
		ms, org := setupMock(t)
		_, _, err := org.ListSensorsFromSelectorIteratively(
			context.Background(), "plat == windows", "",
			ListSensorsIterativeOptions{OnlineOnly: false},
		)
		require.NoError(t, err)
		require.NotContains(t, lastSensorsListCall(t, ms).Query, "is_online_only")
	})

	t.Run("false OnlineOnly with a Limit still omits is_online_only", func(t *testing.T) {
		ms, org := setupMock(t)
		_, _, err := org.ListSensorsFromSelectorIteratively(
			context.Background(), "plat == linux", "",
			ListSensorsIterativeOptions{Limit: 25},
		)
		require.NoError(t, err)
		q := lastSensorsListCall(t, ms).Query
		require.NotContains(t, q, "is_online_only")
		require.Contains(t, q, "limit=25")
	})

	t.Run("Limit and OnlineOnly compose on the same request", func(t *testing.T) {
		ms, org := setupMock(t)
		_, _, err := org.ListSensorsFromSelectorIteratively(
			context.Background(), "plat == linux", "",
			ListSensorsIterativeOptions{Limit: 25, OnlineOnly: true},
		)
		require.NoError(t, err)
		q := lastSensorsListCall(t, ms).Query
		require.Contains(t, q, "is_online_only=true")
		require.Contains(t, q, "limit=25")
	})
}

// TestListSensorsFromSelectorIterativelyOnlineOnlyIntegration exercises the
// OnlineOnly option against the live sensors API. It runs in CI (cloudbuild.yaml
// injects _OID/_KEY); locally it fails fast when those are unset, like the other
// getTestOrgFromEnv tests. Mirrors TestListSensorsOnlineOnly for the iterative
// selector path.
//
// The invariant asserted is that OnlineOnly may only NARROW the result: the
// online-only set for a given selector is a subset of (and no larger than) the
// all-sensors set for the same selector. This holds regardless of how many
// sensors the selector matches in the test org, so the test is robust to org
// composition; the exact query-parameter wiring is pinned by the mock-based
// unit test above.
func TestListSensorsFromSelectorIterativelyOnlineOnlyIntegration(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	const selector = "plat == linux"
	resolve := func(onlineOnly bool) map[string]*Sensor {
		out := map[string]*Sensor{}
		token := ""
		for page := 0; page < 200; page++ {
			m, next, err := org.ListSensorsFromSelectorIteratively(
				context.Background(), selector, token,
				ListSensorsIterativeOptions{Limit: 100, OnlineOnly: onlineOnly},
			)
			a.NoError(err)
			for sid, s := range m {
				out[sid] = s
			}
			if next == "" {
				break
			}
			token = next
		}
		return out
	}

	all := resolve(false)
	online := resolve(true)

	// OnlineOnly narrows: the online set is never larger than the full set.
	if len(online) > len(all) {
		t.Errorf("online-only resolved %d sensors, must be <= all %d for the same selector", len(online), len(all))
	}
	// Every online sensor must also appear in the full set.
	for sid := range online {
		if _, ok := all[sid]; !ok {
			t.Errorf("online-only sensor %s absent from the all-sensors resolution", sid)
		}
	}
}
