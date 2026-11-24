package limacharlie

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// productHives are the Cloud Security, Email Security and Host Posture hives.
var productHives = []string{
	"cloudsec_provider",
	"cloudsec_policy",
	"cloudsec_query",
	"cloudsec_code_rule",
	"mailsec_provider",
	"mailsec_policy",
	"dr-mail",
	"host_posture_rule",
}

func productHiveOptions() SyncOptions {
	hives := map[string]bool{}
	for _, hive := range productHives {
		hives[hive] = true
	}
	return SyncOptions{SyncHives: hives, IncludeLoader: LocalFileIncludeLoader}
}

// seedProductHives writes one record per product hive into ms, as Hive would
// store them. The mailsec_provider record carries the provider tag Hive stamps.
func seedProductHives(ms *MockServer) {
	put := func(hive, key string, data Dict, mtd UsrMtd) {
		store := ms.HiveStore[hive+"/"+testOID]
		if store == nil {
			store = map[string]HiveData{}
			ms.HiveStore[hive+"/"+testOID] = store
		}
		if mtd.Tags == nil {
			mtd.Tags = []string{} // what Hive returns for a record without tags
		}
		store[key] = HiveData{Data: data, UsrMtd: mtd}
	}
	put("cloudsec_provider", "aws-prod", Dict{"provider_type": "aws", "credentials": "hive://secret/aws-prod"}, UsrMtd{Enabled: true, Tags: []string{"tenant"}})
	put("cloudsec_policy", "crown-jewels", Dict{"policy_type": "classification", "classification": Dict{"rules": []interface{}{Dict{"tag": []interface{}{"pii"}}}}}, UsrMtd{Enabled: true})
	put("cloudsec_query", "public-buckets", Dict{"name": "Public buckets", "query": "MATCH (b:Bucket) RETURN b"}, UsrMtd{Enabled: true})
	put("cloudsec_code_rule", "no-eval", Dict{"rules": []interface{}{Dict{"id": "no-eval", "pattern": "eval(...)"}}}, UsrMtd{Enabled: true})
	put("mailsec_provider", "m365", Dict{"provider": "m365", "credentials": "hive://secret/m365-app"}, UsrMtd{Enabled: true, Tags: []string{"tenant", "mailsec:provider:m365"}})
	put("mailsec_policy", "10-vips", Dict{"policy_type": "vips", "vips": Dict{"addresses": []interface{}{"ceo@example.com"}}}, UsrMtd{Enabled: true})
	put("dr-mail", "lookalike-sender", Dict{"detect": Dict{"op": "contains", "path": "sender", "value": "examp1e"}, "respond": []interface{}{}}, UsrMtd{Enabled: true})
	put("host_posture_rule", "acme.firewall-on", Dict{"rule": Dict{"id": "acme.firewall-on"}}, UsrMtd{Enabled: true, Comment: "firewall"})
}

func hiveWrites(ms *MockServer) []string {
	var writes []string
	for _, c := range ms.Calls() {
		if c.Method == http.MethodPost && strings.HasPrefix(c.Path, "/v1/hive/") {
			writes = append(writes, strings.TrimPrefix(c.Path, "/v1/hive/"))
		}
	}
	return writes
}

func writeConfig(t *testing.T, conf OrgConfig) string {
	t.Helper()
	b, err := yaml.Marshal(conf)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "org.yaml")
	require.NoError(t, os.WriteFile(path, b, 0600))
	return path
}

func TestProductHivesAreKnownAndProvidersComeFirst(t *testing.T) {
	index := map[string]int{}
	for i, hive := range KnownHives {
		index[hive] = i
	}
	for _, hive := range productHives {
		_, ok := index[hive]
		require.True(t, ok, "%s missing from KnownHives", hive)
		require.True(t, SyncAll().SyncHives[hive], "%s missing from SyncAll", hive)
	}
	for _, users := range [][]string{
		{"cloudsec_provider", "cloudsec_policy", "cloudsec_query", "cloudsec_code_rule"},
		{"mailsec_provider", "mailsec_policy", "dr-mail"},
	} {
		for _, user := range users[1:] {
			require.Less(t, index[users[0]], index[user], "%s must be applied before %s", users[0], user)
		}
	}
}

