package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
