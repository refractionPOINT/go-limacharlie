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

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	}, false)
	if err != nil {
		t.Errorf("failed sync push err: %+v ", err)
		return
	}

	if orgOps == nil {
		t.Errorf("failed update no org ops: %+v ", err)
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

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	}, false)
	if err != nil {
		t.Errorf("failed sync push err: %+v ", err)
		return
	}

	if orgOps == nil {
		t.Errorf("failed update no org ops: %+v ", err)
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

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	}, false)
	if err != nil {
		t.Errorf("failed sync push err: %+v ", err)
		return
	}

	if orgOps == nil {
		t.Errorf("failed update no org ops: %+v ", err)
		return
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

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveArgs{
		PartitionKey: os.Getenv("_OID"),
		HiveName:     "cloud_sensor",
	}, false)
	if err != nil {
		t.Errorf("failed sync push err: %+v ", err)
		return
	}

	if orgOps == nil {
		t.Errorf("failed update no org ops: %+v ", err)
		return
	}
}

func TestMultipleDataUpdates(t *testing.T) {

}

func TestMultipleUsrMtdUpdate(t *testing.T) {
	//a := assert.New(t)
	//org := getTestOrgFromEnv(a)}
}
