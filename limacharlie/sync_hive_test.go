package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log"
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

	// grab current data as test should not remove anything
	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})

	if err != nil {
		t.Errorf("add data failed to get data %+v \n ", err)
		return
	}

	// yaml config value to add
	yamlAdd := `test-s3-unique-key:
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
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", s3TestHiveKey)

	var hcd HiveConfigData
	err = yaml.Unmarshal([]byte(yamlAdd), &hcd)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for k, value := range hcd {
		hiveData[k] = value
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})

	if err != nil {
		t.Errorf("failed sync push add dry run err: %+v ", err)
		return
	}

	// lets ensure dry run output is correct
	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type for %s is invalid:%s", syncOp.ElementName, syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added for %s is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}

	// run actual push dry run is valid
	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})

	if err != nil {
		t.Errorf("failed sync push add err %+v ", err)
		return
	}

	if orgOps == nil {
		t.Error("failed add orgOps is nil ")
		return
	}

	// lets ens
	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added is invalid:%t", syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed is invalid:%t", syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}
}

func TestDataUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})

	if err != nil {
		t.Errorf("add data failed to get data %+v \n ", err)
		return
	}

	yamlAdd := `test-s3-unique-key:
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

	var hcd HiveConfigData
	err = yaml.Unmarshal([]byte(yamlAdd), &hcd)
	if err != nil {
		t.Errorf("failed to unmarshal test data error: %v", err)
		return
	}

	for k, value := range hcd {
		hiveData[k] = value
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})
	if err != nil {
		t.Errorf("failed sync push update dry run err: %+v ", err)
		return
	}

	syncOpS3 := false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey {
			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
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

	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})
	if err != nil {
		t.Errorf("failed sync push update err: %+v ", err)
		return
	}

	syncOpS3 = false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey {
			syncOpS3 = true
			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
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
		t.Errorf("failed test no update failed to get hive data err: %+v", err)
		return
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})
	if err != nil {
		t.Errorf("failed sync push no update dry run err: %+v ", err)
		return
	}
	if orgOps == nil {
		t.Error("failed sync push no update orgOps is nil")
	}

	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type is invalid:%s", syncOp.ElementName)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp noUpdate for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp noUpdate removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}

	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})

	if err != nil {
		t.Errorf("failed sync push no update err: %+v ", err)
		return
	}

	if orgOps == nil {
		t.Error("failed sync push no update orgOps is nil")
		return
	}

	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}
			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type is invalid:%s", syncOp.ElementName)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp noUpdate for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp noUpdate removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("syncOp add failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
	if !syncOpOffice {
		t.Errorf("syncOp add failed no add operation found for key %s ", office365TestHiveKey)
		return
	}
}

func TestUsrMtdUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	yamlAdd := `test-s3-unique-key:
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

	var hcd HiveConfigData
	err := yaml.Unmarshal([]byte(yamlAdd), &hcd)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hcd}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})
	if err != nil {
		t.Errorf("failed sync usr mtd update dry run err: %+v ", err)
		return
	}
	if orgOps == nil {
		t.Errorf("failed sync push usrt mtd update org ops is nil ")
	}

	syncOpS3 := false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey {
			syncOpS3 = true
			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp usrMtd update element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp usrMtd for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp usrMtd removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("synnOp usrMtd update failed no add operation found for key %s ", s3TestHiveKey)
		return
	}

	// actual run
	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hcd}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})
	if err != nil {
		t.Errorf("failed sync push usr mtd update err: %+v ", err)
		return
	}
	if orgOps == nil {
		t.Errorf("failed sync push usrt mtd update org ops is nil ")
	}

	syncOpS3 = false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey {
			syncOpS3 = true
			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp usrMtd update element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp usrMtd for %s added is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp usrMtd removed for %s is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 {
		t.Errorf("synnOp usrMtd update failed no add operation found for key %s ", s3TestHiveKey)
		return
	}
}

func TestMultipleDataUpdates(t *testing.T) {

	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	// grab current data as test should not remove anything
	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})

	if err != nil {
		t.Errorf("add data failed to get data %+v \n ", err)
		return
	}

	// yaml config value convert s3 to original state and update office 365
	yamlAdd := `test-s3-unique-key:
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
	s3TestHiveKey = "hive-sdk-test-" + randSeq(8)                  // ran
	office365TestHiveKey = "hive-sdk-office365-test-" + randSeq(8) // ran
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)

	var hcd HiveConfigData
	err = yaml.Unmarshal([]byte(yamlAdd), &hcd)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for k, value := range hcd {
		hiveData[k] = value
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})

	if err != nil {
		t.Errorf("failed sync push add dry run err: %+v ", err)
		return
	}

	// lets ensure dry run output is correct
	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added is invalid:%t", syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed is invalid:%t", syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 || !syncOpOffice {
		t.Errorf("synnOp add failed no add operation found for key %s ", s3TestHiveKey)
	}

	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})

	if err != nil {
		t.Errorf("failed sync push add err %+v ", err)
		return
	}

	if orgOps == nil {
		t.Error("failed add orgOps is nil ")
		return
	}

	// lets ens
	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps {
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type is invalid:%s", syncOp.ElementName)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added is invalid:%t", syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed is invalid:%t", syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 || !syncOpOffice {
		t.Errorf("synnOp add failed no add operation found for key %s ", s3TestHiveKey)
	}
}

