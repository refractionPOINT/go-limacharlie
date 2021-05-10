package limacharlie

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntegrityRuleAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	resources, err := org.Resources()
	a.NoError(err)
	_, found := resources[ResourceCategories.Replicant]
	if !found {
		org.ResourceSubscribe("integrity", ResourceCategories.Replicant)
		time.Sleep(5 * time.Second)
		defer org.ResourceUnsubscribe("integrity", ResourceCategories.Replicant)
	}

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
