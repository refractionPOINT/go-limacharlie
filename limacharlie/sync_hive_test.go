package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strings"
	"testing"
)

var syncTestHiveKey string

func TestAddData(t *testing.T) {

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

	yamlAdd := `test-unique-key:
    data:
      s3:
        access_key: "test-accesskey"
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
      tags: null`
	syncTestHiveKey = "hive-test-" + randSeq(8) // ran
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-unique-key", syncTestHiveKey)

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

	yamlAdd := `test-unique-key:
    data:
      s3:
        access_key: "test-accesskey-update"
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
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-unique-key", syncTestHiveKey)

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

	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
		HiveName: "cloud_sensor",
		IsDryRun: false,
	})

	if err != nil {
		t.Errorf("failed sync push update err: %+v ", err)
		return
	}

	if orgOps == nil {
		t.Errorf("failed update org ops is nil ")
		return
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
		t.Errorf("add data failed to get data %+v \n ", err)
		return
	}

	yamlAdd := `test-unique-key:
    data:
      s3:
        access_key: "test-accesskey-update"
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
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-unique-key", syncTestHiveKey)

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
		t.Errorf("failed sync push no update dry run err: %+v ", err)
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
	}
}

func TestUsrMtdUpdate(t *testing.T) {
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

	yamlAdd := `test-unique-key:
    data:
      s3:
        access_key: "test-accesskey"
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
      enabled: true
      expiry: 1663563600000
      tags: ["test1", "test2", "test3", "test4"]`
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-unique-key", syncTestHiveKey)

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
		t.Errorf("failed sync push usr mtd update dry run err: %+v ", err)
		return
	}

	orgOps, err = org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveSyncOptions{
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
}

func TestMultipleDataUpdates(t *testing.T) {

}

func TestMultipleUsrMtdUpdate(t *testing.T) {
	//a := assert.New(t)
	//org := getTestOrgFromEnv(a)}
}