func TestMultipleUsrMtdUpdate(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	// grab current data as test should not remove anything
	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})

	if err != nil {
		t.Errorf("add data failed to get data %+v \n ", err)
		return
	}

	// leave config as is but only update usr_mtd
	yamlAdd := `test-s3-unique-key:
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
	s3TestHiveKey = "hive-sdk-test-" + randSeq(8)                  // ran
	office365TestHiveKey = "hive-sdk-office365-test-" + randSeq(8) // ran
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-s3-unique-key", s3TestHiveKey)
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-office-365-key", office365TestHiveKey)

	var hcd HiveConfigData
	err = yaml.Unmarshal([]byte(yamlAdd), &hcd)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for k, value := range hcd {
		hiveData[k] = value
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})

	if err != nil {
		t.Errorf("failed sync push add dry run err: %+v ", err)
		return
	}

	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps { // lest validate dry run
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type multiple mtd update for %s  is invalid:%s", syncOp.ElementName, syncOp.ElementType)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added multiple mtd update for %s is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed multiple mtd for %s  is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 || !syncOpOffice {
		t.Errorf("synnOp add failed no add operation found for key %s ", s3TestHiveKey)
		return // dry run failed lets return
	}

	// run actual update
	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})

	if err != nil {
		t.Errorf("failed sync push add err %+v ", err)
		return
	}

	if orgOps == nil {
		t.Error("failed add orgOps is nil ")
		return
	}

	syncOpS3, syncOpOffice = false, false
	for _, syncOp := range orgOps { // lets validate actual run
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type multiple mtd update for %s  is invalid:%s", syncOp.ElementName, syncOp.ElementType)
				return
			}
			if !syncOp.IsAdded {
				t.Errorf("syncOp added multiple mtd update for %s is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if syncOp.IsRemoved {
				t.Errorf("syncOp removed multiple mtd for %s  is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 || !syncOpOffice {
		t.Errorf("synnOp add failed no add operation found for key %s ", s3TestHiveKey)
	}
}

func TestRemove(t *testing.T) {

	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// grab current data as test should not remove anything
	hiveData, err := testHiveClient.List(HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	})

	if err != nil {
		t.Errorf("add data failed to get data %+v \n ", err)
		return
	}

	for k, _ := range hiveData {

		if k == s3TestHiveKey || k == office365TestHiveKey {
			delete(hiveData, k)
		}
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: true,
	})

	if err != nil {
		t.Errorf("failed sync push add dry run err: %+v ", err)
		return
	}

	syncOpS3, syncOpOffice := false, false
	for _, syncOp := range orgOps { // lets validate actual run
		if syncOp.ElementName == s3TestHiveKey || syncOp.ElementName == office365TestHiveKey {
			if syncOp.ElementName == s3TestHiveKey {
				syncOpS3 = true
			}

			if syncOp.ElementName == office365TestHiveKey {
				syncOpOffice = true
			}

			if syncOp.ElementType != OrgSyncOpsHiveType.Data {
				t.Errorf("syncOp element type multiple mtd update for %s  is invalid:%s", syncOp.ElementName, syncOp.ElementType)
				return
			}
			if syncOp.IsAdded {
				t.Errorf("syncOp added multiple mtd update for %s is invalid:%t", syncOp.ElementName, syncOp.IsAdded)
				return
			}
			if !syncOp.IsRemoved {
				t.Errorf("syncOp removed multiple mtd for %s  is invalid:%t", syncOp.ElementName, syncOp.IsRemoved)
				return
			}
		}
	}
	if !syncOpS3 || !syncOpOffice {
		t.Errorf("synnOp add failed no add operation found for key %s ", s3TestHiveKey)
	}
}
