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

	a := assert.New(t)
	a.NoError(err)
	a.Equal("cloud_sensor", hiveResp.Hive.Name)
	a.Equal(testKey, hiveResp.Name)
}

func hiveGetTest(t *testing.T) {
	a := assert.New(t)
	hiveData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	a.NoError(err)
	a.NotNil(hiveData.Data)
	a.False(hiveData.UsrMtd.Enabled)
	a.Equal(int64(0), hiveData.UsrMtd.Expiry)
	a.Nil(hiveData.UsrMtd.Tags)
	a.Empty(hiveData.UsrMtd.Comment)
}

func hiveGetMtdTest(t *testing.T) {
	a := assert.New(t)
	hiveData, err := testHiveClient.GetMTD(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	a.NoError(err)
	a.Nil(hiveData.Data)
}

func hiveListTest(t *testing.T) {
	a := assert.New(t)
	hiveSet, err := testHiveClient.List(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID")})

	a.NoError(err)
	a.Contains(hiveSet, testKey)
	a.NotNil(hiveSet[testKey].Data)
}

func hiveListMtdTest(t *testing.T) {
	a := assert.New(t)
	hiveSet, err := testHiveClient.ListMtd(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID")})

	a.NoError(err)
	a.Contains(hiveSet, testKey)
	a.Nil(hiveSet[testKey].Data)
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

	a := assert.New(t)
	a.NoError(err)

	// get newly created data to ensure update processed correctly
	updateData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	a.NoError(err)
	a.Contains(updateData.Data, "s3")
	a.NotNil(updateData.UsrMtd.Tags)
	a.Equal(2, len(updateData.UsrMtd.Tags))
	a.Equal(testComment, updateData.UsrMtd.Comment)
}

func hiveRemove(t *testing.T) {
	a := assert.New(t)
	// test remove and clean up test data
	_, err := testHiveClient.Remove(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey})

	a.NoError(err)
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

	a := assert.New(t)
	a.NoError(err)
	a.NotNil(r)
	a.Equal(2, nRan)

	// get newly created data to ensure update processed correctly
	updateData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey,
	})

	a.NoError(err)
	a.Contains(updateData.Data, "s3")
	a.NotNil(updateData.UsrMtd.Tags)
	a.Equal(3, len(updateData.UsrMtd.Tags))
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
	a := assert.New(t)
	a.NoError(err)
	a.Equal(2, len(responses))
	a.Empty(responses[0].Error)
	a.Empty(responses[1].Error)

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
	a.NoError(err)

	// Check the responses.
	a.Equal(2, len(responses))
	a.Empty(responses[0].Error)
	a.Empty(responses[1].Error)
	a.NotNil(responses[0].Data)
	a.NotNil(responses[1].Data)
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

	a := assert.New(t)
	a.NoError(err)

	// Get the record to extract its GUID
	hiveData, err := testHiveClient.Get(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		Key:          externalAdapterKey})

	a.NoError(err)

	guid := hiveData.SysMtd.GUID
	a.NotEmpty(guid)

	// Now test the GetPublicByGUID method
	etag := hiveData.SysMtd.Etag
	publicData, err := testHiveClient.GetPublicByGUID(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		ETag:         &etag,
	}, guid)

	a.NoError(err)

	// The record should not have changed and the returned data should be
	// empty but successful to indicate no update.
	a.Equal(0, len(publicData.Data))

	// Test with invalid GUID
	_, err = testHiveClient.GetPublicByGUID(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
	}, "invalid-guid")

	a.Error(err)

	// Test with missing required parameters
	_, err = testHiveClient.GetPublicByGUID(HiveArgs{}, guid)
	a.Error(err)

	// Clean up the test record
	_, err = testHiveClient.Remove(HiveArgs{
		HiveName:     "external_adapter",
		PartitionKey: os.Getenv("_OID"),
		Key:          externalAdapterKey,
	})

	a.NoError(err)
}
