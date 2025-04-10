package limacharlie

import (
	"encoding/json"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testHiveClient *HiveClient
var testKey string

func TestHiveClient(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	tests := map[string]func(t *testing.T){
		"add":           hiveAddTest,
		"get":           hiveGetTest,
		"getMtd":        hiveGetMtdTest,
		"list":          hiveListTest,
		"listMtd":       hiveListMtdTest,
		"update":        hiveUpdate,
		"tx":            hiveUpdateTx,
		"getPublicGUID": hiveGetPublicByGUIDTest,
		"remove":        hiveRemove,
		"batch":         hiveBatchTest,
	}

	// ensure test execute in proper order
	testArray := []string{"add", "get", "getMtd", "list", "listMtd", "update", "tx", "getPublicGUID", "remove", "batch"}
	for _, name := range testArray {
		t.Run(name, tests[name])
	}
}

func hiveAddTest(t *testing.T) {
	jsonString := `{
					  "pubsub": {
						"client_options": {
						  "hostname": "gcpTest",
						  "identity": {
							"installation_key": "test install key",
							"oid": "oid-input"
						  },
						  "platform": "gcp",
						  "sensor_seed_key": "gcpTest"
						},
						"project_name": "adf",
						"service_account_creds": "{ gcp }",
						"sub_name": "asdf"
					  },
					  "sensor_type": "pubsub"
				}`
	jsonString = strings.ReplaceAll(jsonString, "oid-input", os.Getenv("_OID"))

	testKey = "hive-test-" + randSeq(8) // ran key to keep track of newly created hive data record
	data := Dict{}
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		panic(err)
	}
	hiveResp, err := testHiveClient.Add(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey,
		Data:         data})

	if err != nil {
		t.Errorf("hive client failed add: %+v \n", err)
		return
	}

	if hiveResp.Hive.Name != "cloud_sensor" {
		t.Errorf("hive add failed incorrect hive name invalidName: %s", hiveResp.Hive.Name)
		return
	}

	if hiveResp.Name != testKey {
		t.Errorf("hive add call failed invalidKey:%s", hiveResp.Name)
		return
	}
}

func hiveGetTest(t *testing.T) {
	hiveData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive Get failed: %+v \n", err)
		return
	}

	if hiveData.Data == nil {
		t.Errorf("hive get failed missing data info")
		return
	}

	if hiveData.UsrMtd.Enabled {
		t.Error("hive get failed UsrMtd enabled should be false")
		return
	}

	if hiveData.UsrMtd.Expiry != 0 {
		t.Errorf("hive get failed UsrMtd expiry should be zero invalidExpiry: %d ", hiveData.UsrMtd.Expiry)
		return
	}

	if hiveData.UsrMtd.Tags != nil {
		t.Errorf("hive get failed UsrMtd tags should be null invalidTags: %s ", hiveData.UsrMtd.Tags)
	}

	if hiveData.UsrMtd.Comment != "" {
		t.Errorf("hive get failed UsrMtd comment should be empty: %s ", hiveData.UsrMtd.Comment)
	}
}

func hiveGetMtdTest(t *testing.T) {
	hiveData, err := testHiveClient.GetMTD(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive GetMtd failed err: %+v \n", err)
		return
	}

	if hiveData.Data != nil {
		t.Error("hive getMtdFailed data is not nil ")
	}
}

func hiveListTest(t *testing.T) {
	hiveSet, err := testHiveClient.List(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID")})

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive List failed: %+v \n", err)
		return
	}

	if _, ok := hiveSet[testKey]; !ok {
		t.Errorf("hive list failed key not found, key: %s ", testKey)
		return
	}

	if hiveSet[testKey].Data == nil {
		t.Errorf("hive list failed test key data nil, key: %s", testKey)
	}
}

func hiveListMtdTest(t *testing.T) {
	hiveSet, err := testHiveClient.ListMtd(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID")})

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive ListMtd failed: %+v \n", err)
		return
	}

	if _, ok := hiveSet[testKey]; !ok {
		t.Errorf("hive listMtd failed test key not found, key: %s ", testKey)
		return
	}

	if hiveSet[testKey].Data != nil {
		t.Error("hive list failed data field not nil")
	}
}

