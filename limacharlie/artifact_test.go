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

	artifactID := uuid.New().String()
	a.NoError(org.CreateArtifactFromBytes(testName, testData, "txt", artifactID, 1, ingestionKey.(string)))

	// delete ingestion key
	resp, _ = org.DelIngestionKeys("__test_key")

	// Download the artifact to make sure it's there.
	r, err := org.ExportArtifact(artifactID, time.Now().Add(1*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	a.NoError(err)
	if string(b) != string(testData) {
		t.Fatalf("artifact data mismatch: %s != %s", string(b), string(testData))
	}
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