// A full export of the product hives pushes to another org unchanged, and
// pushing the same files again writes nothing.
func TestSyncProductHivesRoundTrip(t *testing.T) {
	source, sourceOrg := setupMock(t)
	seedProductHives(source)
	opts := productHiveOptions()

	fetched, err := sourceOrg.SyncFetch(opts)
	require.NoError(t, err)
	for _, hive := range productHives {
		require.Len(t, fetched.Hives[hive], 1, "%s missing from export", hive)
	}
	// The tag Hive stamps is not exported: the record in Git is the one a
	// person writes.
	require.Equal(t, []string{"tenant"}, fetched.Hives["mailsec_provider"]["m365"].UsrMtd.Tags)

	path := writeConfig(t, fetched)
	target, targetOrg := setupMock(t)
	ops, err := targetOrg.SyncPushFromFiles(path, opts)
	require.NoError(t, err)
	require.Len(t, ops, len(productHives))
	for _, hive := range productHives {
		got := target.HiveStore[hive+"/"+testOID]
		require.Len(t, got, 1, "%s not pushed", hive)
		for name, want := range source.HiveStore[hive+"/"+testOID] {
			wantData, err := json.Marshal(want.Data)
			require.NoError(t, err)
			gotData, err := json.Marshal(got[name].Data)
			require.NoError(t, err)
			require.JSONEq(t, string(wantData), string(gotData), "%s/%s data", hive, name)
			require.Equal(t, want.UsrMtd, got[name].UsrMtd, "%s/%s usr_mtd", hive, name)
		}
	}

	// Writes follow KnownHives order, providers first.
	writes := hiveWrites(target)
	require.Len(t, writes, len(productHives))
	for i, hive := range productHives {
		require.True(t, strings.HasPrefix(writes[i], hive+"/"), "write %d is %s, want hive %s", i, writes[i], hive)
	}

	target.ResetCalls()
	ops, err = targetOrg.SyncPushFromFiles(path, opts)
	require.NoError(t, err)
	require.Empty(t, hiveWrites(target), "a repeated push must not rewrite any record")
	for _, op := range ops {
		require.False(t, op.IsAdded, "%s reported as changed on a repeated push", op.ElementName)
	}
}

// An Email Security connection written in Git without Hive's provider tag, or
// exported with it by an older SDK, is not rewritten on every sync, and a real
// change still is.
func TestSyncMailSecProviderIgnoresServerProviderTag(t *testing.T) {
	ms, org := setupMock(t)
	opts := SyncOptions{SyncHives: map[string]bool{"mailsec_provider": true}, IncludeLoader: LocalFileIncludeLoader}
	record := func(tags ...string) OrgConfig {
		return OrgConfig{Version: 3, Hives: orgSyncHives{"mailsec_provider": {
			"m365": {Data: Dict{"provider": "m365", "credentials": "hive://secret/m365-app"}, UsrMtd: UsrMtd{Enabled: true, Tags: tags}},
		}}}
	}

	_, err := org.SyncPushFromFiles(writeConfig(t, record("tenant")), opts)
	require.NoError(t, err)
	require.Equal(t, []string{"tenant", "mailsec:provider:m365"}, ms.HiveStore["mailsec_provider/"+testOID]["m365"].UsrMtd.Tags)

	for name, conf := range map[string]OrgConfig{
		"written without the tag": record("tenant"),
		"exported with the tag":   record("tenant", "mailsec:provider:m365"),
	} {
		ms.ResetCalls()
		_, err := org.SyncPushFromFiles(writeConfig(t, conf), opts)
		require.NoError(t, err)
		require.Empty(t, hiveWrites(ms), "%s: unchanged connection rewritten", name)
	}

	ms.ResetCalls()
	_, err = org.SyncPushFromFiles(writeConfig(t, record("tenant", "vip")), opts)
	require.NoError(t, err)
	require.Len(t, hiveWrites(ms), 1, "a tag change must still be written")
	require.Equal(t, []string{"tenant", "vip", "mailsec:provider:m365"}, ms.HiveStore["mailsec_provider/"+testOID]["m365"].UsrMtd.Tags)
}

// Force removes records missing from the configuration only in the hives the
// configuration lists, so adding product hives to a sync never wipes one that
// the repository does not mention.
func TestSyncForceLeavesUnlistedProductHivesAlone(t *testing.T) {
	ms, org := setupMock(t)
	seedProductHives(ms)
	opts := productHiveOptions()
	opts.IsForce = true

	conf := OrgConfig{Version: 3, Hives: orgSyncHives{"cloudsec_query": {}}}
	_, err := org.SyncPushFromFiles(writeConfig(t, conf), opts)
	require.NoError(t, err)
	require.Empty(t, ms.HiveStore["cloudsec_query/"+testOID], "listed empty hive must be cleared")
	for _, hive := range productHives {
		if hive == "cloudsec_query" {
			continue
		}
		require.Len(t, ms.HiveStore[hive+"/"+testOID], 1, "%s was not in the configuration and must be untouched", hive)
	}
}
