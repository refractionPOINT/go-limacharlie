package limacharlie

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOID = "00000000-0000-0000-0000-000000000001"

func setupMock(t *testing.T) (*MockServer, *Organization) {
	t.Helper()
	ms := NewMockServer(testOID)
	t.Cleanup(ms.Close)
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	return ms, org
}

// --- Client & Organization basics ---

func TestMockNewClient(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	c, err := ms.NewClient()
	require.NoError(t, err)
	assert.Equal(t, testOID, c.options.OID)
	assert.NotEmpty(t, c.options.JWT)
	assert.NotEmpty(t, c.options.APIKey)
}

func TestMockNewOrganization(t *testing.T) {
	ms, org := setupMock(t)
	assert.Equal(t, ms.OID, org.GetOID())
}

func TestMockWhoAmI(t *testing.T) {
	ms, org := setupMock(t)

	// Default WhoAmI
	who, err := org.WhoAmI()
	require.NoError(t, err)
	assert.Contains(t, *who.Organizations, ms.OID)
	assert.NotNil(t, who.Identity)

	// Custom WhoAmI
	customOrgs := []string{"org-1", "org-2"}
	customPerms := []string{"perm-a"}
	ms.WhoAmIResponse = &WhoAmIJsonResponse{
		Organizations: &customOrgs,
		Permissions:   &customPerms,
	}
	who, err = org.WhoAmI()
	require.NoError(t, err)
	assert.Equal(t, customOrgs, *who.Organizations)
	assert.Equal(t, customPerms, *who.Permissions)
}

func TestMockGetInfo(t *testing.T) {
	ms, org := setupMock(t)

	ms.OrgInfo = OrganizationInformation{
		OID:  testOID,
		Name: "Test Org",
	}

	info, err := org.GetInfo()
	require.NoError(t, err)
	assert.Equal(t, "Test Org", info.Name)
	assert.Equal(t, testOID, info.OID)
}

func TestMockGetURLs(t *testing.T) {
	_, org := setupMock(t)

	urls, err := org.GetURLs()
	require.NoError(t, err)
	assert.NotEmpty(t, urls["lc"])
	assert.NotEmpty(t, urls["replay"])
}

func TestMockGetSiteConnectivityInfo(t *testing.T) {
	_, org := setupMock(t)

	info, err := org.GetSiteConnectivityInfo()
	require.NoError(t, err)
	assert.Equal(t, "mock", info.SiteName)
	assert.NotEmpty(t, info.URLs.Lc)
}

func TestMockGetOnlineCount(t *testing.T) {
	ms, org := setupMock(t)

	sid1 := uuid.New().String()
	sid2 := uuid.New().String()
	ms.SensorOnline[sid1] = true
	ms.SensorOnline[sid2] = false

	count, err := org.GetOnlineCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count.Count)
}

func TestMockSetQuota(t *testing.T) {
	_, org := setupMock(t)

	ok, err := org.SetQuota(100)
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- D&R Rules ---

func TestMockDRRules_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Empty(t, rules)

	// Add a rule
	detect := Dict{"event": "NEW_PROCESS", "op": "is", "path": "event/FILE_PATH", "value": "evil.exe"}
	respond := List{Dict{"action": "report", "name": "evil-detected"}}
	err = org.DRRuleAdd("test-rule", detect, respond)
	require.NoError(t, err)

	// List rules
	rules, err = org.DRRules()
	require.NoError(t, err)
	assert.Contains(t, rules, "test-rule")

	// Add another rule
	err = org.DRRuleAdd("test-rule-2", Dict{"event": "DNS_REQUEST"}, List{})
	require.NoError(t, err)

	rules, err = org.DRRules()
	require.NoError(t, err)
	assert.Len(t, rules, 2)

	// Delete rule
	err = org.DRRuleDelete("test-rule")
	require.NoError(t, err)

	rules, err = org.DRRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Contains(t, rules, "test-rule-2")
}

func TestMockDRRules_WithOptions(t *testing.T) {
	_, org := setupMock(t)

	detect := Dict{"event": "NEW_PROCESS"}
	respond := List{Dict{"action": "report", "name": "test"}}
	err := org.DRRuleAdd("ns-rule", detect, respond, NewDRRuleOptions{
		IsReplace: true,
		Namespace: "managed",
		IsEnabled: true,
	})
	require.NoError(t, err)

	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Contains(t, rules, "ns-rule")
}

// --- FP Rules ---

func TestMockFPRules_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	rules, err := org.FPRules()
	require.NoError(t, err)
	assert.Empty(t, rules)

	// Add
	err = org.FPRuleAdd("fp-1", Dict{"op": "is", "path": "event/FILE_PATH", "value": "safe.exe"})
	require.NoError(t, err)

	rules, err = org.FPRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Contains(t, rules, "fp-1")
	assert.Equal(t, testOID, rules["fp-1"].OID)

	// Add another
	err = org.FPRuleAdd("fp-2", Dict{"op": "contains", "path": "event/DOMAIN_NAME", "value": "safe.com"})
	require.NoError(t, err)

	rules, err = org.FPRules()
	require.NoError(t, err)
	assert.Len(t, rules, 2)

	// Delete
	err = org.FPRuleDelete("fp-1")
	require.NoError(t, err)

	rules, err = org.FPRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Contains(t, rules, "fp-2")
}

func TestMockFPRules_Replace(t *testing.T) {
	_, org := setupMock(t)

	err := org.FPRuleAdd("fp-replace", Dict{"value": "v1"})
	require.NoError(t, err)

	err = org.FPRuleAdd("fp-replace", Dict{"value": "v2"}, FPRuleOptions{IsReplace: true})
	require.NoError(t, err)

	rules, err := org.FPRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
}

