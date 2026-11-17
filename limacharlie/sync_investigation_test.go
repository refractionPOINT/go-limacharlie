package limacharlie

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/url"
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

// The listing is the diff baseline, not a later Get of the changed record.
func TestInvestigationSyncNativeCASConflict(t *testing.T) {
	for _, hive := range []string{"investigation_policy", "investigation_plan"} {
		t.Run(hive, func(t *testing.T) {
			ms, org := setupMock(t)
			key := hive + "/" + testOID
			ms.HiveStore[key] = map[string]HiveData{"default": {Data: Dict{"value": "concurrent"}, SysMtd: SysMtd{Etag: "new-version"}}}
			forms := make(chan string, 1)
			ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodGet {
					w.Header().Set("Content-Type", "application/json")
					// The store has advanced since the snapshot used for the diff.
					w.Write([]byte(`{"default":{"data":{"value":"old"},"usr_mtd":{},"sys_mtd":{"etag":"listed-version"}}}`))
					return
				}
				body, _ := io.ReadAll(req.Body)
				form, _ := url.ParseQuery(string(body))
				forms <- form.Get("etag")
				req.Body = io.NopCloser(bytes.NewReader(body))
				ms.handleHive(w, req)
			}
			_, err := org.SyncPush(OrgConfig{Version: 3, Hives: orgSyncHives{hive: {"default": {Data: Dict{"value": "desired"}}}}}, SyncOptions{SyncHives: map[string]bool{hive: true}})
			require.ErrorContains(t, err, "ETAG_MISMATCH")
			require.Equal(t, "listed-version", <-forms)
			require.Equal(t, "concurrent", ms.HiveStore[key]["default"].Data["value"])
		})
	}
}

func TestInvestigationSyncNativeCreateAndMetadataOnly(t *testing.T) {
	ms, org := setupMock(t)
	for _, hive := range []string{"investigation_policy", "investigation_plan"} {
		ha := HiveArgs{HiveName: hive, PartitionKey: testOID, Key: "default"}
		// Missing-record create follows native semantics, with an empty ETag.
		require.NoError(t, org.writeInvestigationHive(ha, SyncHiveData{Data: Dict{"keep": "original"}}, ""))
		cfg := OrgConfig{Version: 3, Hives: orgSyncHives{hive: {"default": {UsrMtd: UsrMtd{Enabled: true, Comment: "metadata only"}}}}}
		_, err := org.SyncPush(cfg, SyncOptions{SyncHives: map[string]bool{hive: true}})
		require.NoError(t, err)
		record := ms.HiveStore[hive+"/"+testOID]["default"]
		require.Equal(t, "original", record.Data["keep"])
		require.Equal(t, "metadata only", record.UsrMtd.Comment)
		// A stale version fails; the current one succeeds.
		require.ErrorContains(t, org.writeInvestigationHive(ha, SyncHiveData{Data: Dict{"keep": "bad"}}, "stale"), "ETAG_MISMATCH")
		require.NoError(t, org.writeInvestigationHive(ha, SyncHiveData{Data: Dict{"keep": "updated"}}, record.SysMtd.Etag))
	}
}

func TestInvestigationSyncAllForceOmittedHivesUntouched(t *testing.T) {
	ms, org := setupMock(t)
	for _, h := range []string{"investigation_policy", "investigation_plan"} {
		ms.HiveStore[h+"/"+testOID] = map[string]HiveData{"default": {Data: Dict{"keep": true}}}
	}
	// SyncAll selects exports, but force deletion only visits config-present hives.
	opts := SyncAll()
	opts.IsForce = true
	_, err := org.SyncPush(OrgConfig{Version: 3, Hives: orgSyncHives{"fp": {}}}, opts)
	require.NoError(t, err)
	for _, h := range []string{"investigation_policy", "investigation_plan"} {
		require.Contains(t, ms.HiveStore[h+"/"+testOID], "default")
	}
}

func TestInvestigationSyncOtherHiveWriteProtocolUnchanged(t *testing.T) {
	for _, hive := range KnownHives {
		if isInvestigationSyncHive(hive) {
			continue
		}
		t.Run(hive, func(t *testing.T) {
			ms, org := setupMock(t)
			posts := make(chan url.Values, 3)
			ms.CustomHandlers["/v1/hive/"] = func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPost {
					body, _ := io.ReadAll(req.Body)
					form, _ := url.ParseQuery(string(body))
					posts <- form
					req.Body = io.NopCloser(bytes.NewReader(body))
				}
				ms.handleHive(w, req)
			}
			opts := SyncOptions{SyncHives: map[string]bool{hive: true}}
			cfg := OrgConfig{Version: 3, Hives: orgSyncHives{hive: {"default": {Data: Dict{"value": "old"}}}}}
			_, err := org.SyncPush(cfg, opts)
			require.NoError(t, err)
			created := <-posts
			require.False(t, created.Has("etag"))
			require.True(t, created.Has("gzdata"))
			cfg.Hives[hive]["default"] = SyncHiveData{Data: Dict{"value": "new"}}
			_, err = org.SyncPush(cfg, opts)
			require.NoError(t, err)
			updated := <-posts
			require.False(t, updated.Has("etag"))
			require.True(t, updated.Has("sys_mtd"))
			require.True(t, updated.Has("data"))
			require.False(t, updated.Has("gzdata"))
			cfg.Hives[hive]["default"] = SyncHiveData{UsrMtd: UsrMtd{Comment: "metadata"}}
			_, err = org.SyncPush(cfg, opts)
			require.NoError(t, err)
			metadata := <-posts
			require.True(t, metadata.Has("etag"))
			require.False(t, metadata.Has("data"))
			require.False(t, metadata.Has("gzdata"))
			require.Equal(t, "new", ms.HiveStore[hive+"/"+testOID]["default"].Data["value"])
		})
	}
}
