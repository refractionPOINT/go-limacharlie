package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"testing"
	"time"
)

var s3TestHiveKey string
var office365TestHiveKey string
var fpTestHiveKey string

func TestHiveAddData(t *testing.T) {

	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	yamlAdd := `hives:
  cloud_sensor:
    test-s3-unique-key:
        data:
          s3:
            access_key: "test-access-key"
            bucket_name: aws-cloudtrail-logs-005407990505-225b8680
            client_options:
              hostname: cloudtrail
              identity:
                installation_key: test-install-key
                oid: oid-input
              platform: aws
              sensor_seed_key: cloudtrail
            secret_key: secret-key
          sensor_type: s3
        usr_mtd:
          enabled: false
          expiry: 0
          tags: null
          comment: something
    test-office-365-key:
        data:
          office365:
            client_id: test-client-id
            client_options:
              hostname: Office 365 test
              identity:
                installation_key: test-install-key
                oid: oid-input
              platform: office365
              sensor_seed_key: Office 365 test
            client_secret: test-secret
            content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
            domain: SecurityInfrastructure.onmicrosoft.com
            endpoint: enterprise
            publisher_id: test-publisher-id
            tenant_id: test-tenant-id
          sensor_type: office365
        usr_mtd:
          enabled: false
          expiry: 0
          tags: null
  fp: 
    'test-sdk-FP':
      data:
        op: and
        rules:
        - op: is
          path: cat
          value: '00285-WIN-RDP_Connection_From_Non-RFC-1918_Address'
        - case sensitive: false
          op: is
          path: detect/event/FILE_PATH
          value: C:\Windows\System32\svchost.exe
      usr_mtd:
        enabled: false
        expiry: 0
        tags:`
	s3TestHiveKey = "hive-sdk-s3-test-" + randSeq(8)
	office365TestHiveKey = "hive-sdk-office365-test-" + randSeq(8)
	fpTestHiveKey = "hive-sdk-fp-test-" + randSeq(8)
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-sdk-FP", fpTestHiveKey)

	var orgConfig OrgConfig
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	a.NoError(err)

	// start of dry run
	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "fp/" + fpTestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + office365TestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps))

	// start of actual run
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"cloud_sensor": true, "dr-general": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "fp/" + fpTestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + office365TestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps))
}

func TestHiveDataUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	yamlAdd := `hives:
  cloud_sensor:
    test-s3-unique-key:
      data:
        s3:
          access_key: "test-access-key-update"
          bucket_name: aws-cloudtrail-logs-005407990505-225b8680
          client_options:
            hostname: cloudtrail
            identity:
              installation_key: test-install-key-update
              oid: oid-input
            platform: aws
            sensor_seed_key: cloudtrail
          secret_key: secret-key
        sensor_type: s3
      usr_mtd:
        enabled: false
        expiry: 0
        comment: else
        tags: null`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)

	var orgConfig OrgConfig
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	a.NoError(err)

	// start of dry run
	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"cloud_sensor": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(expectedOps, orgOps)

	// start of actual run
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"cloud_sensor": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(expectedOps, orgOps)

}

func TestHiveNoUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	hiveSensorData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})
	a.NoError(err)

	hiveFpData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "fp",
	})
	a.NoError(err)

	orgConfig := OrgConfig{}
	configHive := map[HiveName]map[HiveKey]SyncHiveData{
		"cloud_sensor": hiveSensorData.AsSyncConfigData(),
		"fp":           hiveFpData.AsSyncConfigData(),
	}
	orgConfig.Hives = configHive

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	syncOpS3, syncOpOffice, syncOpFp := false, false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey || syncOp.ElementName == "fp/"+fpTestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}
			if syncOp.ElementName == "fp/"+fpTestHiveKey {
				syncOpFp = true
			}

			a.Equal(OrgSyncOperationElementType.Hives, syncOp.ElementType)
			a.False(syncOp.IsAdded)
			a.False(syncOp.IsRemoved)
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp failed testNoUpdate no operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp  testNoUpdate no operation found for key %s ", office365TestHiveKey)
		return
	}
	if !syncOpFp {
		t.Errorf("syncOp failed testNoUpdate no operation found for key %s ", fpTestHiveKey)
		return
	}

	// actual run of sync
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	syncOpS3, syncOpOffice, syncOpFp = false, false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey || syncOp.ElementName == "fp/"+fpTestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}
			if syncOp.ElementName == "fp/"+fpTestHiveKey {
				syncOpFp = true
			}

			a.Equal(OrgSyncOperationElementType.Hives, syncOp.ElementType)
			a.False(syncOp.IsAdded)
			a.False(syncOp.IsRemoved)
		}
	}
	a.True(syncOpS3)
	a.True(syncOpOffice)
	a.True(syncOpFp)
}

func TestHiveUsrMtdUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	yamlAdd := `hives:
  cloud_sensor:
    test-s3-unique-key:
      data:
        s3:
          access_key: "test-access-key-update"
          bucket_name: aws-cloudtrail-logs-005407990505-225b8680
          client_options:
            hostname: cloudtrail
            identity:
              installation_key: test-install-key
              oid: oid-input
            platform: aws
            sensor_seed_key: cloudtrail
          secret_key: secret-key
        sensor_type: s3
      usr_mtd:
        enabled: false
        expiry: 2663563600000
        tags: ["test1", "test2", "test3", "test4"]`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)

	orgConfig := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	a.NoError(err)

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"cloud_sensor": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(expectedOps, orgOps)

	// run actual push
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"cloud_sensor": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(expectedOps, orgOps)

}

func TestHiveMultipleDataUpdates(t *testing.T) {

	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	yamlAdd := `hives:
  cloud_sensor:
    test-s3-unique-key:
        data:
          s3:
            access_key: "test-access-key"
            bucket_name: aws-cloudtrail-logs-005407990505-225b8680
            client_options:
              hostname: cloudtrail
              identity:
                installation_key: test-install-key
                oid: oid-input
              platform: aws
              sensor_seed_key: cloudtrail
            secret_key: secret-key
          sensor_type: s3
        usr_mtd:
          enabled: false
          expiry: 0
          tags: null
    test-office-365-key:
      data:
        office365:
          client_id: test-client-id
          client_options:
            hostname: Office 365 test host name update
            identity:
              installation_key: test-install-key-update
              oid: oid-input
            platform: office365
            sensor_seed_key: Office 365 test update
          client_secret: test-secret
          content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
          domain: SecurityInfrastructure.onmicrosoft.com
          endpoint: enterprise
          publisher_id: test-publisher-id
          tenant_id: test-tenant-id
        sensor_type: office365
      usr_mtd:
        enabled: false
        expiry: 0
        tags: null
  fp: 
    'test-sdk-FP':
      data:
        op: and
        rules:
        - op: is
          path: cat
          value: '00285-WIN-RDP_Connection_From_Non-RFC-1918_Address'
        - case sensitive: true
          op: is
          path: detect/event/FILE_PATH
          value: C:\Windows\System32\svch.exe
      usr_mtd:
        enabled: false
        expiry: 0
        tags:`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-sdk-FP", fpTestHiveKey)

	orgConfig := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	a.NoError(err)

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "fp/" + fpTestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + office365TestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps))

	// process actual run
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "fp/" + fpTestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + office365TestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps))
}

func TestHiveMultipleUsrMtdUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	// yaml data is exactly the same except for changes in mtd data
	yamlAdd := `hives:
  cloud_sensor:
    test-s3-unique-key:
        data:
          s3:
            access_key: "test-access-key"
            bucket_name: aws-cloudtrail-logs-005407990505-225b8680
            client_options:
              hostname: cloudtrail
              identity:
                installation_key: test-install-key
                oid: oid-input
              platform: aws
              sensor_seed_key: cloudtrail
            secret_key: secret-key
          sensor_type: s3
        usr_mtd:
          enabled: false
          expiry: 0
          tags: ["test1", "test2", "test3"]
    test-office-365-key:
      data:
        office365:
          client_id: test-client-id
          client_options:
            hostname: Office 365 test host name update
            identity:
              installation_key: test-install-key-update
              oid: oid-input
            platform: office365
            sensor_seed_key: Office 365 test update
          client_secret: test-secret
          content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
          domain: SecurityInfrastructure.onmicrosoft.com
          endpoint: enterprise
          publisher_id: test-publisher-id
          tenant_id: test-tenant-id
        sensor_type: office365
      usr_mtd:
        enabled: false
        expiry: 0
        tags: ["test1", "test2", "test3"]
  fp: 
    'test-sdk-FP':
      data:
        op: and
        rules:
        - op: is
          path: cat
          value: '00285-WIN-RDP_Connection_From_Non-RFC-1918_Address'
        - case sensitive: true
          op: is
          path: detect/event/FILE_PATH
          value: C:\Windows\System32\svch.exe
      usr_mtd:
        enabled: false
        expiry: 0
        tags: ["test1", "test2", "test3"]`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-sdk-FP", fpTestHiveKey)

	orgConfig := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	a.NoError(err)

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "fp/" + fpTestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + office365TestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	if !a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps)) {
		return
	}

	// process actual run
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "fp/" + fpTestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + office365TestHiveKey, IsAdded: true, IsRemoved: false},
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "cloud_sensor/" + s3TestHiveKey, IsAdded: true, IsRemoved: false},
	})
	a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps))
}

