package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntegrityRuleAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	rules, err := org.IntegrityRules()
	a.NoError(err)
	a.Empty(rules)

	ruleName := "testintegrityrule"
	rule := IntegrityRule{}.
		WithPatterns([]string{"c:\\test.txt"}).
		WithPlatforms([]string{"windows"})
	err = org.IntegrityRuleAdd(ruleName, rule)
	a.NoError(err)

	rules, err = org.IntegrityRules()
	a.NoError(err)
	ruleFound, found := rules[ruleName]
	a.True(found)
	a.NotEmpty(ruleFound.CreatedBy)
	a.NotZero(ruleFound.LastUpdated)
	a.Equal(rule.Patterns, ruleFound.Patterns)
	a.Equal(IntegrityRuleFilter{
		Tags:      []string{},
		Platforms: rule.Filters.Platforms,
	}, ruleFound.Filters)

	err = org.IntegrityRuleDelete(ruleName)
	a.NoError(err)

	rules, err = org.IntegrityRules()
	a.NoError(err)
	a.Empty(rules)
}
