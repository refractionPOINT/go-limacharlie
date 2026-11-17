package limacharlie

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestInvestigationSyncFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/investigation/policy-plan.yaml")
	require.NoError(t, err)
	var cfg OrgConfig
	require.NoError(t, yaml.Unmarshal(raw, &cfg))
	cfg.Version = OrgConfigLatestVersion
	for _, h := range []string{"investigation_policy", "investigation_plan"} {
		require.Contains(t, KnownHives, h)
		require.True(t, SyncAll().SyncHives[h])
	}
	// Exercise opaque fields and all user metadata, including a disabled record.
	rec := cfg.Hives["investigation_policy"]["default"]
	rec.Data["future_field"] = Dict{"identity": "fictional", "triggers": []string{"fictional"}}
	rec.UsrMtd.Expiry = 1900000000
	rec.UsrMtd.Comment = "fictional IaC"
	rec.UsrMtd.Enabled = false
	cfg.Hives["investigation_policy"]["default"] = rec
	ms, org := setupMock(t)
	opts := SyncOptions{SyncHives: map[string]bool{"investigation_policy": true, "investigation_plan": true}}
	_, err = org.SyncPush(cfg, opts)
	require.NoError(t, err)
	// Change an existing record, exercising the native version read/write path.
	rec.Data["future_field"] = "updated"
	cfg.Hives["investigation_policy"]["default"] = rec
	_, err = org.SyncPush(cfg, opts)
	require.NoError(t, err)
	got, err := org.SyncFetch(opts)
	require.NoError(t, err)
	a, err := yaml.Marshal(cfg.Hives)
	require.NoError(t, err)
	b, err := yaml.Marshal(got.Hives)
	require.NoError(t, err)
	require.Equal(t, string(a), string(b))
	require.False(t, strings.Contains(string(b), "sys_mtd"))
	require.False(t, strings.Contains(string(b), "etag"))
	// Force deletion is limited to the selected hive and selected tags.
	ms.HiveStore["investigation_policy/"+testOID]["keep"] = HiveData{Data: Dict{"x": true}, UsrMtd: UsrMtd{Tags: []string{"other"}}}
	opts.IsForce = true
	opts.Tags = []string{"fictional-iac"}
	opts.SyncHives["investigation_plan"] = false
	cfg.Hives["investigation_policy"] = map[string]SyncHiveData{}
	cfg.Hives["investigation_plan"] = map[string]SyncHiveData{}
	_, err = org.SyncPush(cfg, opts)
	require.NoError(t, err)
	require.NotContains(t, ms.HiveStore["investigation_policy/"+testOID], "default")
	require.Contains(t, ms.HiveStore["investigation_policy/"+testOID], "keep")
	require.Contains(t, ms.HiveStore["investigation_plan/"+testOID], "fictional-triage")
}

// Conflicts must be returned, without retrying an unconditional overwrite.
func TestInvestigationSyncNativeCASConflict(t *testing.T) {
	ms, org := setupMock(t)
	seen := false
	ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodGet {
			w.Write([]byte(`{"data":{"old":true},"usr_mtd":{},"sys_mtd":{"etag":"native-version"}}`))
			return
		}
		require.NoError(t, req.ParseForm())
		require.Equal(t, "native-version", req.Form.Get("etag"))
		require.Empty(t, req.Form.Get("sys_mtd"))
		seen = true
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"ETAG_MISMATCH"}`))
	}
	err := org.updateHiveConfigData(HiveArgs{HiveName: "investigation_policy", PartitionKey: testOID, Key: "default"}, SyncHiveData{Data: Dict{}, UsrMtd: UsrMtd{Enabled: true}})
	require.Error(t, err)
	require.True(t, seen)
	require.Contains(t, err.Error(), "ETAG_MISMATCH")
}