// --- Installation Keys ---

func TestMockInstallationKeys_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	keys, err := org.InstallationKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)

	// Add
	iid, err := org.AddInstallationKey(InstallationKey{
		Description: "test key",
		Tags:        []string{"tag1", "tag2"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, iid)

	// List
	keys, err = org.InstallationKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "test key", keys[0].Description)

	// Get specific
	k, err := org.InstallationKey(iid)
	require.NoError(t, err)
	assert.Equal(t, iid, k.ID)
	assert.Equal(t, "test key", k.Description)

	// Delete
	err = org.DelInstallationKey(iid)
	require.NoError(t, err)

	keys, err = org.InstallationKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestMockInstallationKeys_WithCustomIID(t *testing.T) {
	_, org := setupMock(t)

	customIID := uuid.New().String()
	iid, err := org.AddInstallationKey(InstallationKey{
		ID:          customIID,
		Description: "custom",
	})
	require.NoError(t, err)
	assert.Equal(t, customIID, iid)

	k, err := org.InstallationKey(customIID)
	require.NoError(t, err)
	assert.Equal(t, "custom", k.Description)
}

// --- Outputs ---

func TestMockOutputs_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	outputs, err := org.Outputs()
	require.NoError(t, err)
	assert.Empty(t, outputs)

	// Add
	_, err = org.OutputAdd(OutputConfig{
		Name:   "test-output",
		Module: OutputTypes.Syslog,
		Type:   OutputType.Event,
	})
	require.NoError(t, err)

	// List
	outputs, err = org.Outputs()
	require.NoError(t, err)
	assert.Len(t, outputs, 1)
	assert.Contains(t, outputs, "test-output")

	// Add another
	_, err = org.OutputAdd(OutputConfig{
		Name:   "s3-output",
		Module: OutputTypes.S3,
		Type:   OutputType.Detect,
	})
	require.NoError(t, err)

	outputs, err = org.Outputs()
	require.NoError(t, err)
	assert.Len(t, outputs, 2)

	// Delete
	_, err = org.OutputDel("test-output")
	require.NoError(t, err)

	outputs, err = org.Outputs()
	require.NoError(t, err)
	assert.Len(t, outputs, 1)
	assert.Contains(t, outputs, "s3-output")
}

// --- Sensors ---

func TestMockSensors_ListAndGet(t *testing.T) {
	ms, org := setupMock(t)

	sid1 := uuid.New().String()
	sid2 := uuid.New().String()
	ms.SensorStore[sid1] = &Sensor{
		OID:      testOID,
		SID:      sid1,
		Hostname: "host-1",
		Platform: Platforms.Windows,
	}
	ms.SensorStore[sid2] = &Sensor{
		OID:      testOID,
		SID:      sid2,
		Hostname: "host-2",
		Platform: Platforms.Linux,
	}

	sensors, err := org.ListSensors()
	require.NoError(t, err)
	assert.Len(t, sensors, 2)
	assert.Contains(t, sensors, sid1)
	assert.Contains(t, sensors, sid2)
	assert.Equal(t, "host-1", sensors[sid1].Hostname)
	assert.Equal(t, "host-2", sensors[sid2].Hostname)
}

func TestMockSensors_Get(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{
		OID:        testOID,
		SID:        sid,
		Hostname:   "test-host",
		InternalIP: "10.0.0.1",
		ExternalIP: "1.2.3.4",
		Platform:   Platforms.Windows,
	}

	s := org.GetSensor(sid)
	assert.Equal(t, "test-host", s.Hostname)
	assert.Equal(t, "10.0.0.1", s.InternalIP)
	assert.Equal(t, "1.2.3.4", s.ExternalIP)
}

func TestMockSensors_Tags(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{OID: testOID, SID: sid, Hostname: "tagged-host"}

	s := org.GetSensor(sid)

	// Initially no tags
	tags, err := s.GetTags()
	require.NoError(t, err)
	assert.Empty(t, tags)

	// Add tags
	err = s.AddTag("important", 0)
	require.NoError(t, err)

	err = s.AddTag("monitored", 24*time.Hour)
	require.NoError(t, err)

	tags, err = s.GetTags()
	require.NoError(t, err)
	assert.Len(t, tags, 2)

	tagNames := make([]string, len(tags))
	for i, ti := range tags {
		tagNames[i] = ti.Tag
	}
	assert.Contains(t, tagNames, "important")
	assert.Contains(t, tagNames, "monitored")

	// Remove a tag
	err = s.RemoveTag("important")
	require.NoError(t, err)

	tags, err = s.GetTags()
	require.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "monitored", tags[0].Tag)
}

func TestMockSensors_Isolation(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{OID: testOID, SID: sid}

	s := org.GetSensor(sid)

	assert.False(t, ms.SensorIsolation[sid])

	err := s.IsolateFromNetwork()
	require.NoError(t, err)
	assert.True(t, ms.SensorIsolation[sid])

	err = s.RejoinNetwork()
	require.NoError(t, err)
	assert.False(t, ms.SensorIsolation[sid])
}

func TestMockSensors_Delete(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{OID: testOID, SID: sid}

	s := org.GetSensor(sid)
	err := s.Delete()
	require.NoError(t, err)

	assert.NotContains(t, ms.SensorStore, sid)
}

