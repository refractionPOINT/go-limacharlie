package limacharlie

import (
	"testing"
)

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
