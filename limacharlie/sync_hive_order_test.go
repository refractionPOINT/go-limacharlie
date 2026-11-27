package limacharlie

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// orderWriteLog wraps the mock hive endpoint: it records every write and delete
// in arrival order, and can refuse a write with refuse.
type orderWriteLog struct {
	mu     sync.Mutex
	events []string
}

func (l *orderWriteLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

func (l *orderWriteLog) install(ms *MockServer, hive string, refuse func(key string, body string) bool) {
	ms.CustomHandlers["/v1/hive/"+hive+"/"] = func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(req.URL.Path, "/v1/hive/"+hive+"/"), "/"), "/")
		if (req.Method == http.MethodPost || req.Method == http.MethodDelete) && len(parts) >= 2 {
			key := parts[1]
			if req.Method == http.MethodPost && refuse != nil && refuse(key, readBody(req)) {
				ms.writeError(w, 400, "missing_arg")
				return
			}
			l.mu.Lock()
			l.events = append(l.events, req.Method+" "+key)
			l.mu.Unlock()
		}
		ms.handleHive(w, req)
	}
}

func opNames(ops []OrgSyncOperation) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, fmt.Sprintf("%s add=%v rm=%v", op.ElementName, op.IsAdded, op.IsRemoved))
	}
	return out
}

func opElements(ops []OrgSyncOperation) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, op.ElementName)
	}
	return out
}

func planRecord(source string) SyncHiveData {
	return SyncHiveData{Data: Dict{"yaml": source}, UsrMtd: UsrMtd{Enabled: true}}
}

func checkSource(name string, calls ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "kind: check\nname: %s\ngather:\n", name)
	for i, c := range calls {
		fmt.Fprintf(&b, "  step%d:\n    use: template:%s\n", i, c)
	}
	return b.String()
}

func alertSource(name string, check string) string {
	return fmt.Sprintf("kind: alert\nname: %s\ngather:\n  s:\n    use: template:%s\n", name, check)
}

// A hive's records are written by name, whatever the map order of the config.
func TestSyncHiveWritesRecordsByName(t *testing.T) {
	var names []string
	for i := 0; i < 40; i++ {
		names = append(names, fmt.Sprintf("rec-%02d", (i*17)%40))
	}
	want := slices.Clone(names)
	sort.Strings(want)
	cfg := OrgConfig{Version: 3, Hives: orgSyncHives{"cloudsec_query": SyncHiveConfigData{}}}
	for _, n := range names {
		cfg.Hives["cloudsec_query"][n] = SyncHiveData{Data: Dict{"n": n}, UsrMtd: UsrMtd{Enabled: true}}
	}
	var firstOps []string
	for run := 0; run < 25; run++ {
		ms, org := setupMock(t)
		log := &orderWriteLog{}
		log.install(ms, "cloudsec_query", nil)
		dry, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"cloudsec_query": true}, IsDryRun: true})
		require.NoError(t, err)
		require.Empty(t, log.list(), "a dry run writes nothing")
		ops, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"cloudsec_query": true}})
		require.NoError(t, err)
		var written []string
		for _, e := range log.list() {
			written = append(written, strings.TrimPrefix(e, "POST "))
		}
		require.Equal(t, want, written)
		require.Equal(t, opElements(dry), opElements(ops), "a dry run lists the operations in the order a real push applies them")
		if run == 0 {
			firstOps = opNames(ops)
		}
		require.Equal(t, firstOps, opNames(ops), "two runs of the same push give the same operation list")
	}
}

// A push that changes a check and the alerts that call it is never refused
// because an alert was written before its check.
func TestSyncInvestigationPlanWritesChecksFirst(t *testing.T) {
	// Names sort alerts before checks, so name order alone would fail.
	cfg := OrgConfig{Version: 3, Hives: orgSyncHives{"investigation_plan": SyncHiveConfigData{
		"a-alert":    planRecord(alertSource("a-alert", "z-check")),
		"b-alert":    planRecord(alertSource("b-alert", "y-check")),
		"c-alert":    planRecord("kind: alert\nname: c-alert\n"),
		"y-check":    planRecord(checkSource("y-check")),
		"z-check":    planRecord(checkSource("z-check", "y-check")),
		"x-check":    planRecord(checkSource("x-check", "w-missing-from-push")),
		"unreadable": planRecord(":\n\t- not yaml {"),
		"no-yaml":    {Data: Dict{"other": 1}, UsrMtd: UsrMtd{Enabled: true}},
	}}}
	want := []string{"x-check", "y-check", "z-check", "a-alert", "b-alert", "c-alert", "no-yaml", "unreadable"}
	var firstOps []string
	for run := 0; run < 40; run++ {
		ms, org := setupMock(t)
		log := &orderWriteLog{}
		var done sync.Mutex
		written := map[string]bool{}
		// The hive refuses an alert while a check of the push is not stored yet.
		log.install(ms, "investigation_plan", func(key, body string) bool {
			done.Lock()
			defer done.Unlock()
			if strings.HasSuffix(key, "-alert") {
				for _, c := range []string{"x-check", "y-check", "z-check"} {
					if !written[c] {
						return true
					}
				}
			}
			written[key] = true
			return false
		})
		ops, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"investigation_plan": true}})
		require.NoError(t, err)
		var order []string
		for _, e := range log.list() {
			order = append(order, strings.TrimPrefix(e, "POST "))
		}
		require.Equal(t, want, order)
		if run == 0 {
			firstOps = opNames(ops)
		}
		require.Equal(t, firstOps, opNames(ops))
	}
}

