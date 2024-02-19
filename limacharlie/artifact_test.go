package limacharlie

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestArtifactUpload(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	var ingestion_key string
	ingestion_keys, _ := org.GetIngestionKeys()
	for k, v := range ingestion_keys {
		if k == "__lc_sensor_artifact" {
			ingestion_key = v.(string)
		}
	}
	testName := uuid.NewString()
	testData := "thisisatestartifact"
	a.NoError(org.CreateArtifact(testName, testData, ingestion_key))
}

func TestLoggingAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	unsubReplicantCB, err := findUnsubscribeReplicantCallback(org, "logging")
	a.NoError(err)
	if unsubReplicantCB != nil {
		defer unsubReplicantCB()
	}

	artifactsRules, err := org.ArtifactsRules()
	a.NoError(err)
	artifactsRulesStartCount := len(artifactsRules)

	a.NoError(org.ArtifactRuleAdd("test-rule", ArtifactRule{
		IsIgnoreCert:   true,
		IsDeleteAfter:  true,
		DaysRetentions: 90,
		Patterns:       []string{"/var/log.log", "/home/user"},
		Filters: ArtifactRuleFilter{
			Tags:      []string{"test-tag0"},
			Platforms: []string{"windows", "chrome"},
		},
	}))

	artifactsRules, err = org.ArtifactsRules()
	a.NoError(err)
	a.Equal(artifactsRulesStartCount+1, len(artifactsRules))

	a.NoError(org.ArtifactRuleDelete("test-rule"))

	artifactsRules, err = org.ArtifactsRules()
	a.NoError(err)
	a.Equal(artifactsRulesStartCount, len(artifactsRules))
}

// func TestArtifactExport(t *testing.T) {
// 	o, err := NewOrganizationFromClientOptions(ClientOptions{
// 		OID: "",
// 		APIKey: "",
// 	}, nil)
// 	if err != nil {
// 		panic(err)
// 	}
// 	r, err := o.ExportArtifact("", time.Now().Add(1*time.Minute))
// 	if err != nil {
// 		panic(err)
// 	}
// 	b, err := io.ReadAll(r)
// 	if err != nil {
// 		panic(err)
// 	}
// 	panic(fmt.Sprintf("%x", sha256.Sum256(b)))
// }
