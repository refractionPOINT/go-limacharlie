package limacharlie

import (
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestArtifactUpload(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// create ingestion key
	resp, _ := org.SetIngestionKeys("__test_key")
	ingestionKey := resp["key"]

	testName := uuid.NewString()
	testData := []byte("thisisatestartifactthisisatestartifactthisisatestartifactthisisatestartifactthisisatestartifactthisisatestartifact")

	// Tweak the artifact part size to make sure we test the multipart upload.
	maxUploadFilePartSize = 15

	a.NoError(org.CreateArtifactFromBytes(testName, testData, "txt", uuid.New().String(), 1, ingestionKey.(string)))

	// delete ingestion key
	resp, _ = org.DelIngestionKeys("__test_key")

	// Download the artifact to make sure it's there.
	r, err := org.ExportArtifact("", time.Now().Add(1*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	a.NoError(err)
	if string(b) != string(testData) {
		t.Fatalf("artifact data mismatch: %s != %s", string(b), string(testData))
	}
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