// Without the order the same push is refused: the refusal is real, not a test artefact.
func TestSyncInvestigationPlanRefusalStopsAtFirstRefusedWrite(t *testing.T) {
	ms, org := setupMock(t)
	log := &orderWriteLog{}
	log.install(ms, "investigation_plan", func(key, body string) bool { return key == "b-check" })
	cfg := OrgConfig{Version: 3, Hives: orgSyncHives{"investigation_plan": SyncHiveConfigData{
		"a-check": planRecord(checkSource("a-check")),
		"b-check": planRecord(checkSource("b-check")),
		"c-check": planRecord(checkSource("c-check")),
		"a-alert": planRecord(alertSource("a-alert", "a-check")),
	}}}
	_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{"investigation_plan": true}})
	require.Error(t, err)
	// Always the same records applied before the refusal.
	require.Equal(t, []string{"POST a-check"}, log.list())
}

// Removals are deterministic too: what calls a record is removed before it.
func TestSyncInvestigationPlanRemovalOrder(t *testing.T) {
	seed := SyncHiveConfigData{
		"a-alert": planRecord(alertSource("a-alert", "z-check")),
		"b-alert": planRecord(alertSource("b-alert", "y-check")),
		"y-check": planRecord(checkSource("y-check")),
		"z-check": planRecord(checkSource("z-check", "y-check")),
		"keep":    planRecord(alertSource("keep", "y-check")),
	}
	want := []string{"DELETE b-alert", "DELETE a-alert", "DELETE z-check", "DELETE y-check"}
	for run := 0; run < 25; run++ {
		ms, org := setupMock(t)
		ms.HiveStore["investigation_plan/"+testOID] = map[string]HiveData{}
		for k, v := range seed {
			ms.HiveStore["investigation_plan/"+testOID][k] = HiveData{Data: v.Data, UsrMtd: v.UsrMtd, SysMtd: SysMtd{Etag: "e-" + k}}
		}
		log := &orderWriteLog{}
		log.install(ms, "investigation_plan", nil)
		cfg := OrgConfig{Version: 3, Hives: orgSyncHives{"investigation_plan": SyncHiveConfigData{"keep": seed["keep"]}}}
		opts := SyncOptions{SyncHives: map[string]bool{"investigation_plan": true}, IsForce: true}
		dryOpts := opts
		dryOpts.IsDryRun = true
		dry, err := org.SyncPush(cfg, dryOpts)
		require.NoError(t, err)
		ops, err := org.SyncPush(cfg, opts)
		require.NoError(t, err)
		require.Equal(t, want, log.list())
		require.Equal(t, opNames(dry), opNames(ops))
		require.Equal(t, []string{
			"investigation_plan/b-alert", "investigation_plan/a-alert", "investigation_plan/z-check", "investigation_plan/y-check",
		}, opElements(ops)[len(opElements(ops))-4:])
	}
}

func TestCallsFirst(t *testing.T) {
	calls := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": nil,
		"d": {"d", "outside"},
	}
	require.Equal(t, []string{"c", "b", "a", "d"}, callsFirst([]string{"a", "b", "c", "d"}, calls))
	// A cycle falls back to name order, and does not hold back what is not in it.
	cycle := map[string][]string{"a": {"b"}, "b": {"a"}, "c": nil}
	require.Equal(t, []string{"c", "a", "b"}, callsFirst([]string{"a", "b", "c"}, cycle))
}

func TestInvestigationRecordShape(t *testing.T) {
	isCheck, called := investigationRecordShape(Dict{"yaml": checkSource("c", "z", "a", "z") + "act:\n  - on_lease_end:\n      kind: check\n      check: template:lease\n"})
	require.True(t, isCheck)
	require.Equal(t, []string{"a", "lease", "z"}, called)
	isCheck, _ = investigationRecordShape(Dict{"yaml": alertSource("x", "c")})
	require.False(t, isCheck)
	isCheck, _ = investigationRecordShape(Dict{"yaml": 7})
	require.False(t, isCheck)
}
