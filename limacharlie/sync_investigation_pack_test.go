package limacharlie

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// investigationSyncHives lists the investigation hives in the order sync applies them.
var investigationSyncHives = []string{"investigation_policy", "investigation_pack", "investigation_plan"}

const testPackYAML = "name: fictional_pack\nversion: 1\nbrief: A fictional shared check\ndescription: Reads a fictional fact.\nfacts:\n  f:\n    op: fictional.op\nexports:\n  e:\n    fact: f\n    path: x\n"

const testPlanUsingPackYAML = `{"gather":{"g":{"use":"fictional_pack"}}}`

// hiveWriteLog records the data writes and deletions that reach the mock Hive,
// in arrival order, as "<verb> <hive>/<key>".
type hiveWriteLog struct {
	mu     sync.Mutex
	events []string
}

func (l *hiveWriteLog) add(e string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *hiveWriteLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

// watchHiveWrites wraps the mock's Hive handler. The real Hive does not check
// that a pack exists when a template is written (the engine resolves it when it
// loads the template), so this stand-in simulates a stricter hive: it refuses a
// write to a plan that uses "fictional_pack" unless that pack is stored. The
// order of the writes is then checked by outcome as well as by the log.
func watchHiveWrites(ms *MockServer) *hiveWriteLog {
	log := &hiveWriteLog{}
	ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
		parts := strings.SplitN(strings.TrimPrefix(req.URL.Path, "/v1/hive/"), "/", 4)
		if len(parts) >= 3 && (req.Method == http.MethodPost || req.Method == http.MethodDelete) {
			hive, key := parts[0], parts[2]
			if req.Method == http.MethodDelete {
				log.add("delete " + hive + "/" + key)
			} else {
				body, _ := io.ReadAll(req.Body)
				req.Body = io.NopCloser(strings.NewReader(string(body)))
				form, _ := url.ParseQuery(string(body))
				if hive == "investigation_plan" && strings.Contains(writtenData(form), "fictional_pack") {
					ms.mu.Lock()
					_, packStored := ms.HiveStore["investigation_pack/"+testOID]["fictional_pack"]
					ms.mu.Unlock()
					if !packStored {
						ms.writeError(w, http.StatusBadRequest, "pack fictional_pack does not exist")
						return
					}
				}
				log.add("write " + hive + "/" + key)
			}
		}
		ms.handleHive(w, req)
	}
	return log
}

// writtenData is the record data of a write: plain on an update, gzipped on a create.
func writtenData(form url.Values) string {
	if gz := form.Get("gzdata"); gz != "" {
		raw, err := base64.StdEncoding.DecodeString(gz)
		if err != nil {
			return ""
		}
		r, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return ""
		}
		defer r.Close()
		out, _ := io.ReadAll(r)
		return string(out)
	}
	return form.Get("data")
}

func investigationPackConfig() OrgConfig {
	return OrgConfig{Version: OrgConfigLatestVersion, Hives: orgSyncHives{
		"investigation_policy": {"default": {Data: Dict{"paused": false}, UsrMtd: UsrMtd{Enabled: true, Tags: []string{"allow-api-key-promotion"}}}},
		"investigation_pack":   {"fictional_pack": {Data: Dict{"yaml": testPackYAML}, UsrMtd: UsrMtd{Enabled: true}}},
		"investigation_plan":   {"fictional-triage": {Data: Dict{"gather": Dict{"g": Dict{"use": "fictional_pack"}}}, UsrMtd: UsrMtd{Enabled: true}}},
	}}
}

func TestInvestigationPackIsASyncHiveAfterPolicyBeforePlan(t *testing.T) {
	idx := map[string]int{}
	for i, h := range KnownHives {
		idx[h] = i
	}
	require.Contains(t, idx, "investigation_pack")
	require.True(t, SyncAll().SyncHives["investigation_pack"])
	require.Less(t, idx["investigation_policy"], idx["investigation_pack"])
	require.Less(t, idx["investigation_pack"], idx["investigation_plan"])
	require.True(t, isInvestigationSyncHive("investigation_pack"))
}

