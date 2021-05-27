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
