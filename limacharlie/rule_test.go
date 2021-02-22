package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDRRuleList(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	rules, err := org.DRRules()
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("unexpected preexisting rules in list: %+v", rules)
	}
}

func TestDRRuleAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	rules, err := org.DRRules()
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("unexpected preexisting rules in add/delete: %+v", rules)
	}

	testRuleName := "testrule"
	testRuleExp := int64(3600)
	testRuleDetect := map[string]interface{}{
		"op":    "is",
		"event": "NEW_PROCESS",
		"path":  "event/nope",
		"value": "never",
	}
	testRuleResponse := []map[string]interface{}{{
		"action": "report",
		"name":   "test",
	}}

	err = org.DRRuleAdd(testRuleName, testRuleDetect, testRuleResponse, NewDRRuleOptions{
		IsEnabled: true,
		TTL:       testRuleExp,
	})
	a.NoError(err)

	rules, err = org.DRRules()
	a.NoError(err)
	if len(rules) == 0 {
		t.Errorf("rules is empty")
	} else if _, ok := rules[testRuleName]; !ok {
		t.Errorf("test rule not found: %+v", rules)
	}

	err = org.DRDelRules(testRuleName)
	a.NoError(err)

	rules, err = org.DRRules()
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("rules is not empty")
	}
}
