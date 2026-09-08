package limacharlie

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSyncACLHiveRoundTrip(t *testing.T) {
	for _, selection := range []string{"sync_all", "known_hives"} {
		t.Run(selection, func(t *testing.T) {
			source, sourceOrg := setupMock(t)
			source.HiveStore["acl/"+testOID] = map[string]HiveData{
				"finance": {
					Data: Dict{"members": List{
						Dict{"type": "user", "id": "test-user-uid"},
						Dict{"type": "api_key", "id": "test-key-id"},
						Dict{"type": "group", "id": "test-group-id"},
					}},
					UsrMtd: UsrMtd{Enabled: true, Tags: []string{"managed"}, Comment: "IaC scope"},
				},
				"locked": {Data: Dict{"members": List{}}, UsrMtd: UsrMtd{Enabled: true, Tags: []string{}}},
			}
			source.HiveStore["secret/"+testOID] = map[string]HiveData{
				"credential": {
					Data: Dict{"secret": "test-only-value"},
					UsrMtd: UsrMtd{Enabled: false, Expiry: 1900000000,
						Tags: []string{"ordinary", "acl:finance", "acl:locked"}, Comment: "preserve classification"},
				},
			}
			hives := SyncAll().SyncHives
			if selection == "known_hives" {
				hives = map[string]bool{}
				for _, hive := range KnownHives {
					hives[hive] = true
				}
			}
			opts := SyncOptions{SyncHives: hives, IncludeLoader: LocalFileIncludeLoader}
			fetched, err := sourceOrg.SyncFetch(opts)
			require.NoError(t, err)
			require.Contains(t, fetched.Hives, "acl", "scope membership must not disappear from a full export")
			require.Len(t, fetched.Hives["acl"], 2)
			yamlConfig, err := yaml.Marshal(fetched)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "org.yaml")
			require.NoError(t, os.WriteFile(path, yamlConfig, 0600))

			target, targetOrg := setupMock(t)
			ops, err := targetOrg.SyncPushFromFiles(path, opts)
			require.NoError(t, err)
			require.NotEmpty(t, ops)
			for _, hive := range []string{"acl", "secret"} {
				require.Len(t, target.HiveStore[hive+"/"+testOID], len(source.HiveStore[hive+"/"+testOID]))
				for name, want := range source.HiveStore[hive+"/"+testOID] {
					got := target.HiveStore[hive+"/"+testOID][name]
					require.Equal(t, want.UsrMtd, got.UsrMtd, "%s/%s metadata changed", hive, name)
					wantData, err := yaml.Marshal(want.Data)
					require.NoError(t, err)
					gotData, err := yaml.Marshal(got.Data)
					require.NoError(t, err)
					require.Equal(t, string(wantData), string(gotData), "%s/%s content changed", hive, name)
				}
			}
			// A second push is a true no-op, not an ACL metadata rewrite.
			target.ResetCalls()
			ops, err = targetOrg.SyncPushFromFiles(path, opts)
			require.NoError(t, err)
			for _, op := range ops {
				require.False(t, op.IsAdded)
				require.False(t, op.IsRemoved)
			}
			for _, call := range target.Calls() {
				require.NotEqual(t, http.MethodPost, call.Method)
				require.NotEqual(t, http.MethodDelete, call.Method)
			}
		})
	}
}

func TestSyncACLRequiresExplicitSelection(t *testing.T) {
	ms, org := setupMock(t)
	conf := OrgConfig{Version: OrgConfigLatestVersion, Hives: orgSyncHives{
		"acl": {"finance": {Data: Dict{"members": List{}}, UsrMtd: UsrMtd{Enabled: true}}},
	}}
	for _, hives := range []map[string]bool{nil, {}, {"acl": false}} {
		ops, err := org.SyncPush(conf, SyncOptions{SyncHives: hives})
		require.NoError(t, err)
		require.Empty(t, ops)
		require.Empty(t, ms.HiveStore["acl/"+testOID])
	}
}

func TestSyncACLPermissionErrors(t *testing.T) {
	for _, operation := range []string{"fetch", "push"} {
		t.Run(operation, func(t *testing.T) {
			ms, org := setupMock(t)
			ms.CustomHandlers["/v1/hive/acl/"] = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if operation == "push" && r.Method == http.MethodGet {
					w.Write([]byte(`{}`))
					return
				}
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"ACL_PERMISSION_REQUIRED"}`))
			}
			opts := SyncOptions{SyncHives: map[string]bool{"acl": true}}
			var err error
			if operation == "fetch" {
				_, err = org.SyncFetch(opts)
			} else {
				_, err = org.SyncPush(OrgConfig{Hives: orgSyncHives{
					"acl": {"finance": {Data: Dict{"members": List{}}}},
				}}, opts)
			}
			require.ErrorContains(t, err, "ACL_PERMISSION_REQUIRED")
		})
	}
}
