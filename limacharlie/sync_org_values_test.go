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

// retention_months is pushed, fetched and compared like any other org value.
func TestSyncRetentionMonthsOrgValue(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)

	// Dry run reports the change without making it.
	ops, err := org.SyncPush(orgValuesConf(t, "org-value:\n  retention_months: \"24\"\n"), SyncOptions{SyncOrgValues: true, IsDryRun: true})
	require.NoError(t, err)
	require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "retention_months", IsAdded: true}}, ops)
	require.NotContains(t, ms.OrgValues, "retention_months")

	ops, err = org.SyncPush(orgValuesConf(t, "org-value:\n  retention_months: \"24\"\n"), SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "retention_months", IsAdded: true}}, ops)
	require.Equal(t, "24", ms.OrgValues["retention_months"])

	fetched, err := org.SyncFetch(SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, "24", fetched.OrgValues["retention_months"])

	// The same push again changes nothing; a different value is a change.
	ops, err = org.SyncPush(orgValuesConf(t, "org-value:\n  retention_months: \"24\"\n"), SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "retention_months"}}, ops)

	ops, err = org.SyncPush(orgValuesConf(t, "org-value:\n  retention_months: \"12\"\n"), SyncOptions{SyncOrgValues: true})
	require.NoError(t, err)
	require.Equal(t, []OrgSyncOperation{{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "retention_months", IsAdded: true}}, ops)
	require.Equal(t, "12", ms.OrgValues["retention_months"])
}

// A forced push never clears retention_months (the API accepts only 12, 18, 24
// and 36, so an empty value would be refused, and a template that does not
// name it must not shorten an org's retention). Naming it changes it.
func TestSyncForceKeepsRetentionMonths(t *testing.T) {
	ms := NewMockServer("00000000-0000-0000-0000-000000000001")
	defer ms.Close()
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	ms.OrgValues["retention_months"] = "36"
	ms.OrgValues["twilio"] = "to-be-removed"

	for _, dryRun := range []bool{true, false} {
		ops, err := org.SyncPush(orgValuesConf(t, "org-value:\n  otx: x\n"), SyncOptions{SyncOrgValues: true, IsForce: true, IsDryRun: dryRun})
		require.NoError(t, err)
		require.Equal(t, sortSyncOps([]OrgSyncOperation{
			{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "otx", IsAdded: true},
			{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "twilio", IsRemoved: true},
		}), sortSyncOps(ops), "dry run %v", dryRun)
		require.Equal(t, "36", ms.OrgValues["retention_months"], "dry run %v", dryRun)
		ms.OrgValues["twilio"] = "to-be-removed"
	}

	_, err = org.SyncPush(orgValuesConf(t, "org-value:\n  retention_months: \"12\"\n"), SyncOptions{SyncOrgValues: true, IsForce: true})
	require.NoError(t, err)
	require.Equal(t, "12", ms.OrgValues["retention_months"])
}

// Raising retention above 12 can be refused (free plan, or a key without
// billing.ctrl). The push fails, names the value and carries the API's reason
// instead of reporting success.
func TestSyncRetentionMonthsRefusedByAPI(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		reason string
	}{
		{"missing permission", http.StatusUnauthorized, `{"error":"unauthorized, missing permission: billing.ctrl"}`, "billing.ctrl"},
		{"free tier", http.StatusForbidden, `{"error":"extended retention requires a paid plan"}`, "requires a paid plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ms := NewMockServer("00000000-0000-0000-0000-000000000001")
			defer ms.Close()
			org, err := ms.NewOrganization()
			require.NoError(t, err)
			ms.CustomHandlers["/v1/configs/"+ms.OID+"/retention_months"] = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}

			ops, err := org.SyncPush(orgValuesConf(t, "org-value:\n  retention_months: \"24\"\n"), SyncOptions{SyncOrgValues: true})
			require.Error(t, err)
			require.Contains(t, err.Error(), "retention_months")
			require.Contains(t, err.Error(), tc.reason)
			require.Empty(t, ops, "a refused value is not reported as applied")
		})
	}
}
