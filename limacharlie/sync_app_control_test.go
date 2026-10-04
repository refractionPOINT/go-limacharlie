package limacharlie

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Every hive in KnownHives must be selected by SyncAll(), and vice versa, so a
// hive added to one list cannot silently be missing from a full export.
func TestSyncAllMatchesKnownHives(t *testing.T) {
	all := SyncAll().SyncHives
	require.Len(t, all, len(KnownHives))
	for _, hive := range KnownHives {
		require.True(t, all[hive], "%s is in KnownHives but not selected by SyncAll", hive)
	}
	for _, hive := range []string{"app_control_policy", "app_control_rule"} {
		require.Contains(t, KnownHives, hive)
	}
}

func TestSyncAppControlHivesRoundTrip(t *testing.T) {
	hives := []string{"app_control_policy", "app_control_rule"}
	source, sourceOrg := setupMock(t)
	source.HiveStore["app_control_policy/"+testOID] = map[string]HiveData{
		"workstations": {
			Data:   Dict{"mode": "monitor"},
			UsrMtd: UsrMtd{Enabled: true, Tags: []string{"managed"}, Comment: "IaC policy"},
		},
	}
	source.HiveStore["app_control_rule/"+testOID] = map[string]HiveData{
		"block-tool": {
			Data:   Dict{"action": "deny", "path": "C:\\tools\\tool.exe"},
			UsrMtd: UsrMtd{Enabled: true, Tags: []string{}},
		},
	}
	opts := SyncOptions{SyncHives: map[string]bool{hives[0]: true, hives[1]: true}, IncludeLoader: LocalFileIncludeLoader}
	fetched, err := sourceOrg.SyncFetch(opts)
	require.NoError(t, err)
	for _, hive := range hives {
		require.Len(t, fetched.Hives[hive], 1, "%s missing from export", hive)
	}

	yamlConfig, err := yaml.Marshal(fetched)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "org.yaml")
	require.NoError(t, os.WriteFile(path, yamlConfig, 0600))

	target, targetOrg := setupMock(t)
	ops, err := targetOrg.SyncPushFromFiles(path, opts)
	require.NoError(t, err)
	require.NotEmpty(t, ops)
	for _, hive := range hives {
		require.Len(t, target.HiveStore[hive+"/"+testOID], 1, "%s not pushed", hive)
		for name, want := range source.HiveStore[hive+"/"+testOID] {
			got := target.HiveStore[hive+"/"+testOID][name]
			require.Equal(t, want.UsrMtd, got.UsrMtd)
			require.Equal(t, want.Data, got.Data)
		}
	}
}
