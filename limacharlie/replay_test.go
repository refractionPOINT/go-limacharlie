package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplayDRRuleLiteralMatch(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Create a simple rule that matches NEW_PROCESS events containing "test"
	req := ReplayDRRuleRequest{
		Rule: Dict{
			"detect": Dict{
				"event": "NEW_PROCESS",
				"op":    "contains",
				"path":  "event/FILE_PATH",
				"value": "test",
			},
			"respond": List{
				Dict{"action": "report", "name": "test-detection"},
			},
		},
		Events: []Dict{
			{
				"routing": Dict{"event_type": "NEW_PROCESS"},
				"event":   Dict{"FILE_PATH": "/path/to/test/binary"},
			},
		},
	}

	result, err := org.ReplayDRRule(req)
	a.NoError(err)
	a.Empty(result.Error)
	a.True(result.DidMatch)
}
