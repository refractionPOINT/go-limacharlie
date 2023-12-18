package limacharlie

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type unsubscribeReplicantCB = func()

func findUnsubscribeCallback(org *Organization, category string, name string) (unsubscribeReplicantCB, error) {
	// To simplify all tests, we assume no resources are subscribed to
	// and all subscriptions need to be undone after.
	cb := func() {
		org.logger.Info(fmt.Sprintf("cleaning up resource: %s/%s", category, name))
		org.ResourceUnsubscribe(name, category)
		time.Sleep(6 * time.Second)
	}
	org.ResourceSubscribe(name, category)
	time.Sleep(6 * time.Second)

	resources, err := org.Resources()
	if err != nil {
		return nil, nil
	}

	resourceCat, found := resources[category]
	if !found {
		return nil, fmt.Errorf("failed to subscribe to ressource %s/%s", category, name)
	}

	if _, found = resourceCat[name]; !found {
		return nil, fmt.Errorf("failed to subscribe to ressource %s/%s", category, name)
	}

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