func TestMockSensors_Task(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{OID: testOID, SID: sid}

	s := org.GetSensor(sid)
	err := s.Task("os_version")
	require.NoError(t, err)

	// Verify call was recorded
	calls := ms.Calls()
	found := false
	for _, call := range calls {
		if call.Method == "POST" && call.Path == "/v1/"+sid {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a POST to /v1/%s for tasking", sid)
}

func TestMockSensors_ActiveSensors(t *testing.T) {
	ms, org := setupMock(t)

	sid1 := uuid.New().String()
	sid2 := uuid.New().String()
	ms.SensorOnline[sid1] = true
	ms.SensorOnline[sid2] = false

	result, err := org.ActiveSensors([]string{sid1, sid2})
	require.NoError(t, err)
	assert.True(t, result[sid1])
	assert.False(t, result[sid2])
}

func TestMockSensors_GetAllTags(t *testing.T) {
	ms, org := setupMock(t)

	ms.AllTags = []string{"tag1", "tag2", "tag3"}

	tags, err := org.GetAllTags()
	require.NoError(t, err)
	assert.Equal(t, []string{"tag1", "tag2", "tag3"}, tags)
}

func TestMockSensors_GetSensorsWithTag(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorTags[sid] = map[string]TagInfo{
		"important": {Tag: "important", By: "mock"},
	}

	result, err := org.GetSensorsWithTag("important")
	require.NoError(t, err)
	assert.Contains(t, result, sid)
}

// --- Users ---

func TestMockUsers_CRUD(t *testing.T) {
	ms, org := setupMock(t)

	// Pre-populate
	ms.UserEmails = []string{"existing@test.com"}

	users, err := org.GetUsers()
	require.NoError(t, err)
	assert.Equal(t, []string{"existing@test.com"}, users)

	// Add user
	resp, err := org.AddUser("new@test.com", false, UserRoleOperator)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	users, err = org.GetUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Contains(t, users, "new@test.com")

	// Remove user
	err = org.RemoveUser("existing@test.com")
	require.NoError(t, err)

	users, err = org.GetUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "new@test.com", users[0])
}

func TestMockUsers_Permissions(t *testing.T) {
	ms, org := setupMock(t)

	ms.UserEmails = []string{"user@test.com"}

	// Add permission
	err := org.AddUserPermission("user@test.com", "dr.list")
	require.NoError(t, err)

	err = org.AddUserPermission("user@test.com", "sensor.task")
	require.NoError(t, err)

	// Get permissions
	perms, err := org.GetUsersPermissions()
	require.NoError(t, err)
	assert.Contains(t, perms.UserPermissions["user@test.com"], "dr.list")
	assert.Contains(t, perms.UserPermissions["user@test.com"], "sensor.task")

	// Remove permission
	err = org.RemoveUserPermission("user@test.com", "dr.list")
	require.NoError(t, err)

	perms, err = org.GetUsersPermissions()
	require.NoError(t, err)
	assert.NotContains(t, perms.UserPermissions["user@test.com"], "dr.list")
	assert.Contains(t, perms.UserPermissions["user@test.com"], "sensor.task")
}

func TestMockUsers_SetRole(t *testing.T) {
	ms, org := setupMock(t)

	ms.UserEmails = []string{"user@test.com"}

	resp, err := org.SetUserRole("user@test.com", UserRoleAdministrator)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, UserRoleAdministrator, resp.Role)
	assert.Equal(t, UserRoleAdministrator, ms.UserRoles["user@test.com"])
}

// --- Resources ---

func TestMockResources_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	res, err := org.Resources()
	require.NoError(t, err)
	assert.Empty(t, res)

	// Subscribe
	err = org.ResourceSubscribe("lookup", ResourceCategories.API)
	require.NoError(t, err)

	err = org.ResourceSubscribe("exfil", ResourceCategories.Replicant)
	require.NoError(t, err)

	res, err = org.Resources()
	require.NoError(t, err)
	assert.Contains(t, res[ResourceCategories.API], "lookup")
	assert.Contains(t, res[ResourceCategories.Replicant], "exfil")

	// Unsubscribe
	err = org.ResourceUnsubscribe("lookup", ResourceCategories.API)
	require.NoError(t, err)

	res, err = org.Resources()
	require.NoError(t, err)
	assert.NotContains(t, res[ResourceCategories.API], "lookup")
	assert.Contains(t, res[ResourceCategories.Replicant], "exfil")
}

// --- Ingestion Keys ---

func TestMockIngestionKeys_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	keys, err := org.GetIngestionKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)

	// Create
	resp, err := org.SetIngestionKeys("my-key")
	require.NoError(t, err)
	assert.NotEmpty(t, resp)

	// List
	keys, err = org.GetIngestionKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// Delete
	_, err = org.DelIngestionKeys("my-key")
	require.NoError(t, err)

	keys, err = org.GetIngestionKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

// --- Payloads ---

func TestMockPayloads_ListAndDelete(t *testing.T) {
	ms, org := setupMock(t)

	// Pre-populate
	ms.PayloadStore["payload-1"] = Payload{
		Name: "payload-1",
		Oid:  testOID,
		Size: 1024,
	}
	ms.PayloadStore["payload-2"] = Payload{
		Name: "payload-2",
		Oid:  testOID,
		Size: 2048,
	}

	payloads, err := org.Payloads()
	require.NoError(t, err)
	assert.Len(t, payloads, 2)
	assert.Contains(t, payloads, "payload-1")
	assert.Contains(t, payloads, "payload-2")

	// Delete
	err = org.DeletePayload("payload-1")
	require.NoError(t, err)

	payloads, err = org.Payloads()
	require.NoError(t, err)
	assert.Len(t, payloads, 1)
	assert.NotContains(t, payloads, "payload-1")
}

