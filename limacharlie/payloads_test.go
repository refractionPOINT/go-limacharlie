package limacharlie

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
)

func TestPayloads(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	testName := uuid.NewString()
	testData := []byte("thisisatestpayload")

	// Create a new Payload.
	err := org.CreatePayloadFromBytes(testName, testData)
	a.NoError(err)

	// Verify it is there.
	payloads, err := org.Payloads()
	a.NoError(err)
	entry, ok := payloads[testName]
	a.Equal(ok, true, "failed to find new payload in list: %+v", payloads)
	a.Equal(entry.Name, testName)
	a.Equal(entry.Oid, org.client.options.OID)
	a.Equal(entry.Size, uint64(len(testData)))

	// Try to re-create it to get an error.
	err = org.CreatePayloadFromBytes(testName, testData)
	a.Error(err, "adding a payload with the same name should raise an error: %s", err)

	// Get the payload data.
	data, err := org.Payload(testName)
	a.NoError(err)
	a.NotEqual(data, nil)
	a.Equal(string(data), string(testData))

	// Delete the payload.
	err = org.DeletePayload(testName)
	a.NoError(err)

	// Make sure it is deleted.
	payloads, err = org.Payloads()
	a.NoError(err)
	_, ok = payloads[testName]
	a.Equal(ok, false)
}
