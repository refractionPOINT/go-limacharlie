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

	respPO, err := org.InsightObjectsPerObject(InsightObjectsRequest{
		IndicatorName:  "%google%",
		ObjectType:     InsightObjectTypes.Domain,
		ObjectTypeInfo: InsightObjectTypeInfoTypes.Summary,
		AllowWildcards: true,
	})
	a.NoError(err)
	a.Equal("%google%", respPO.IndicatorName)
	a.Equal(InsightObjectTypes.Domain, respPO.ObjectType)

	_, err = org.InsightObjectsBatch(InsightObjectsBatchRequest{
		Objects: map[InsightObjectType][]string{
			InsightObjectTypes.Domain: {"google.com", "microsoft.com"},
		},
	})
	a.NoError(err)
}

// TestSearchIOCSummary tests the new SearchIOCSummary method
func TestSearchIOCSummary(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test exact match search
	resp, err := org.SearchIOCSummary(IOCSearchParams{
		SearchTerm:    "google.com",
		ObjectType:    InsightObjectTypes.Domain,
		CaseSensitive: false,
	})
	a.NoError(err)
	a.NotNil(resp)
	a.Equal("google.com", resp.Name)
	a.Equal(InsightObjectTypes.Domain, resp.Type)

	// Test wildcard search
	respWildcard, err := org.SearchIOCSummary(IOCSearchParams{
		SearchTerm:    "%google%",
		ObjectType:    InsightObjectTypes.Domain,
		CaseSensitive: false,
	})
	a.NoError(err)
	a.NotNil(respWildcard)
	a.Equal("%google%", respWildcard.Name)
}

// TestSearchIOCLocations tests the new SearchIOCLocations method
func TestSearchIOCLocations(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	resp, err := org.SearchIOCLocations(IOCSearchParams{
		SearchTerm: "google.com",
		ObjectType: InsightObjectTypes.Domain,
		// CaseSensitive is always forced to false for locations
	})
	a.NoError(err)
	a.NotNil(resp)
	a.Equal("google.com", resp.Name)
	a.Equal(InsightObjectTypes.Domain, resp.Type)
	a.NotNil(resp.Locations)
}

// TestSearchHostname tests the new SearchHostname method
func TestSearchHostname(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	results, err := org.SearchHostname("test")
	a.NoError(err)
	a.NotNil(results)
	// Results may be empty, but should not error
}
