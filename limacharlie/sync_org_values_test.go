package limacharlie

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func orgValuesConf(t *testing.T, raw string) OrgConfig {
	t.Helper()
	conf := OrgConfig{}
	require.NoError(t, yaml.Unmarshal([]byte(raw), &conf))
	return conf
}

// sensor_auto_upgrade is pushed and fetched like any other org value.
func TestSyncSensorAutoUpgradeOrgValue(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)

	ops, err := org.SyncPush(orgValuesConf(t, "org-value:\n  sensor_auto_upgrade: \"true\"\n"), SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "sensor_auto_upgrade", IsAdded: true}}, ops)
	require.Equal(t, "true", ms.OrgValues["sensor_auto_upgrade"])

	fetched, err := org.SyncFetch(SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, "true", fetched.OrgValues["sensor_auto_upgrade"])

	// The same push again changes nothing.
	ops, err = org.SyncPush(orgValuesConf(t, "org-value:\n  sensor_auto_upgrade: \"true\"\n"), SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "sensor_auto_upgrade"}}, ops)

	// The value names a release channel. Sync does not interpret it: a channel
	// is pushed and fetched as written, and it is the API that refuses a value
	// it does not know.
	for _, channel := range []string{"latest", "stable"} {
		ops, err = org.SyncPush(orgValuesConf(t, "org-value:\n  sensor_auto_upgrade: "+channel+"\n"), SyncOptions{SyncOrgValues: true})
		require.NoError(t, err)
		require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "sensor_auto_upgrade", IsAdded: true}}, ops, channel)
		require.Equal(t, channel, ms.OrgValues["sensor_auto_upgrade"])
		fetched, err = org.SyncFetch(SyncOptions{SyncOrgValues: true})
		require.NoError(t, err)
		require.Equal(t, channel, fetched.OrgValues["sensor_auto_upgrade"])
	}
}

// A forced push removes the org values the config does not mention, except
// sensor_auto_upgrade: a template that does not name it must not turn off what
// the org's owner turned on.
func TestSyncForceKeepsSensorAutoUpgrade(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	ms.OrgValues["sensor_auto_upgrade"] = "true"
	ms.OrgValues["twilio"] = "to-be-removed"
	ms.OrgValues["otx"] = "kept"

	for _, dryRun := range []bool{true, false} {
		ops, err := org.SyncPush(orgValuesConf(t, "org-value:\n  otx: kept\n"), SyncOptions{SyncOrgValues: true, IsForce: true, IsDryRun: dryRun})
		require.NoError(t, err)
		require.Equal(t, sortSyncOps([]OrgSyncOperation{
			{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "otx"},
			{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "twilio", IsRemoved: true},
		}), sortSyncOps(ops), "dry run %v", dryRun)
		require.Equal(t, "true", ms.OrgValues["sensor_auto_upgrade"])
	}
	require.Equal(t, "", ms.OrgValues["twilio"])

	// Naming it is how a template turns it off.
	ops, err := org.SyncPush(orgValuesConf(t, "org-value:\n  otx: kept\n  sensor_auto_upgrade: \"false\"\n"), SyncOptions{SyncOrgValues: true, IsForce: true})
	require.NoError(t, err)
	require.Equal(t, sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "otx"},
		{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "sensor_auto_upgrade", IsAdded: true},
	}), sortSyncOps(ops))
	require.Equal(t, "false", ms.OrgValues["sensor_auto_upgrade"])
}

// An API that does not know the value yet (it answers the read with an error)
// is not a failure: the value is left out of a fetch and a push goes on.
func TestSyncSensorAutoUpgradeUnknownToAPI(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	ms.OrgValues["otx"] = "kept"

	fetched, err := org.SyncFetch(SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.NotContains(t, fetched.OrgValues, "sensor_auto_upgrade")
	require.Equal(t, "kept", fetched.OrgValues["otx"])

	_, err = org.SyncPush(orgValuesConf(t, "org-value:\n  otx: kept\n"), SyncOptions{SyncOrgValues: true, IsForce: true})
	require.NoError(t, err)
	require.NotContains(t, ms.OrgValues, "sensor_auto_upgrade")
}

// A push that fails on one value says which, and stops at the same place
// every time: the values before it in name order are written, the ones after
// are not.
func TestSyncOrgValueFailureNamesTheValue(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	// The key lacks module.update: only sensor_auto_upgrade is refused.
	ms.CustomHandlers["/v1/configs/"+ms.OID+"/sensor_auto_upgrade"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized, missing permission: module.update"}`))
	}

	conf := orgValuesConf(t, "org-value:\n  otx: a\n  sensor_auto_upgrade: \"true\"\n  twilio: b\n")
	_, err = org.SyncPush(conf, SyncOptions{SyncOrgValues: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sensor_auto_upgrade")
	require.Equal(t, "a", ms.OrgValues["otx"], "written before the failure")
	require.NotContains(t, ms.OrgValues, "twilio", "not reached")
}