// --- Extensions ---

func TestMockExtensions_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	exts, err := org.Extensions()
	require.NoError(t, err)
	assert.Empty(t, exts)

	// Subscribe
	err = org.SubscribeToExtension("ext-1")
	require.NoError(t, err)

	err = org.SubscribeToExtension("ext-2")
	require.NoError(t, err)

	exts, err = org.Extensions()
	require.NoError(t, err)
	assert.Len(t, exts, 2)
	assert.Contains(t, exts, ExtensionName("ext-1"))
	assert.Contains(t, exts, ExtensionName("ext-2"))

	// Unsubscribe
	err = org.UnsubscribeFromExtension("ext-1")
	require.NoError(t, err)

	exts, err = org.Extensions()
	require.NoError(t, err)
	assert.Len(t, exts, 1)
	assert.Contains(t, exts, ExtensionName("ext-2"))

	// ReKey (just verify it doesn't error)
	err = org.ReKeyExtension("ext-2")
	require.NoError(t, err)
}

func TestMockExtensions_Schema(t *testing.T) {
	_, org := setupMock(t)

	schema, err := org.GetExtensionSchema("some-ext")
	require.NoError(t, err)
	assert.NotNil(t, schema)
}

// --- Hive ---

func TestMockHive_CRUD(t *testing.T) {
	_, org := setupMock(t)

	hc := NewHiveClient(org)

	args := HiveArgs{
		HiveName:     "ip-rep",
		PartitionKey: testOID,
		Key:          "my-record",
	}

	// List - initially empty
	records, err := hc.List(HiveArgs{HiveName: "ip-rep", PartitionKey: testOID})
	require.NoError(t, err)
	assert.Empty(t, records)

	// Add a record
	enabled := true
	expiry := int64(0)
	args.Data = Dict{"reputation": "malicious", "score": 95}
	args.Enabled = &enabled
	args.Expiry = &expiry
	resp, err := hc.Add(args)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Guid)
	assert.Equal(t, "my-record", resp.Name)

	// Get the record
	hd, err := hc.Get(HiveArgs{HiveName: "ip-rep", PartitionKey: testOID, Key: "my-record"})
	require.NoError(t, err)
	assert.Equal(t, "malicious", hd.Data["reputation"])
	assert.True(t, hd.UsrMtd.Enabled)
	assert.NotEmpty(t, hd.SysMtd.Etag)

	// Get metadata only
	hdMtd, err := hc.GetMTD(HiveArgs{HiveName: "ip-rep", PartitionKey: testOID, Key: "my-record"})
	require.NoError(t, err)
	assert.Nil(t, hdMtd.Data)
	assert.True(t, hdMtd.UsrMtd.Enabled)

	// List again
	records, err = hc.List(HiveArgs{HiveName: "ip-rep", PartitionKey: testOID})
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// Remove
	_, err = hc.Remove(HiveArgs{HiveName: "ip-rep", PartitionKey: testOID, Key: "my-record"})
	require.NoError(t, err)

	records, err = hc.List(HiveArgs{HiveName: "ip-rep", PartitionKey: testOID})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestMockHive_MultipleRecords(t *testing.T) {
	_, org := setupMock(t)

	hc := NewHiveClient(org)
	enabled := true
	expiry := int64(0)

	for _, key := range []string{"rec-1", "rec-2", "rec-3"} {
		_, err := hc.Add(HiveArgs{
			HiveName:     "test-hive",
			PartitionKey: testOID,
			Key:          key,
			Data:         Dict{"key": key},
			Enabled:      &enabled,
			Expiry:       &expiry,
		})
		require.NoError(t, err)
	}

	records, err := hc.List(HiveArgs{HiveName: "test-hive", PartitionKey: testOID})
	require.NoError(t, err)
	assert.Len(t, records, 3)
}

func TestMockHive_Rename(t *testing.T) {
	_, org := setupMock(t)

	hc := NewHiveClient(org)
	enabled := true
	expiry := int64(0)

	_, err := hc.Add(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "old-name",
		Data:         Dict{"value": "test"},
		Enabled:      &enabled,
		Expiry:       &expiry,
	})
	require.NoError(t, err)

	resp, err := hc.Rename(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "old-name",
	}, "new-name")
	require.NoError(t, err)
	assert.Equal(t, "new-name", resp.Name)

	// Old key should be gone
	_, err = hc.Get(HiveArgs{HiveName: "test-hive", PartitionKey: testOID, Key: "old-name"})
	assert.Error(t, err)

	// New key should exist
	hd, err := hc.Get(HiveArgs{HiveName: "test-hive", PartitionKey: testOID, Key: "new-name"})
	require.NoError(t, err)
	assert.Equal(t, "test", hd.Data["value"])
}

// --- Exfil Rules ---

func TestMockExfilRules_Events(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	rules, err := org.ExfilRules()
	require.NoError(t, err)
	assert.Empty(t, rules.Events)

	// Add event rule
	err = org.ExfilRuleEventAdd("exfil-1", ExfilRuleEvent{
		Events: []string{"NEW_PROCESS", "DNS_REQUEST"},
		Filters: ExfilEventFilters{
			Tags:      []string{"vip"},
			Platforms: []string{"windows"},
		},
	})
	require.NoError(t, err)

	rules, err = org.ExfilRules()
	require.NoError(t, err)
	assert.Len(t, rules.Events, 1)
	assert.Contains(t, rules.Events, ExfilRuleName("exfil-1"))
	assert.Equal(t, []string{"NEW_PROCESS", "DNS_REQUEST"}, rules.Events["exfil-1"].Events)

	// Delete event rule
	err = org.ExfilRuleEventDelete("exfil-1")
	require.NoError(t, err)

	rules, err = org.ExfilRules()
	require.NoError(t, err)
	assert.Empty(t, rules.Events)
}

