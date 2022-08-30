package limacharlie

import (
	"fmt"
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

	orgInfo, err := org.GetInfo()
	if err != nil {
		fmt.Println("error getting orgInfo ", err)
	} else {
		fmt.Println("this is orgInfo in drrule and delete")
		fmt.Println(orgInfo.OID)
		fmt.Println(orgInfo.Name)
	}

	testRuleName := "testrule" + randSeq(6)
	testRuleExp := int64(1773563700000)
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

	rules, err = org.DRRules(WithNamespace("general"))
	a.NoError(err)
	if len(rules) == 0 {
		t.Errorf("rules is empty")
	} else if _, ok := rules[testRuleName]; !ok {
		t.Errorf("test rule not found: %+v", rules)
	}

	err = org.DRRuleDelete(testRuleName)
	a.NoError(err)

	rules, err = org.DRRules()
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("rules is not empty")
	}
}
