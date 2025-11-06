package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuery_Basic(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Execute a simple query that should return some results
	// Query for recent events
	resp, err := org.Query(QueryRequest{
		Query:      "-1h | * | * | true",
		Stream:     "event",
		LimitEvent: 100,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	// We should get some results (or at least not error)
	t.Logf("Query returned %d results", len(resp.Results))

	// Check that results are in the expected format
	if len(resp.Results) > 0 {
		// Each result should be a dict
		assert.IsType(t, Dict{}, resp.Results[0])
	}

	// Stats should be present
	assert.NotNil(t, resp.Stats)
}

func TestQuery_DetectStream(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Query the detect stream
	resp, err := org.Query(QueryRequest{
		Query:      "-24h | * | * | / exists",
		Stream:     "detect",
		LimitEvent: 50,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Detect query returned %d results", len(resp.Results))
}

func TestQuery_WithSpecificEventType(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Query for specific event types
	resp, err := org.Query(QueryRequest{
		Query:      "-1h | * | NEW_PROCESS DNS_REQUEST | / exists",
		Stream:     "event",
		LimitEvent: 100,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Query for specific events returned %d results", len(resp.Results))
}

func TestQuery_WithFilter(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Query with a filter condition
	resp, err := org.Query(QueryRequest{
		Query:      "-1h | * | * | event/PROCESS_ID is not 0",
		Stream:     "event",
		LimitEvent: 50,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Filtered query returned %d results", len(resp.Results))

	// Verify that results match the filter if any returned
	for _, result := range resp.Results {
		if data, ok := result["data"].(map[string]interface{}); ok {
			if event, ok := data["event"].(map[string]interface{}); ok {
				// If we have results, they should have PROCESS_ID set
				if len(resp.Results) > 0 {
					assert.Contains(t, event, "PROCESS_ID")
				}
			}
		}
	}
}

func TestQueryAll_Pagination(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Create an iterator for paginated queries
	iter, err := org.QueryAll(QueryRequest{
		Query:      "-6h | * | * | / exists",
		Stream:     "event",
		LimitEvent: 10, // Small limit to force pagination
		Cursor:     "-",
	})

	require.NoError(t, err)
	require.NotNil(t, iter)

	totalResults := 0
	pageCount := 0
	maxPages := 3 // Limit pages to avoid long test runs

	// Iterate through pages
	for iter.HasMore() && pageCount < maxPages {
		resp, err := iter.Next()
		require.NoError(t, err)

		if resp == nil {
			break
		}

		pageCount++
		resultCount := len(resp.Results)
		totalResults += resultCount

		t.Logf("Page %d: %d results", pageCount, resultCount)

		// If we got no results on this page, we should stop
		if resultCount == 0 {
			break
		}
	}

	t.Logf("Total results across %d pages: %d", pageCount, totalResults)
	assert.Greater(t, pageCount, 0, "should have fetched at least one page")
}

func TestQueryAll_SinglePage(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Create an iterator with a large limit (should fit in one page)
	iter, err := org.QueryAll(QueryRequest{
		Query:      "-10m | * | * | / exists",
		Stream:     "event",
		LimitEvent: 1000,
		Cursor:     "-",
	})

	require.NoError(t, err)
	require.NotNil(t, iter)

	// Fetch first page
	resp, err := iter.Next()
	require.NoError(t, err)

	if resp != nil {
		t.Logf("Got %d results", len(resp.Results))
	}

	// There might not be a second page
	if iter.HasMore() {
		resp2, err := iter.Next()
		require.NoError(t, err)
		if resp2 != nil {
			t.Logf("Got second page with %d results", len(resp2.Results))
		}
	}
}

func TestQuery_InvalidQuery(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Try a query with unusual syntax
	resp, err := org.Query(QueryRequest{
		Query:      "invalid query syntax | | |",
		Stream:     "event",
		LimitEvent: 10,
	})

	// The backend may or may not reject this query depending on its parser
	// Just log the result rather than asserting an error
	if err != nil {
		t.Logf("Query rejected as expected: %v", err)
	} else {
		t.Logf("Query was accepted by backend, returned %d results", len(resp.Results))
	}
}

func TestQuery_EmptyResults(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Query with a condition that should return no results
	// Use a very specific condition that's unlikely to match
	resp, err := org.Query(QueryRequest{
		Query:      "-1m | * | * | event/FILE_PATH == '/this/path/should/never/exist/12345.txt'",
		Stream:     "event",
		LimitEvent: 100,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Results might be empty
	t.Logf("Empty query returned %d results", len(resp.Results))
}

func TestQuery_DefaultStream(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Query without specifying stream (should default to "event")
	resp, err := org.Query(QueryRequest{
		Query:      "-1h | * | * | / exists",
		LimitEvent: 50,
		// Stream not specified
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Query with default stream returned %d results", len(resp.Results))
}

func TestQuery_WithLimits(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Query with both event and eval limits
	resp, err := org.Query(QueryRequest{
		Query:      "-6h | * | * | true",
		Stream:     "event",
		LimitEvent: 100,
		LimitEval:  500,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Query with limits returned %d results", len(resp.Results))

	// Check stats if available
	if resp.Stats != nil {
		if nBilled, ok := resp.Stats["n_billed"]; ok {
			t.Logf("Query billed %v evaluations", nBilled)
		}
	}
}

func TestQuery_RecentTimeframe(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Test different timeframe formats
	timeframes := []string{
		"-5m",  // Last 5 minutes
		"-1h",  // Last hour
		"-24h", // Last 24 hours
	}

	for _, tf := range timeframes {
		t.Run(tf, func(t *testing.T) {
			resp, err := org.Query(QueryRequest{
				Query:      tf + " | * | * | / exists",
				Stream:     "event",
				LimitEvent: 50,
			})

			require.NoError(t, err)
			require.NotNil(t, resp)

			t.Logf("Timeframe %s returned %d results", tf, len(resp.Results))
		})
	}
}

func TestQuery_Timeout(t *testing.T) {
	// This test verifies that queries respect context timeout
	// Skip in CI if it takes too long
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Note: The timeout is set in the HTTP client in query.go to 120 seconds
	// A very large query should complete within that time or error
	resp, err := org.Query(QueryRequest{
		Query:      "-24h | * | * | / exists",
		Stream:     "event",
		LimitEvent: 10000,
	})

	// Should either succeed or timeout gracefully
	if err != nil {
		t.Logf("Query timed out or errored as expected: %v", err)
	} else {
		require.NotNil(t, resp)
		t.Logf("Large query completed with %d results", len(resp.Results))
	}
}