func TestMockExfilRules_Watches(t *testing.T) {
	_, org := setupMock(t)

	// Add watch rule
	err := org.ExfilRuleWatchAdd("watch-1", ExfilRuleWatch{
		Event:    "NEW_PROCESS",
		Value:    "evil.exe",
		Path:     []string{"event", "FILE_PATH"},
		Operator: "contains",
		Filters: ExfilEventFilters{
			Tags:      []string{},
			Platforms: []string{},
		},
	})
	require.NoError(t, err)

	rules, err := org.ExfilRules()
	require.NoError(t, err)
	assert.Len(t, rules.Watches, 1)
	assert.Equal(t, "evil.exe", rules.Watches["watch-1"].Value)
	assert.Equal(t, "contains", rules.Watches["watch-1"].Operator)

	// Delete
	err = org.ExfilRuleWatchDelete("watch-1")
	require.NoError(t, err)

	rules, err = org.ExfilRules()
	require.NoError(t, err)
	assert.Empty(t, rules.Watches)
}

// --- Artifact Rules ---

func TestMockArtifactRules_CRUD(t *testing.T) {
	_, org := setupMock(t)

	// Initially empty
	rules, err := org.ArtifactsRules()
	require.NoError(t, err)
	assert.Empty(t, rules)

	// Add
	err = org.ArtifactRuleAdd("logs-rule", ArtifactRule{
		Patterns:       []string{"/var/log/**"},
		DaysRetentions: 30,
		IsDeleteAfter:  true,
		Filters: ArtifactRuleFilter{
			Tags:      []string{"servers"},
			Platforms: []string{"linux"},
		},
	})
	require.NoError(t, err)

	rules, err = org.ArtifactsRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Contains(t, rules, ArtifactRuleName("logs-rule"))
	assert.Equal(t, []string{"/var/log/**"}, rules["logs-rule"].Patterns)
	assert.True(t, rules["logs-rule"].IsDeleteAfter)

	// Delete
	err = org.ArtifactRuleDelete("logs-rule")
	require.NoError(t, err)

	rules, err = org.ArtifactsRules()
	require.NoError(t, err)
	assert.Empty(t, rules)
}

// --- Org Values ---

func TestMockOrgValues_GetSet(t *testing.T) {
	ms, org := setupMock(t)

	// Pre-populate
	ms.OrgValues["my-config"] = "initial-value"

	// Get
	val, err := org.OrgValueGet("my-config")
	require.NoError(t, err)
	assert.Equal(t, "my-config", val.Name)
	assert.Equal(t, "initial-value", val.Value)

	// Set
	err = org.OrgValueSet("my-config", "updated-value")
	require.NoError(t, err)

	val, err = org.OrgValueGet("my-config")
	require.NoError(t, err)
	assert.Equal(t, "updated-value", val.Value)

	// Set new key
	err = org.OrgValueSet("new-config", "new-value")
	require.NoError(t, err)

	val, err = org.OrgValueGet("new-config")
	require.NoError(t, err)
	assert.Equal(t, "new-value", val.Value)
}

// --- Insight Objects ---

func TestMockInsightObjects_Summary(t *testing.T) {
	ms, org := setupMock(t)

	// Pre-populate
	ms.IOCSummaries["domain/evil.com"] = IOCSummaryResponse{
		Type:      InsightObjectTypes.Domain,
		Name:      "evil.com",
		FromCache: false,
	}

	resp, err := org.SearchIOCSummary(IOCSearchParams{
		SearchTerm: "evil.com",
		ObjectType: InsightObjectTypes.Domain,
	})
	require.NoError(t, err)
	assert.Equal(t, "evil.com", resp.Name)
	assert.Equal(t, InsightObjectTypes.Domain, resp.Type)
}

func TestMockInsightObjects_SummaryMiss(t *testing.T) {
	_, org := setupMock(t)

	// Search for something not pre-populated
	resp, err := org.SearchIOCSummary(IOCSearchParams{
		SearchTerm: "unknown.com",
		ObjectType: InsightObjectTypes.Domain,
	})
	require.NoError(t, err)
	assert.Equal(t, "unknown.com", resp.Name)
}

// --- Hostname Search ---

func TestMockHostnameSearch(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.HostnameResults["web-server"] = []HostnameSearchResult{
		{SID: sid, Hostname: "web-server"},
	}

	results, err := org.SearchHostname("web-server")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, sid, results[0].SID)
}