func TestHiveRemove(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	hiveSensorData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})
	a.NoError(err)

	hiveFpData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "fp",
	})
	a.NoError(err)

	for k := range hiveSensorData {
		if k == s3TestHiveKey || k == office365TestHiveKey {
			delete(hiveSensorData, k)
		}
	}

	for k := range hiveFpData {
		if k == fpTestHiveKey {
			delete(hiveFpData, k)
		}
	}

	orgConfig := OrgConfig{}
	orgConfig.Hives = map[HiveName]map[HiveKey]SyncHiveData{
		"cloud_sensor": hiveSensorData.AsSyncConfigData(),
		"fp":           hiveFpData.AsSyncConfigData(),
	}
	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, IsForce: true, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	syncOpS3, syncOpOffice, syncOpFp := false, false, false
	for _, syncOp := range orgOps { // lets validate actual run
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey || syncOp.ElementName == "fp/"+fpTestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}
			if syncOp.ElementName == "fp/"+fpTestHiveKey {
				syncOpFp = true
			}

			a.Equal(OrgSyncOperationElementType.Hives, syncOp.ElementType)
			a.False(syncOp.IsAdded)
			a.True(syncOp.IsRemoved)
		}
	}
	a.True(syncOpS3)
	a.True(syncOpOffice)
	a.True(syncOpFp)

	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, IsForce: true, SyncHives: map[string]bool{"cloud_sensor": true, "fp": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	syncOpS3, syncOpOffice, syncOpFp = false, false, false
	for _, syncOp := range orgOps { // lets validate actual run
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey || syncOp.ElementName == "fp/"+fpTestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}
			if syncOp.ElementName == "fp/"+fpTestHiveKey {
				syncOpFp = true
			}

			a.Equal(OrgSyncOperationElementType.Hives, syncOp.ElementType)
			a.False(syncOp.IsAdded)
			a.True(syncOp.IsRemoved)
		}
	}
	a.True(syncOpS3)
	a.True(syncOpOffice)
	a.True(syncOpFp)
}

func TestHiveDRService(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	hive := NewHiveClient(org)

	err := org.ResourceSubscribe("yara", "replicant")
	a.NoError(err)

	// give changes a few secs to take place before list call
	time.Sleep(time.Second * 2)
	setData, err := hive.List(HiveArgs{HiveName: "dr-service", PartitionKey: os.Getenv("_OID")})
	a.NoError(err)

	// ensure data is returning as null
	for k, v := range setData {
		a.Nil(v.Data, "set data should be nil for key %s", k)
	}

	yaraRule := `
hives:
  dr-service:
    __YaraReplicant___sensor_sync_yara:
      data: null
      usr_mtd:
        enabled: false
        expiry: 2663563600000
        tags: ["test1", "test2", "test3"]`

	orgConfig := OrgConfig{}
	err = yaml.Unmarshal([]byte(yaraRule), &orgConfig)
	a.NoError(err)

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHives: map[string]bool{"dr-service": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Hives, ElementName: "dr-service/" + "__YaraReplicant___sensor_sync_yara", IsAdded: true, IsRemoved: false},
	})
	if !a.Equal(sortSyncOps(expectedOps), sortSyncOps(orgOps)) {
		return
	}

	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHives: map[string]bool{"dr-service": true}})
	a.NoError(err)
	a.NotEmpty(orgOps)

	drData, err := hive.GetMTD(HiveArgs{HiveName: "dr-service", PartitionKey: os.Getenv("_OID"), Key: "__YaraReplicant___sensor_sync_yara"})
	a.NoError(err)

	a.Nil(drData.Data)
	a.False(drData.UsrMtd.Enabled)
	a.Equal(int64(2663563600000), drData.UsrMtd.Expiry)
	a.Equal(3, len(drData.UsrMtd.Tags))

	err = org.ResourceUnsubscribe("yara", "replicant")
	a.NoError(err)
}

