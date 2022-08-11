package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

var testHiveData *HiveData
var testHiveClient *HiveClient
var testKey string

func TestHiveSdk(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	// ensure test execute in proper order
	tests := map[string]func(t *testing.T){
		"add":     hiveAddTest,
		"get":     hiveGetTest,
		"getMtd":  hiveGetMtdTest,
		"list":    hiveListTest,
		"listMtd": hiveListMtdTest,
		"update":  hiveUpdate,
		"remove":  hiveRemove,
	}

	for name, function := range tests {
		t.Run(name, function)
	}
}

func hiveAddTest(t *testing.T) {

	jsonString := `{
				  "pubsub": {
					"client_options": {
					  "hostname": "gcpTest",
					  "identity": {
						"installation_key": "fake key",
						"oid": "oid to input"
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

	testKey = randSeq(8)
	_, err := testHiveClient.Add(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "partition key to add",
		Key:          testKey, Data: []byte(jsonString)}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive client failed: %+v \n", err)
		return
	}
}

func hiveGetTest(t *testing.T) {

	_, err := testHiveClient.Get(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "partition key to add",
		Key:          testKey}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive Get failed: %+v \n", err)
		return
	}
}

func hiveGetMtdTest(t *testing.T) {

	_, err := testHiveClient.GetMTD(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "partition key to add",
		Key:          testKey}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive GetMtd failed: %+v \n", err)
		return
	}
}

func hiveListTest(t *testing.T) {

	_, err := testHiveClient.List(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "partition key to add",
		Key:          testKey}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive List failed: %+v \n", err)
		return
	}
}

func hiveListMtdTest(t *testing.T) {

	_, err := testHiveClient.ListMtd(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "partition key to add",
		Key:          testKey}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive ListMtd failed: %+v \n", err)
		return
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
							"installation_key": "fake-key",
							"oid": "oid to input"
						  },
						  "platform": "text",
						  "sensor_seed_key": "syslog-test"
						},
						"prefix": "prefix",
						"secret_key": "secret key"
					  },
					  "sensor_type": "s3"
				}`

	_, err := testHiveClient.Update(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "8cbe27f4-bfa1-4afb-ba19-138cd51389cd",
		Key:          testKey, Data: []byte(jsonString)}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive client failed: %+v \n", err)
		return
	}

}

func hiveRemove(t *testing.T) {

	_, err := testHiveClient.List(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: "partition key to add",
		Key:          testKey}, false)

	// validate test ran correctly
	if err != nil {
		t.Errorf("hive Remove failed: %+v \n", err)
		return
	}

}

func randSeq(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
