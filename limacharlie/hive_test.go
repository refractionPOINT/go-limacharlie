package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

var testHiveClient *HiveClient
var testKey string

func TestHiveClient(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testHiveClient = NewHiveClient(org)

	tests := map[string]func(t *testing.T){
		"add":     hiveAddTest,
		"get":     hiveGetTest,
		"getMtd":  hiveGetMtdTest,
		"list":    hiveListTest,
		"listMtd": hiveListMtdTest,
		"update":  hiveUpdate,
		"remove":  hiveRemove,
	}

	// ensure test execute in proper order
	testArray := []string{"add", "get", "getMtd", "list", "listMtd", "update", "remove"}
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
	data := []byte(jsonString)
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

	data := []byte(jsonString)
	_, err := testHiveClient.Update(HiveArgs{
		HiveName:     "cloud_sensor",
		PartitionKey: os.Getenv("_OID"),
		Key:          testKey,
		Data:         data,
		Tags:         []string{"test1", "test2"},
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
