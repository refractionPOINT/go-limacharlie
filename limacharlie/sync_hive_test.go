package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"testing"
)

var s3TestHiveKey string
var office365TestHiveKey string

func TestAddData(t *testing.T) {

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
          tags: null`
	s3TestHiveKey = "hive-sdk-s3-test-" + randSeq(8)               // ran
	office365TestHiveKey = "hive-sdk-office365-test-" + randSeq(8) // ran
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)

	var orgConfig OrgConfig
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	if err != nil {
		t.Errorf("error unmarshal TestAddData : %v", err)
		return
	}

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHive: true})
	if err != nil {
		t.Errorf("hive sync push failure TestAddData err: %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps testAddData")
		return
	}

	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp element failed testAddData type is invalid:%s", syncOp.ElementName)
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added failed testAddData is invalid:%t", syncOp.IsAdded)
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed failed testAddData is invalid:%t", syncOp.IsRemoved)
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed testAddData no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed testAddData no add operation found for key %s ", office365TestHiveKey)
		return
	}

	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}

	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp element failed testAddData type is invalid:%s", syncOp.ElementName)
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added failed testAddData is invalid:%t", syncOp.IsAdded)
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed failed testAddData is invalid:%t", syncOp.IsRemoved)
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed testAddData no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed testAddData no add operation found for key %s ", office365TestHiveKey)
		return
	}
}

func TestDataUpdate(t *testing.T) {
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
        tags: null`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)

	var orgConfig OrgConfig
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	if err != nil {
		t.Errorf("unmarshal testDataUpdate err: %v", err)
		return
	}

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps testDataUpdate ")
		return
	}

	syncOpS3 := false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
			syncOpS3 = true

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp update s3 element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp update isAdded value for s3 is invalid:%t", syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp s3 update remove value is invalid:%t", syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("synnOp update failed no add operation found for key %s ", s3TestHiveKey)
		return
	}

	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}

	syncOpS3 = false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
			syncOpS3 = true
			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp s3 update element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp update isAdded value for s3 is invalid:%t", syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp s3 remove is invalid:%t", syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("synnOp update failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps for update ")
	}
}

func TestNoUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})
	if err != nil {
		t.Errorf("failed testNoUpdate failed to get hive data err: %+v", err)
		return
	}
	orgConfig := OrgConfig{}
	configHive := map[HiveName]map[HiveKey]HiveData{
		"cloud_sensor": hiveData,
	}
	orgConfig.Hives = configHive

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHive: true})
	if err != nil {
		t.Errorf("hive sync push testNoUpdate err: %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps for testNoUpdate")
		return
	}

	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp element type testNoUpdate is invalid:%s", syncOp.ElementName)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp testNoUpdate for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testNoUpdate removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed testNoUpdate no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed testNoUpdate no add operation found for key %s ", office365TestHiveKey)
		return
	}

	// actual run of sync
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps for update ")
		return
	}

	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp element type testNoUpdate is invalid:%s", syncOp.ElementName)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp testNoUpdate for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testNoUpdate removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed testNoUpdate no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed testNoUpdate no add operation found for key %s ", office365TestHiveKey)
		return
	}
}

func TestUsrMtdUpdate(t *testing.T) {
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
        expiry: 1663563600000
        tags: ["test1", "test2", "test3", "test4"]`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)

	orgConfig := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	if err != nil {
		t.Errorf("unmarshal testUsrMtdUpdate error: %v", err)
		return
	}

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHive: true})
	if err != nil {
		t.Errorf("hive sync push testUsrMtdUpdate err: %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps for update ")
		return
	}

	syncOpS3 := false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
			syncOpS3 = true

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp testUsrMtdUpdate update element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp testUsrMtdUpdate for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testUsrMtdUpdate removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("synnOp testUsrMtdUpdate update failed no add operation found for key %s ", s3TestHiveKey)
		return
	}

	// run actual push
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("error no orgOps for update ")
		return
	}

	syncOpS3 = false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
			syncOpS3 = true

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp testUsrMtdUpdate update element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp testUsrMtdUpdate for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testUsrMtdUpdate removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("synnOp testUsrMtdUpdate update failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
}

func TestMultipleDataUpdates(t *testing.T) {

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
        tags: null`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)

	orgConfig := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	if err != nil {
		t.Errorf("unmarshal testMultipleDataUpdates error: %v", err)
		return
	}

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push testMultipleDataUpdates %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("fith test failed no org opts present ")
		return
	}

	// lets ensure dry run output is correct
	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp testMultipleDataUpdates element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp testMultipleDataUpdates failed for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testMultipleDataUpdates failed for %s removed is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp testMultipleDataUpdates add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp testMultipleDataUpdates add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}

	// process actual run
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("fith test failed no org opts present ")
	}

	// lets ensure dry run output is correct
	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp testMultipleDataUpdates element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp testMultipleDataUpdates failed for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testMultipleDataUpdates failed for %s removed is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp testMultipleDataUpdates add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp testMultipleDataUpdates add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}
}

func TestMultipleUsrMtdUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	// yaml config value convert s3 to original state and update office 365
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
          expiry: 1663563600000
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
        expiry: 1663563600000
        tags: ["test1", "test2", "test3"]`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)

	orgConfig := OrgConfig{}
	err := yaml.Unmarshal([]byte(yamlAdd), &orgConfig)
	if err != nil {
		t.Errorf("unmarshal testMultipleUsrMtdUpdate error: %v", err)
		return
	}

	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncHive: true})
	if err != nil {
		t.Errorf("error  testMultipleUsrMtdUpdate hive sync push %+v ", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("testMultipleUsrMtdUpdate failed no org opts present ")
		return
	}

	// lets ensure dry run output is correct
	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp testMultipleUsrMtdUpdateelement type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp testMultipleUsrMtdUpdate failed for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testMultipleUsrMtdUpdate failed for %s removed is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp testMultipleUsrMtdUpdate add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOptestMultipleUsrMtdUpdate add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}

	// process actual run
	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, SyncHive: true})
	if err != nil {
		t.Errorf("error testMultipleUsrMtdUpdate hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("testMultipleUsrMtdUpdate failed no org opts present ")
		return
	}

	// lets ensure dry run output is correct
	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp testMultipleUsrMtdUpdateelement type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp testMultipleUsrMtdUpdate failed for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp testMultipleUsrMtdUpdate failed for %s removed is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp testMultipleUsrMtdUpdate add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOptestMultipleUsrMtdUpdate add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}
}

func TestRemove(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})
	if err != nil {
		t.Errorf("testRemove hive list err: %+v", err)
		return
	}

	for k, _ := range hiveData {
		if k == s3TestHiveKey || k == office365TestHiveKey {
			delete(hiveData, k)
		}
	}

	orgConfig := OrgConfig{}
	orgConfig.Hives = map[HiveName]map[HiveKey]HiveData{
		"cloud_sensor": hiveData,
	}
	orgOps, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, IsForce: true, SyncHive: true})
	if err != nil {
		t.Errorf("error TestRemove hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("testRemove failed no org opts present ")
		return
	}

	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps { // lets validate actual run
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp element type TestRemove for %s  is invalid:%s", syncOp.ElementName, syncOp.ElementType)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp added TestRemove for %s is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if !syncOp.IsRemoved {
				t.Errorf("syncOp removed TestRemove for %s  is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp remove failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp remove failed no add operation found for key %s ", office365TestHiveKey)
		return
	}

	orgOps, err = org.SyncPush(orgConfig, SyncOptions{IsDryRun: false, IsForce: true, SyncHive: true})
	if err != nil {
		t.Errorf("error hive sync push %+v", err)
		return
	}
	if orgOps == nil || len(orgOps) == 0 {
		t.Errorf("seventh test failed no org opts present ")
	}

	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps { // lets validate actual run
		if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey || syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
			if syncOp.ElementName == "cloud_sensor/"+s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == "cloud_sensor/"+office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOperationElementType.Hives {
				t.Errorf("syncOp element type TestRemove for %s  is invalid:%s", syncOp.ElementName, syncOp.ElementType)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp added TestRemove for %s is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if !syncOp.IsRemoved {
				t.Errorf("syncOp removed TestRemove for %s  is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp remove failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp remove failed no add operation found for key %s ", office365TestHiveKey)
		return
	}
}
