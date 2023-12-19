package limacharlie

import (
	"testing"

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
	for sourceName, _ := range sources {
		_ = org.YaraSourceDelete(sourceName)
	}

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
	for ruleName, _ := range rules {
		_ = org.YaraRuleDelete(ruleName)
	}

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
		Tags:      rule.Filters.Tags,
		Platforms: rule.Filters.Platforms,
	}, ruleFound.Filters)
	a.True(rule.EqualsContent(ruleFound))

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