func TestMockHostnameSearch_NoResults(t *testing.T) {
	_, org := setupMock(t)

	results, err := org.SearchHostname("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- Billing ---

func TestMockBilling_Status(t *testing.T) {
	ms, org := setupMock(t)

	ms.BillingStatus = &BillingOrgStatus{IsPastDue: true}

	status, err := org.GetBillingOrgStatus()
	require.NoError(t, err)
	assert.True(t, status.IsPastDue)
}

func TestMockBilling_Details(t *testing.T) {
	ms, org := setupMock(t)

	ms.BillingDetails = &BillingOrgDetails{
		Status: map[string]interface{}{"is_past_due": false},
	}

	details, err := org.GetBillingOrgDetails()
	require.NoError(t, err)
	assert.NotNil(t, details.Status)
}

func TestMockBilling_Plans(t *testing.T) {
	ms, org := setupMock(t)

	ms.BillingPlans = []BillingPlan{
		{ID: "plan-1", Name: "Pro", Price: 9.99},
		{ID: "plan-2", Name: "Enterprise", Price: 49.99},
	}

	plans, err := org.GetBillingAvailablePlans()
	require.NoError(t, err)
	require.Len(t, plans, 2)
	assert.Equal(t, "Pro", plans[0].Name)
}

func TestMockBilling_Invoice(t *testing.T) {
	_, org := setupMock(t)

	resp, err := org.GetBillingInvoiceURL(2025, 6, "")
	require.NoError(t, err)
	assert.NotEmpty(t, resp["url"])
}

func TestMockBilling_Auth(t *testing.T) {
	_, org := setupMock(t)

	auth, err := org.GetBillingUserAuthRequirements()
	require.NoError(t, err)
	assert.NotNil(t, auth)
}

// --- Groups ---

func TestMockGroups_CreateAndList(t *testing.T) {
	_, _ = setupMock(t)

	ms := NewMockServer(testOID)
	defer ms.Close()

	c, err := ms.NewClient()
	require.NoError(t, err)

	// Create
	resp, err := c.CreateGroup("my-group")
	require.NoError(t, err)
	assert.True(t, resp.Success)
	gid := resp.Data.GID
	assert.NotEmpty(t, gid)

	// List
	groups, err := c.GetGroups()
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "my-group", groups[0].Name)

	// Get concurrent
	concurrent, err := c.GetGroupsConcurrent()
	require.NoError(t, err)
	assert.Len(t, concurrent, 1)
}

func TestMockGroups_MembersAndOwners(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	c, err := ms.NewClient()
	require.NoError(t, err)

	// Create group
	resp, err := c.CreateGroup("test-group")
	require.NoError(t, err)
	gid := resp.Data.GID

	g := c.GetGroup(gid)

	// Add member
	err = g.AddMember("member@test.com", false)
	require.NoError(t, err)

	// Add owner
	err = g.AddOwner("owner@test.com", false)
	require.NoError(t, err)

	// Verify
	info, err := g.GetInfo()
	require.NoError(t, err)
	assert.Contains(t, info.Members, "member@test.com")
	assert.Contains(t, info.Owners, "owner@test.com")

	// Remove member
	err = g.RemoveMember("member@test.com")
	require.NoError(t, err)

	info, err = g.GetInfo()
	require.NoError(t, err)
	assert.NotContains(t, info.Members, "member@test.com")

	// Delete group
	err = g.Delete()
	require.NoError(t, err)

	groups, err := c.GetGroups()
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestMockGroups_OrgManagement(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	c, err := ms.NewClient()
	require.NoError(t, err)

	resp, err := c.CreateGroup("org-group")
	require.NoError(t, err)
	gid := resp.Data.GID

	g := c.GetGroup(gid)

	// Add org
	err = g.AddOrg("org-123")
	require.NoError(t, err)

	info, err := g.GetInfo()
	require.NoError(t, err)
	assert.Len(t, info.Orgs, 1)
	assert.Equal(t, "org-123", info.Orgs[0].OrgID)

	// Remove org
	err = g.RemoveOrg("org-123")
	require.NoError(t, err)

	info, err = g.GetInfo()
	require.NoError(t, err)
	assert.Empty(t, info.Orgs)
}

func TestMockGroups_AddToGroup(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	c, err := ms.NewClient()
	require.NoError(t, err)

	org, err := NewOrganization(c)
	require.NoError(t, err)

	resp, err := c.CreateGroup("add-to-group")
	require.NoError(t, err)
	gid := resp.Data.GID

	ok, err := org.AddToGroup(gid)
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- Call Tracking ---

func TestMockCallTracking(t *testing.T) {
	ms, org := setupMock(t)

	ms.ResetCalls()
	assert.Empty(t, ms.Calls())

	_, _ = org.GetInfo()
	_, _ = org.DRRules()

	calls := ms.Calls()
	assert.Len(t, calls, 2)

	assert.Equal(t, "GET", calls[0].Method)
	assert.Contains(t, calls[0].Path, "orgs")
	assert.Equal(t, "GET", calls[1].Method)
	assert.Contains(t, calls[1].Path, "rules")
}

func TestMockCallTrackingReset(t *testing.T) {
	ms, org := setupMock(t)

	_, _ = org.GetInfo()
	assert.NotEmpty(t, ms.Calls())

	ms.ResetCalls()
	assert.Empty(t, ms.Calls())
}

// --- Authorize ---

func TestMockAuthorize(t *testing.T) {
	_, org := setupMock(t)

	// Should succeed with default perms
	ident, perms, err := org.Authorize([]string{"org.get"})
	require.NoError(t, err)
	assert.NotEmpty(t, ident)
	assert.NotEmpty(t, perms)

	// Should fail with missing perms
	_, _, err = org.Authorize([]string{"nonexistent.permission"})
	assert.Error(t, err)
}

// --- Concurrent access ---

func TestMockConcurrentAccess(t *testing.T) {
	_, org := setupMock(t)

	done := make(chan bool, 10)

	// Concurrent D&R rule adds
	for i := 0; i < 10; i++ {
		go func(n int) {
			name := fmt.Sprintf("rule-%d", n)
			_ = org.DRRuleAdd(name, Dict{"event": "TEST"}, List{})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Len(t, rules, 10)
}

// --- Integration: full workflow ---

func TestMockIntegration_FullWorkflow(t *testing.T) {
	ms, org := setupMock(t)

	// 1. Check org info
	info, err := org.GetInfo()
	require.NoError(t, err)
	assert.Equal(t, ms.OID, info.OID)

	// 2. Add a detection rule
	err = org.DRRuleAdd("detect-evil", Dict{
		"event": "NEW_PROCESS",
		"op":    "contains",
		"path":  "event/FILE_PATH",
		"value": "evil.exe",
	}, List{
		Dict{"action": "report", "name": "evil-process"},
	})
	require.NoError(t, err)

	// 3. Add a false positive rule
	err = org.FPRuleAdd("fp-safe", Dict{
		"op":    "is",
		"path":  "event/FILE_PATH",
		"value": "safe.exe",
	})
	require.NoError(t, err)

	// 4. Add an installation key
	iid, err := org.AddInstallationKey(InstallationKey{
		Description: "production key",
		Tags:        []string{"production"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, iid)

	// 5. Add output
	_, err = org.OutputAdd(OutputConfig{
		Name:   "siem-output",
		Module: OutputTypes.Syslog,
		Type:   OutputType.Detect,
	})
	require.NoError(t, err)

	// 6. Add a sensor and manage it
	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{
		OID:      testOID,
		SID:      sid,
		Hostname: "prod-server-01",
		Platform: Platforms.Linux,
	}
	ms.SensorOnline[sid] = true

	s := org.GetSensor(sid)
	assert.Equal(t, "prod-server-01", s.Hostname)

	err = s.AddTag("production", 0)
	require.NoError(t, err)

	err = s.Task("os_version")
	require.NoError(t, err)

	// 7. Verify everything is in place
	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)

	fpRules, err := org.FPRules()
	require.NoError(t, err)
	assert.Len(t, fpRules, 1)

	keys, err := org.InstallationKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	outputs, err := org.Outputs()
	require.NoError(t, err)
	assert.Len(t, outputs, 1)

	tags, err := s.GetTags()
	require.NoError(t, err)
	assert.Len(t, tags, 1)

	// 8. Verify calls were tracked
	calls := ms.Calls()
	assert.True(t, len(calls) > 5, "expected many calls, got %d", len(calls))
}

// --- Service and Extension requests ---

func TestMockServiceRequest(t *testing.T) {
	ms, org := setupMock(t)

	// Direct service request
	resp := Dict{}
	err := org.ServiceRequest(&resp, "exfil", Dict{"action": "list_rules"}, false)
	require.NoError(t, err)

	// Verify call recorded
	found := false
	for _, call := range ms.Calls() {
		if call.Method == "POST" && strings.Contains(call.Path, "service") {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// --- Pre-populated state ---

func TestMockPrePopulated(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	// Pre-populate before creating org
	ms.DRRules["pre-existing-rule"] = Dict{
		"detect":  Dict{"event": "NEW_PROCESS"},
		"respond": List{},
	}
	ms.FPRules["pre-existing-fp"] = FPRule{
		Detection: Dict{"op": "is"},
		OID:       testOID,
		Name:      "pre-existing-fp",
	}
	ms.InstallationKeyStore["pre-iid"] = InstallationKey{
		ID:          "pre-iid",
		Description: "pre-existing",
		Key:         "key-value",
		JsonKey:     "{}",
		Tags:        []string{},
	}

	org, err := ms.NewOrganization()
	require.NoError(t, err)

	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Contains(t, rules, "pre-existing-rule")

	fpRules, err := org.FPRules()
	require.NoError(t, err)
	assert.Contains(t, fpRules, "pre-existing-fp")

	keys, err := org.InstallationKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

// --- Edge cases ---

func TestMockDeleteNonExistent(t *testing.T) {
	_, org := setupMock(t)

	// Deleting non-existent items should not error
	err := org.DRRuleDelete("nonexistent")
	assert.NoError(t, err)

	err = org.FPRuleDelete("nonexistent")
	assert.NoError(t, err)

	err = org.DelInstallationKey("nonexistent")
	assert.NoError(t, err)
}

func TestMockEmptyLists(t *testing.T) {
	_, org := setupMock(t)

	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Empty(t, rules)

	fpRules, err := org.FPRules()
	require.NoError(t, err)
	assert.Empty(t, fpRules)

	keys, err := org.InstallationKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)

	outputs, err := org.Outputs()
	require.NoError(t, err)
	assert.Empty(t, outputs)

	exts, err := org.Extensions()
	require.NoError(t, err)
	assert.Empty(t, exts)

	users, err := org.GetUsers()
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestMockMultipleMockServers(t *testing.T) {
	ms1 := NewMockServer("00000000-0000-0000-0000-000000000001")
	ms2 := NewMockServer("00000000-0000-0000-0000-000000000002")
	defer ms1.Close()
	defer ms2.Close()

	org1, err := ms1.NewOrganization()
	require.NoError(t, err)

	org2, err := ms2.NewOrganization()
	require.NoError(t, err)

	// Add rule to org1 only
	err = org1.DRRuleAdd("org1-rule", Dict{}, List{})
	require.NoError(t, err)

	rules1, err := org1.DRRules()
	require.NoError(t, err)
	assert.Len(t, rules1, 1)

	rules2, err := org2.DRRules()
	require.NoError(t, err)
	assert.Empty(t, rules2)
}

// --- Device ---

func TestMockDevice_AddTag(t *testing.T) {
	ms, org := setupMock(t)

	did := uuid.New().String()
	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{
		OID: testOID,
		SID: sid,
		DID: did,
	}

	d := &Device{
		DID:          did,
		Organization: org,
	}
	err := d.AddTag("device-tag", 24*time.Hour)
	require.NoError(t, err)

	// Verify the call was made
	found := false
	for _, call := range ms.Calls() {
		if call.Method == "POST" && strings.Contains(call.Path, did+"/tags") {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// --- Hive advanced ---

func TestMockHive_ListMtd(t *testing.T) {
	_, org := setupMock(t)

	hc := NewHiveClient(org)
	enabled := true
	expiry := int64(0)

	_, err := hc.Add(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "rec-1",
		Data:         Dict{"secret": "data"},
		Enabled:      &enabled,
		Expiry:       &expiry,
	})
	require.NoError(t, err)

	records, err := hc.ListMtd(HiveArgs{HiveName: "test-hive", PartitionKey: testOID})
	require.NoError(t, err)
	assert.Len(t, records, 1)
	// ListMtd strips data
	assert.Nil(t, records["rec-1"].Data)
	assert.True(t, records["rec-1"].UsrMtd.Enabled)
}

func TestMockHive_Update(t *testing.T) {
	_, org := setupMock(t)

	hc := NewHiveClient(org)
	enabled := true
	expiry := int64(0)

	// Add initial record
	_, err := hc.Add(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "update-me",
		Data:         Dict{"version": "v1"},
		Enabled:      &enabled,
		Expiry:       &expiry,
	})
	require.NoError(t, err)

	// Update data
	resp, err := hc.Update(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "update-me",
		Data:         Dict{"version": "v2"},
		Enabled:      &enabled,
		Expiry:       &expiry,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Guid)

	// Verify updated
	hd, err := hc.Get(HiveArgs{HiveName: "test-hive", PartitionKey: testOID, Key: "update-me"})
	require.NoError(t, err)
	assert.Equal(t, "v2", hd.Data["version"])
}

func TestMockHive_UpdateTx(t *testing.T) {
	_, org := setupMock(t)

	hc := NewHiveClient(org)
	enabled := true
	expiry := int64(0)

	// Add initial record
	_, err := hc.Add(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "tx-rec",
		Data:         Dict{"counter": float64(1)},
		Enabled:      &enabled,
		Expiry:       &expiry,
	})
	require.NoError(t, err)

	// Transactional update
	resp, err := hc.UpdateTx(HiveArgs{
		HiveName:     "test-hive",
		PartitionKey: testOID,
		Key:          "tx-rec",
	}, func(record *HiveData) (*HiveData, error) {
		counter, _ := record.Data["counter"].(float64)
		record.Data["counter"] = counter + 1
		return record, nil
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify
	hd, err := hc.Get(HiveArgs{HiveName: "test-hive", PartitionKey: testOID, Key: "tx-rec"})
	require.NoError(t, err)
	assert.Equal(t, float64(2), hd.Data["counter"])
}

// --- ListSensorsFromSelector ---

func TestMockListSensorsFromSelector(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{
		OID:      testOID,
		SID:      sid,
		Hostname: "selected-host",
		Platform: Platforms.Linux,
	}

	sensors, err := org.ListSensorsFromSelector("plat == linux")
	require.NoError(t, err)
	// Mock returns all sensors regardless of selector (selector filtering is server-side)
	assert.Len(t, sensors, 1)
	assert.Equal(t, "selected-host", sensors[sid].Hostname)
}

// --- Custom handler ---

func TestMockCustomHandler(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	// Override the D&R rules endpoint with custom behavior
	ms.CustomHandlers[fmt.Sprintf("/v1/rules/%s", testOID)] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"custom-rule": {"detect": "custom"}}`))
	}

	org, err := ms.NewOrganization()
	require.NoError(t, err)

	rules, err := org.DRRules()
	require.NoError(t, err)
	assert.Contains(t, rules, "custom-rule")
}

// --- Sensors with DID ---

func TestMockSensors_WithDevice(t *testing.T) {
	ms, org := setupMock(t)

	sid := uuid.New().String()
	did := uuid.New().String()
	ms.SensorStore[sid] = &Sensor{
		OID:      testOID,
		SID:      sid,
		DID:      did,
		Hostname: "device-host",
	}

	s := org.GetSensor(sid)
	assert.Equal(t, "device-host", s.Hostname)
	assert.NotNil(t, s.Device)
	assert.Equal(t, did, s.Device.DID)
}

// --- RefreshJWT through mock ---

func TestMockRefreshJWT(t *testing.T) {
	ms := NewMockServer(testOID)
	defer ms.Close()

	c, err := ms.NewClient()
	require.NoError(t, err)

	jwt, err := c.RefreshJWT(0)
	require.NoError(t, err)
	assert.Equal(t, "mock-jwt-token-refreshed", jwt)
	assert.Equal(t, "mock-jwt-token-refreshed", c.GetCurrentJWT())
}

// --- URL caching ---

func TestMockGetURLs_Caching(t *testing.T) {
	ms, org := setupMock(t)

	ms.ResetCalls()

	urls1, err := org.GetURLs()
	require.NoError(t, err)

	urls2, err := org.GetURLs()
	require.NoError(t, err)

	assert.Equal(t, urls1, urls2)

	// Second call should be cached (no additional HTTP call)
	calls := ms.Calls()
	urlCalls := 0
	for _, c := range calls {
		if strings.Contains(c.Path, "/url") {
			urlCalls++
		}
	}
	assert.Equal(t, 1, urlCalls, "expected only 1 URL fetch, rest should be cached")
}