func hiveUpdate(t *testing.T) {
	jsonString := `{
				  "s3": {
					"access_key": "access key",
					"bucket_name": "bucket_name",
					"client_options": {
					  "hostname": "syslog-test",
					  "identity": {
						"installation_key": "test install key",
						"oid": "oid-input"
					  },
					  "platform": "text",
					  "sensor_seed_key": "syslog-test"
					},
					"prefix": "prefix",
					"secret_key": "secret key"
				  },
				  "sensor_type": "s3"
				}`
	jsonString = strings.ReplaceAll(jsonString, "oid-input", os.Getenv("_OID"))

	data := Dict{}
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		panic(err)
	}
	testComment := "test comment"
	_, err := testHiveClient.Update(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey,
		Data:         data,
		Tags:         []string{"test1", "test2"},
		Comment:      &testComment,
	})

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive update failed, error: %+v \n", err)
		return
	}

	// get newly created data to ensure update processed correctly
	updateData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	if err != nil {
		t.Errorf("hive update failiure check, error %+v ", err)
		return
	}

	if _, ok := updateData.Data["s3"]; !ok {
		t.Error("hive update failed missing s3 key data field ")
		return
	}

	if updateData.UsrMtd.Tags == nil {
		t.Errorf("hive update failed tags not set, tags:%s ", []string{"test1", "test2"})
		return
	}

	if len(updateData.UsrMtd.Tags) != 2 {
		t.Errorf("hive update failed invalid tag length of %d", len(updateData.UsrMtd.Tags))
	}

	if updateData.UsrMtd.Comment != testComment {
		t.Errorf("hive update failed invalid comment, comment:%s ", testComment)
	}
}

func hiveRemove(t *testing.T) {
	// test remove and clean up test data
	_, err := testHiveClient.Remove(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive Remove failed, error: %+v", err)
	}
}

func randSeq(n int) string {
	rand.Seed(time.Now().Unix())
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func hiveUpdateTx(t *testing.T) {
	nRan := 0

	r, err := testHiveClient.UpdateTx(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey,
	}, func(record *HiveData) (*HiveData, error) {
		record.UsrMtd.Tags = []string{"test1", "test2", "test4"}
		nRan++

		// We artificially do an update our of band to trigger a retry.
		jsonString := `{
			"s3": {
			  "access_key": "access key",
			  "bucket_name": "bucket_name",
			  "client_options": {
				"hostname": "syslog-test",
				"identity": {
				  "installation_key": "test install key",
				  "oid": "oid-input"
				},
				"platform": "text",
				"sensor_seed_key": "syslog-test"
			  },
			  "prefix": "prefix",
			  "secret_key": "secret key"
			},
			"sensor_type": "s3"
		  }`
		jsonString = strings.ReplaceAll(jsonString, "oid-input", os.Getenv("_OID"))

		data := Dict{}
		if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
			panic(err)
		}
		_, err := testHiveClient.Add(HiveArgs{
			HiveName:     "cloud_sensor",
			PartitionKey: os.Getenv("_OID"),
			Key:          testKey,
			Data:         data,
			Tags:         []string{"test1", "test3"},
		})

		// validate test ran correctly
		if err != nil {
			t.Errorf("hive update failed, error: %+v \n", err)
			return nil, err
		}

		return record, nil
	})

	if err != nil {
		t.Errorf("hive update tx failed, error: %+v \n", err)
		return
	}

	if r == nil {
		t.Error("hive update tx failed, update aborted")
		return
	}

	if nRan != 2 {
		t.Errorf("hive update tx failed invalid number of retries, nRan: %d", nRan)
		return
	}

	// get newly created data to ensure update processed correctly
	updateData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey,
	})

	if err != nil {
		t.Errorf("hive update failiure check, error %+v ", err)
		return
	}

	if _, ok := updateData.Data["s3"]; !ok {
		t.Error("hive update failed missing s3 key data field ")
		return
	}

	if updateData.UsrMtd.Tags == nil {
		t.Errorf("hive update failed tags not set, tags:%s ", []string{"test1", "test2", "test4"})
		return
	}

	if len(updateData.UsrMtd.Tags) != 3 {
		t.Errorf("hive update failed invalid tag length of %d", len(updateData.UsrMtd.Tags))
	}
}

