package limacharlie

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type unsubscribeReplicantCB = func()

func findUnsubscribeCallback(org *Organization, category string, name string) (unsubscribeReplicantCB, error) {
	cb := func() {
		org.ResourceUnsubscribe(name, category)
	}
	resources, err := org.Resources()
	if err != nil {
		return nil, nil
	}

	resourceCatReplicant, found := resources[category]
	if !found {
		org.ResourceSubscribe(name, category)
		time.Sleep(5 * time.Second)
		return cb, nil
	}

	if _, found = resourceCatReplicant[name]; found {
		return nil, nil
	}
	org.ResourceSubscribe(name, category)
	time.Sleep(5 * time.Second)
	return cb, nil
}

func findUnsubscribeReplicantCallback(org *Organization, replicantName string) (unsubscribeReplicantCB, error) {
	return findUnsubscribeCallback(org, ResourceCategories.Replicant, replicantName)
}

func findUnsubscribeApiCallback(org *Organization, apiName string) (unsubscribeReplicantCB, error) {
	return findUnsubscribeCallback(org, ResourceCategories.API, apiName)
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
