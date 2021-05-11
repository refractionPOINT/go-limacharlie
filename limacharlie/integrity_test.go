package limacharlie

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type unsubscribeReplicantCB = func()

func findUnsubscribeReplicantCallback(org *Organization, replicantName string) (unsubscribeReplicantCB, error) {
	cb := func() {
		org.ResourceUnsubscribe(replicantName, ResourceCategories.Replicant)
	}
	resources, err := org.Resources()
	if err != nil {
		return nil, nil
	}

	resourceCatReplicant, found := resources[ResourceCategories.Replicant]
	if !found {
		org.ResourceSubscribe(replicantName, ResourceCategories.Replicant)
		time.Sleep(5 * time.Second)
		return cb, nil
	}

	if _, found = resourceCatReplicant[replicantName]; found {
		return nil, nil
	}
	return cb, nil

}

func TestIntegrityRuleAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	unsubReplicantCB, err := findUnsubscribeReplicantCallback(org, "integrity")
	a.NoError(err)
	if unsubReplicantCB != nil {
		defer unsubReplicantCB()
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