func hiveBatchTest(t *testing.T) {
	// Build a dummy record.
	jsonString := `{
		"s3": {
		  "access_key": "access key",
		  "bucket_name": "bucket_name",
		  "client_options": {
			"hostname": "syslog-test",
			"identity": {
			  "installation_key": "test install key",
			  "oid": "oid-input"
			},
			"platform": "text",
			"sensor_seed_key": "syslog-test"
		  },
		  "prefix": "prefix",
		  "secret_key": "secret key"
		},
		"sensor_type": "s3"
	  }`
	jsonString = strings.ReplaceAll(jsonString, "oid-input", os.Getenv("_OID"))

	data := Dict{}
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		panic(err)
	}

	// Perform the same test as we did for Get and Set but in a batch
	// and check we get the same results.
	batch := testHiveClient.NewBatchOperations()
	batch.SetRecord(RecordID{
		Hive: HiveID{
			Name:      "cloud_sensor",
			Partition: PartitionID(os.Getenv("_OID")),
		},
		Name: "test2",
	}, ConfigRecordMutation{
		Data: data,
		UsrMtd: &UsrMtd{
			Tags:    []string{"test"},
			Enabled: true,
		},
	})
	batch.SetRecord(RecordID{
		Hive: HiveID{
			Name:      "cloud_sensor",
			Partition: PartitionID(os.Getenv("_OID")),
		},
		Name: "test3",
	}, ConfigRecordMutation{
		Data: data,
		UsrMtd: &UsrMtd{
			Tags:    []string{"test3"},
			Enabled: true,
		},
	})

	responses, err := batch.Execute()
	if err != nil {
		t.Errorf("Batch failed: %+v", err)
		return
	}
	if len(responses) != 2 {
		t.Errorf("Batch failed: expected 2 responses, got %d", len(responses))
		return
	}
	if responses[0].Error != "" {
		t.Errorf("Batch 1 failed: %s", responses[0].Error)
		return
	}
	if responses[1].Error != "" {
		t.Errorf("Batch 2 failed: %s", responses[1].Error)
		return
	}

	// Now fetch both records in a batch and also delete them in the same batch.
	// Finally fetch the deleted records to ensure they are gone.
	// Batches are processed in parallel, so we can't do a serial check with deletes etc.
	batch = testHiveClient.NewBatchOperations()
	batch.GetRecord(RecordID{
		Hive: HiveID{
			Name:      "cloud_sensor",
			Partition: PartitionID(os.Getenv("_OID")),
		},
		Name: "test2",
	})
	batch.GetRecord(RecordID{
		Hive: HiveID{
			Name:      "cloud_sensor",
			Partition: PartitionID(os.Getenv("_OID")),
		},
		Name: "test3",
	})

	responses, err = batch.Execute()
	if err != nil {
		t.Errorf("Batch failed: %+v", err)
		return
	}

	// Check the responses.
	if len(responses) != 2 {
		t.Errorf("Batch failed: expected 6 responses, got %d", len(responses))
		return
	}
	if responses[0].Error != "" {
		t.Errorf("Batch 1 failed: %s", responses[0].Error)
		return
	}
	if responses[1].Error != "" {
		t.Errorf("Batch 2 failed: %s", responses[1].Error)
		return
	}
	if responses[0].Data == nil {
		t.Errorf("Batch 1 failed: missing data")
		return
	}
	if responses[1].Data == nil {
		t.Errorf("Batch 2 failed: missing data")
		return
	}
}

func hiveGetPublicByGUIDTest(t *testing.T) {
	// First create a record in the external_adapter hive
	externalAdapterKey := "external-test-" + randSeq(8)
	// Build a dummy record.
	jsonString := `{
		"s3": {
		  "access_key": "access key",
		  "bucket_name": "bucket_name",
		  "client_options": {
			"hostname": "syslog-test",
			"identity": {
			  "installation_key": "test install key",
			  "oid": "oid-input"
			},
			"platform": "text",
			"sensor_seed_key": "syslog-test"
		  },
		  "prefix": "prefix",
		  "secret_key": "secret key"
		},
		"sensor_type": "s3"
	  }`
	jsonString = strings.ReplaceAll(jsonString, "oid-input", os.Getenv("_OID"))

	testData := Dict{}
	if err := json.Unmarshal([]byte(jsonString), &testData); err != nil {
		panic(err)
	}

	// Add the test record to external_adapter hive
	_, err := testHiveClient.Add(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		Key:          externalAdapterKey,
		Data:         testData,
	})

	if err != nil {
		t.Errorf("failed to create test record in external_adapter: %v", err)
		return
	}

	// Get the record to extract its GUID
	hiveData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		Key:          externalAdapterKey})

	if err != nil {
		t.Errorf("failed to get record for GUID test setup: %v", err)
		return
	}

	guid := hiveData.SysMtd.GUID
	if guid == "" {
		t.Error("record has no GUID")
		return
	}

	// Now test the GetPublicByGUID method
	etag := hiveData.SysMtd.Etag
	publicData, err := testHiveClient.GetPublicByGUID(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		ETag:         &etag,
	}, guid)

	if err != nil {
		t.Errorf("GetPublicByGUID failed: %v", err)
		return
	}

	// The record should not have changed and the returned data should be
	// empty but successful to indicate no update.
	if len(publicData.Data) != 0 {
		t.Errorf("GetPublicByGUID failed: expected empty data, got %v", publicData.Data)
		return
	}

	// Test with invalid GUID
	_, err = testHiveClient.GetPublicByGUID(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
	}, "invalid-guid")

	if err == nil {
		t.Error("expected error with invalid GUID, got nil")
		return
	}

	// Test with missing required parameters
	_, err = testHiveClient.GetPublicByGUID(HiveArgs{}, guid)
	if err == nil {
		t.Error("expected error with missing parameters, got nil")
		return
	}

	// Clean up the test record
	_, err = testHiveClient.Remove(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		Key:          externalAdapterKey,
	})

	if err != nil {
		t.Errorf("failed to clean up test record: %v", err)
	}
}