func TestInvestigationPackRoundTrip(t *testing.T) {
	_, org := setupMock(t)
	cfg := investigationPackConfig()
	opts := SyncOptions{SyncHives: map[string]bool{"investigation_policy": true, "investigation_pack": true, "investigation_plan": true}}
	_, err := org.SyncPush(cfg, opts)
	require.NoError(t, err)
	// Second push: nothing differs, nothing is written.
	ops, err := org.SyncPush(cfg, opts)
	require.NoError(t, err)
	for _, op := range ops {
		require.False(t, op.IsAdded || op.IsRemoved, "unexpected change to %s", op.ElementName)
	}
	got, err := org.SyncFetch(opts)
	require.NoError(t, err)
	a, err := yaml.Marshal(cfg.Hives)
	require.NoError(t, err)
	b, err := yaml.Marshal(got.Hives)
	require.NoError(t, err)
	require.Equal(t, string(a), string(b))
	require.NotContains(t, string(b), "etag")

	// An edited pack is written against the version listed at the start of the apply.
	rec := cfg.Hives["investigation_pack"]["fictional_pack"]
	rec.Data = Dict{"yaml": strings.Replace(testPackYAML, "version: 1", "version: 2", 1)}
	cfg.Hives["investigation_pack"]["fictional_pack"] = rec
	_, err = org.SyncPush(cfg, opts)
	require.NoError(t, err)
	got, err = org.SyncFetch(opts)
	require.NoError(t, err)
	require.Contains(t, got.Hives["investigation_pack"]["fictional_pack"].Data["yaml"], "version: 2")
}

func TestInvestigationPackNativeCASConflict(t *testing.T) {
	ms, org := setupMock(t)
	key := "investigation_pack/" + testOID
	ms.HiveStore[key] = map[string]HiveData{"fictional_pack": {Data: Dict{"yaml": "concurrent"}, SysMtd: SysMtd{Etag: "newer"}}}
	ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && !strings.HasSuffix(req.URL.Path, "/data") {
			// The listing the apply diffs against is older than the store.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"fictional_pack":{"data":{"yaml":"old"},"usr_mtd":{},"sys_mtd":{"etag":"listed"}}}`))
			return
		}
		ms.handleHive(w, req)
	}
	cfg := OrgConfig{Version: 3, Hives: orgSyncHives{"investigation_pack": {"fictional_pack": {Data: Dict{"yaml": "desired"}}}}}
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"investigation_pack": true}})
	require.ErrorContains(t, err, "ETAG_MISMATCH")
	require.Equal(t, "concurrent", ms.HiveStore[key]["fictional_pack"].Data["yaml"])
}

// A template that uses a pack lands in the same sync as the pack, whatever the
// order of the map the configuration was loaded into.
func TestInvestigationPackAppliedBeforeTemplateUsingIt(t *testing.T) {
	for i := 0; i < 20; i++ {
		ms, org := setupMock(t)
		log := watchHiveWrites(ms)
		_, err := org.SyncPush(investigationPackConfig(), SyncAll())
		require.NoError(t, err)
		require.Equal(t, []string{
			"write investigation_policy/default",
			"write investigation_pack/fictional_pack",
			"write investigation_plan/fictional-triage",
		}, log.snapshot())
	}
}

// Removal runs in the reverse of the apply order, after every hive was applied:
// the template that uses a pack goes before the pack, the pack before the settings.
func TestInvestigationPackRemovedAfterTemplateUsingIt(t *testing.T) {
	ms, org := setupMock(t)
	seed := investigationPackConfig()
	opts := SyncOptions{SyncHives: map[string]bool{"investigation_policy": true, "investigation_pack": true, "investigation_plan": true}}
	_, err := org.SyncPush(seed, opts)
	require.NoError(t, err)
	log := watchHiveWrites(ms)
	opts.IsForce = true
	empty := OrgConfig{Version: 3, Hives: orgSyncHives{"investigation_policy": {}, "investigation_pack": {}, "investigation_plan": {}, "fp": {"new": {Data: Dict{"value": "x"}}}}}
	opts.SyncHives["fp"] = true
	_, err = org.SyncPush(empty, opts)
	require.NoError(t, err)
	require.Equal(t, []string{
		"write fp/new",
		"delete investigation_plan/fictional-triage",
		"delete investigation_pack/fictional_pack",
		"delete investigation_policy/default",
	}, log.snapshot())
}

