package limacharlie

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFPRuleList(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	_, err := org.FPRules()
	a.NoError(err)
}

func TestFPRuleAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	rules, err := org.FPRules()
	a.NoError(err)
	a.Empty(rules, "unexpected preexisting rules in list: %+v", rules)

	fpRuleName := "testrule" + "-" + randSeq(6)
	err = org.FPRuleAdd(fpRuleName, Dict{
		"op":    "ends with",
		"path":  "detect/event/FILE_PATH",
		"value": "this_is_fine.exe",
	})
	a.NoError(err)

	err = org.FPRuleAdd(fpRuleName, Dict{
		"op":    "ends with",
		"path":  "detect/event/FILE_PATH",
		"value": "this_is_fine_again.exe",
	})
	a.Error(err, "adding a rule with the same name should raise an error: %s", err)

	err = org.FPRuleAdd(fpRuleName, Dict{
		"op":    "ends with",
		"path":  "detect/event/FILE_PATH",
		"value": "this_is_fine_again.exe",
	}, FPRuleOptions{IsReplace: true})
	a.NoError(err, "replacing a rule should not raise an error: %s", err)

	rules, err = org.FPRules()
	a.NoError(err)
	a.GreaterOrEqual(1, len(rules))

	err = org.FPRuleDelete(fpRuleName)
	a.NoError(err)

	rules, err = org.FPRules()
	a.NoError(err)
	if _, ok := rules[fpRuleName]; ok {
		t.Errorf("fp rule with key %s was not deleted  ", fpRuleName)
	}

}