func TestHiveMerge(t *testing.T) {

	yamlOne := `hives:
  cloud_sensor:
    test-s3-unique-key:
        data:
          s3:
            access_key: "test-access-key"
            bucket_name: aws-cloudtrail-logs-005407990505-225b8680
            client_options:
              hostname: cloudtrail
              identity:
                installation_key: test-install-key
                oid: oid-input
              platform: aws
              sensor_seed_key: cloudtrail
            secret_key: secret-key
          sensor_type: s3
        usr_mtd:
          enabled: false
          expiry: 0
          tags: null
    test-office-365-key:
      data:
        office365:
          client_id: test-client-id
          client_options:
            hostname: Office 365 test host name update
            identity:
              installation_key: test-install-key-update
              oid: oid-input
            platform: office365
            sensor_seed_key: Office 365 test update
          client_secret: test-secret
          content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
          domain: SecurityInfrastructure.onmicrosoft.com
          endpoint: enterprise
          publisher_id: test-publisher-id
          tenant_id: test-tenant-id
        sensor_type: office365
      usr_mtd:
        enabled: false
        expiry: 0
        tags: null`
	yamlOne = strings.ReplaceAll(yamlOne, "oid-input", os.Getenv("_OID"))
	yamlOne = strings.ReplaceAll(yamlOne, "test-s3-unique-key", s3TestHiveKey)
	yamlOne = strings.ReplaceAll(yamlOne, "test-office-365-key", office365TestHiveKey)

	orgConfigOne := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlOne), &orgConfigOne)
	a := assert.New(t)
	a.NoError(err)

	yaml2 := `hives:
  cloud_sensor:
    test-s3-unique-key:
        data:
          s3:
            access_key: "test-access-key"
            bucket_name: aws-cloudtrail-logs-005407990505-225b8680
            client_options:
              hostname: cloudtrail
              identity:
                installation_key: test-install-key
                oid: oid-input
              platform: aws
              sensor_seed_key: cloudtrail
            secret_key: secret-key
          sensor_type: s3
        usr_mtd:
          enabled: false
          expiry: 2663563600000
          tags: ["test1", "test2", "test3"]
    test-office-365-key:
      data:
        office365:
          client_id: test-client-id
          client_options:
            hostname: Office 365 test host name update
            identity:
              installation_key: test-install-key-update
              oid: oid-input
            platform: office365
            sensor_seed_key: Office 365 test update
          client_secret: test-secret
          content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
          domain: SecurityInfrastructure.onmicrosoft.com
          endpoint: enterprise
          publisher_id: test-publisher-id
          tenant_id: test-tenant-id
        sensor_type: office365
      usr_mtd:
        enabled: false
        expiry: 2663563600000
        tags: ["test1", "test2", "test3"]
    test-gcpTest-key:
      data:
        pubsub:
          client_options:
            hostname: gcpTest
            identity:
              installation_key: test-intsll-key
              oid: test-oin
            platform: gcp
            sensor_seed_key: gcpTest
          project_name: adf
          service_account_creds: "{ gcp }"
          sub_name: asdf
      usr_mtd:
        enabled: false
        expiry: 0
        tags:`
	yaml2 = strings.ReplaceAll(yaml2, "oid-input", os.Getenv("_OID"))
	yaml2 = strings.ReplaceAll(yaml2, "test-s3-unique-key", s3TestHiveKey)
	yaml2 = strings.ReplaceAll(yaml2, "test-office-365-key", office365TestHiveKey)

	orgConfigTwo := OrgConfig{}
	err = yaml.Unmarshal([]byte(yaml2), &orgConfigTwo)
	a.NoError(err)

	// process merge
	newOrgConfig := orgConfigOne.Merge(orgConfigTwo)
	for n := range newOrgConfig.Hives {
		for k, data := range newOrgConfig.Hives[n] {
			if k == s3TestHiveKey || k == office365TestHiveKey {
				equal, err := data.Equals(orgConfigTwo.Hives[n][k])
				a.NoError(err)
				a.True(equal, "config data should be equal for key %s", k)
			}
		}
	}

	// validate newOrgConfig also contains new data
	gcpTest := newOrgConfig.Hives["cloud_sensor"]["test-gcpTest-key"]
	a.NotNil(gcpTest.Data)
}