// A refused write stops the apply before anything is removed.
func TestInvestigationRemovalsWaitForEveryHiveToApply(t *testing.T) {
	ms, org := setupMock(t)
	ms.HiveStore["investigation_pack/"+testOID] = map[string]HiveData{"stale": {Data: Dict{"yaml": "x"}}}
	log := watchHiveWrites(ms)
	cfg := OrgConfig{Version: 3, Hives: orgSyncHives{
		"investigation_pack": {},
		// Refused by the watcher: its pack is not stored.
		"investigation_plan": {"fictional-triage": {Data: Dict{"gather": Dict{"g": Dict{"use": "fictional_pack"}}}}},
	}}
	_, err := org.SyncPush(cfg, SyncOptions{IsForce: true, SyncHives: map[string]bool{"investigation_pack": true, "investigation_plan": true}})
	require.Error(t, err)
	require.Contains(t, ms.HiveStore["investigation_pack/"+testOID], "stale")
	require.Empty(t, log.snapshot())
}

func TestInvestigationPackDryRunReportsWithoutWriting(t *testing.T) {
	ms, org := setupMock(t)
	log := watchHiveWrites(ms)
	ops, err := org.SyncPush(investigationPackConfig(), SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"investigation_policy": true, "investigation_pack": true, "investigation_plan": true}})
	require.NoError(t, err)
	require.Len(t, ops, 3)
	require.Empty(t, log.snapshot())
	require.Empty(t, ms.HiveStore["investigation_pack/"+testOID])
}

// Removals wait for hives that come after the investigation hives in KnownHives
// too: their writes land first, and a refused write there removes nothing.
func TestInvestigationRemovalsWaitForLaterHives(t *testing.T) {
	cfg := OrgConfig{Version: 3, Hives: orgSyncHives{
		"investigation_pack":   {},
		"investigation_plan":   {},
		"host_posture_rule":    {"rule": {Data: Dict{"value": "x"}}},
		"investigation_policy": {},
	}}
	opts := SyncOptions{IsForce: true, SyncHives: map[string]bool{"investigation_policy": true, "investigation_pack": true, "investigation_plan": true, "host_posture_rule": true}}
	seed := func(ms *MockServer) {
		ms.HiveStore["investigation_policy/"+testOID] = map[string]HiveData{"default": {Data: Dict{"x": true}}}
		ms.HiveStore["investigation_pack/"+testOID] = map[string]HiveData{"fictional_pack": {Data: Dict{"yaml": "x"}}}
		ms.HiveStore["investigation_plan/"+testOID] = map[string]HiveData{"fictional-triage": {Data: Dict{"x": true}}}
	}

	t.Run("later hive applied first", func(t *testing.T) {
		ms, org := setupMock(t)
		seed(ms)
		log := watchHiveWrites(ms)
		_, err := org.SyncPush(cfg, opts)
		require.NoError(t, err)
		require.Equal(t, []string{
			"write host_posture_rule/rule",
			"delete investigation_plan/fictional-triage",
			"delete investigation_pack/fictional_pack",
			"delete investigation_policy/default",
		}, log.snapshot())
	})

	t.Run("later hive refused", func(t *testing.T) {
		ms, org := setupMock(t)
		seed(ms)
		log := watchHiveWrites(ms)
		inner := ms.CustomHandlers["/v1/hive/"]
		ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodPost && strings.HasPrefix(req.URL.Path, "/v1/hive/host_posture_rule/") {
				ms.writeError(w, http.StatusBadRequest, "refused")
				return
			}
			inner(w, req)
		}
		_, err := org.SyncPush(cfg, opts)
		require.Error(t, err)
		require.Empty(t, log.snapshot())
		require.Contains(t, ms.HiveStore["investigation_plan/"+testOID], "fictional-triage")
		require.Contains(t, ms.HiveStore["investigation_pack/"+testOID], "fictional_pack")
		require.Contains(t, ms.HiveStore["investigation_policy/"+testOID], "default")
	})
}
