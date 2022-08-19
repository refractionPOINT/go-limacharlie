package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strings"
	"testing"
)

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
	testKey = "hive-test-" + randSeq(8) // ran
	yamlAdd = strings.ReplaceAll(yamlAdd, "oid-input", os.Getenv("_OID"))
	yamlAdd = strings.ReplaceAll(yamlAdd, "test-unique-key", testKey)

	var hcd HiveConfigData
	err = yaml.Unmarshal([]byte(yamlAdd), &hcd)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for k, value := range hcd {
		hiveData[k] = value
	}

	orgOps, err := org.HiveSyncPush(HiveConfig{Data: hiveData}, HiveArgs{Key: ""}, false)
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
	//	a := assert.New(t)
	//	org := getTestOrgFromEnv(a)
	//
	//	yamlTest := `version: 1
	//data:
	// Office 365 test:
	//   data:
	//     office365:
	//       client_id: 568712c8-a058-45e4-88a1-7eea17d32abf
	//       client_options:
	//         hostname: Office 365 test
	//         identity:
	//           installation_key: test installation key
	//           oid: 8cbe27f4-bfa1-4afb-ba19-138cd51389cd
	//         platform: office365
	//         sensor_seed_key: Office 365 test test update
	//       client_secret: g6A7Q~9IKqdd2Ftf64wluVoqzWSLC5tJOlzsG
	//       content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
	//       domain: SecurityInfrastructure.onmicrosoft.com
	//       endpoint: enterprise
	//       publisher_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//       tenant_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//     sensor_type: office365
	//   usr_mtd:
	//     enabled: false
	//     expiry: 0`
}

func TestNoUpdate(t *testing.T) {
	//	a := assert.New(t)
	//	org := getTestOrgFromEnv(a)
	//
	//	yamlTest := `version: 1
	//data:
	//  Office 365 test:
	//    data:
	//      office365:
	//        client_id: 568712c8-a058-45e4-88a1-7eea17d32abf
	//        client_options:
	//          hostname: Office 365 test
	//          identity:
	//            installation_key: 5f074feb-1127-41b0-920f-cb50dc1f1437
	//            oid: 8cbe27f4-bfa1-4afb-ba19-138cd51389cd
	//          platform: office365
	//          sensor_seed_key: Office 365 test
	//        client_secret: g6A7Q~9IKqdd2Ftf64wluVoqzWSLC5tJOlzsG
	//        content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
	//        domain: SecurityInfrastructure.onmicrosoft.com
	//        endpoint: enterprise
	//        publisher_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//        tenant_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//      sensor_type: office365
	//    usr_mtd:
	//      enabled: false
	//      expiry: 0`

}

func TestUsrMtdUpdate(t *testing.T) {
	//	a := assert.New(t)
	//	org := getTestOrgFromEnv(a)
	//
	//	yamlTest := `version: 1
	//data:
	//  Office 365 test:
	//    data:
	//      office365:
	//        client_id: 568712c8-a058-45e4-88a1-7eea17d32abf
	//        client_options:
	//          hostname: Office 365 test
	//          identity:
	//            installation_key: 5f074feb-1127-41b0-920f-cb50dc1f1437
	//            oid: 8cbe27f4-bfa1-4afb-ba19-138cd51389cd
	//          platform: office365
	//          sensor_seed_key: Office 365 test
	//        client_secret: g6A7Q~9IKqdd2Ftf64wluVoqzWSLC5tJOlzsG
	//        content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
	//        domain: SecurityInfrastructure.onmicrosoft.com
	//        endpoint: enterprise
	//        publisher_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//        tenant_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//      sensor_type: office365
	//    usr_mtd:
	//      enabled: true
	//      expiry: 0`
	//
	//	yamlTest := `version: 1
	//data:
	//  Office 365 test:
	//    data:
	//      office365:
	//        client_id: 568712c8-a058-45e4-88a1-7eea17d32abf
	//        client_options:
	//          hostname: Office 365 test
	//          identity:
	//            installation_key: 5f074feb-1127-41b0-920f-cb50dc1f1437
	//            oid: 8cbe27f4-bfa1-4afb-ba19-138cd51389cd
	//          platform: office365
	//          sensor_seed_key: Office 365 test
	//        client_secret: g6A7Q~9IKqdd2Ftf64wluVoqzWSLC5tJOlzsG
	//        content_types: Audit.AzureActiveDirectory,Audit.Exchange,Audit.SharePoint,Audit.General,DLP.All
	//        domain: SecurityInfrastructure.onmicrosoft.com
	//        endpoint: enterprise
	//        publisher_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//        tenant_id: 6623cd65-07f3-4a03-a246-48da7845af4b
	//      sensor_type: office365
	//    usr_mtd:
	//      enabled: false
	//      expiry: 1755487360000`

}

func TestMultipleDataUpdates(t *testing.T) {
	//a := assert.New(t)
	//org := getTestOrgFromEnv(a)

}

func TestMultipleUsrMtdUpdate(t *testing.T) {
	//a := assert.New(t)
	//org := getTestOrgFromEnv(a)

}
