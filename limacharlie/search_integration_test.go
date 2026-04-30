package limacharlie

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearch_ExecuteSmoke runs a cheap ExecuteSearch against the live API
// and asserts the flow completes without error. Gated on the same _OID /
// _KEY env vars the rest of the integration tests in this package use
// (see test_fixture.go). Run with creds present; silent skip otherwise
// via getTestClientOpts' FailNow.
func TestSearch_ExecuteSmoke(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	now := time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pages := 0
	err := org.ExecuteSearch(ctx,
		SearchRequest{
			Query:     "* | * | *",
			StartTime: now - 600, // last 10 minutes
			EndTime:   now,
		},
		SearchExecuteOptions{
			PollInterval:    500 * time.Millisecond,
			MaxPollAttempts: 120,
		},
		func(page *SearchPoll) (bool, error) {
			pages++
			// Stop after the first page; one completion is enough proof
			// that init + poll + follow-token wiring works end to end.
			return false, nil
		},
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, pages, 1)
}
