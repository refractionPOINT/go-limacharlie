package limacharlie

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestYaraRuleAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	unsubReplicantCB, err := findUnsubscribeReplicantCallback(org, "yara")
	a.NoError(err)
	if unsubReplicantCB != nil {
		defer unsubReplicantCB()
	}

	sources, err := org.YaraListSources()
	a.NoError(err)
	a.Empty(sources)

	source := YaraSource{
		Source: "https://github.com/Neo23x0/signature-base/blob/master/yara/expl_log4j_cve_2021_44228.yar",
	}
	err = org.YaraSourceAdd("testsource", source)
	a.NoError(err)

	srcData, err := org.YaraGetSource("testsource")
	a.NoError(err)
	a.NotEmpty(srcData)

	rules, err := org.YaraListRules()
	a.NoError(err)
	a.Empty(rules)

	ruleName := "testyararule"
	rule := YaraRule{
		Sources: []string{"testsource"},
		Filters: YaraRuleFilter{
			Tags:      []string{"t1"},
			Platforms: []string{"windows"},
		},
	}
	err = org.YaraRuleAdd(ruleName, rule)
	a.NoError(err)

	rules, err = org.YaraListRules()
	a.NoError(err)
	ruleFound, found := rules[ruleName]
	a.True(found)
	a.NotEmpty(ruleFound.Author)
	a.NotZero(ruleFound.LastUpdated)
	a.Equal(YaraRuleFilter{
		Tags:      []string{},
		Platforms: rule.Filters.Platforms,
	}, ruleFound.Filters)

	err = org.YaraRuleDelete(ruleName)
	a.NoError(err)

	rules, err = org.YaraListRules()
	a.NoError(err)
	a.Empty(rules)

	err = org.YaraSourceDelete("testsource")
	a.NoError(err)

	sources, err = org.YaraListSources()
	a.NoError(err)
	a.Empty(sources)
}
