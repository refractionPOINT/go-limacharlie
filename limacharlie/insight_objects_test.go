package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsightObjects(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	resp, err := org.InsightObjects(InsightObjectsRequest{
		IndicatorName:  "%google%",
		ObjectType:     InsightObjectTypes.Domain,
		ObjectTypeInfo: InsightObjectTypeInfoTypes.Summary,
		AllowWildcards: true,
	})
	a.NoError(err)
	a.Equal("%google%", resp.IndicatorName)
	a.Equal(InsightObjectTypes.Domain, resp.ObjectType)
}

func TestInsightObjectsPerObject(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	resp, err := org.InsightObjectsPerObject(InsightObjectsRequest{
		IndicatorName:  "%google%",
		ObjectType:     InsightObjectTypes.Domain,
		ObjectTypeInfo: InsightObjectTypeInfoTypes.Summary,
		AllowWildcards: true,
	})
	a.NoError(err)
	a.Equal("%google%", resp.IndicatorName)
	a.Equal(InsightObjectTypes.Domain, resp.ObjectType)
}

func TestInsightObjectsBatch(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	_, err := org.InsightObjectsBatch(InsightObjectsBatchRequest{
		Objects: map[InsightObjectType][]string{
			InsightObjectTypes.Domain: {"google.com", "microsoft.com"},
		},
	})
	a.NoError(err)
}
