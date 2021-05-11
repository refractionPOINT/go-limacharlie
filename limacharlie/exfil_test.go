package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestExfilTestSuite(t *testing.T) {
	suite.Run(t, new(ExfilTestSuite))
}

type ExfilTestSuite struct {
	suite.Suite

	org              *Organization
	unsubReplicantCB unsubscribeReplicantCB
}

func (s *ExfilTestSuite) SetupSuite() {
	s.org = getTestOrgFromEnv(s.Assert())
	cb, err := findUnsubscribeReplicantCallback(s.org, "exfil")
	s.NoError(err)
	s.unsubReplicantCB = cb
}

func (s *ExfilTestSuite) TearDownSuite() {
	if s.unsubReplicantCB != nil {
		s.unsubReplicantCB()
	}
}

func (s *ExfilTestSuite) TestEventAddDelete() {
	rules, err := s.org.ExfilRules()
	s.NoError(err)
	rulesEventsLenStart := len(rules.Events)

	ruleName := "eventRule0"
	ruleEvent := ExfilRuleEvent{
		Events: []string{"NEW_TCP4_CONNECTION", "NEW_TCP6_CONNECTION"},
		Filters: ExfilEventFilters{
			Tags:      []string{"vip"},
			Platforms: []string{"windows", "linux"},
		},
	}
	s.NoError(s.org.ExfilRuleEventAdd(ruleName, ruleEvent))

	rules, err = s.org.ExfilRules()
	s.NoError(err)
	s.Equal(rulesEventsLenStart+1, len(rules.Events))
	rule, found := rules.Events[ruleName]
	s.True(found)
	s.NotEmpty(rule.CreatedBy)
	s.NotZero(rule.LastUpdated)
	s.Equal(ruleEvent.Events, rule.Events)
	s.Equal(ruleEvent.Filters, rule.Filters)

	err = s.org.ExfilRuleEventDelete(ruleName)
	s.NoError(err)

	rules, err = s.org.ExfilRules()
	s.NoError(err)
	s.Equal(rulesEventsLenStart, len(rules.Events))
}

func (s *ExfilTestSuite) TestWatchAddDelete() {
	rules, err := s.org.ExfilRules()
	s.NoError(err)
	s.Empty(rules.Watch)

	ruleName := "watchRule0"
	ruleWatch := ExfilRuleWatch{
		Event:    "MODULE_LOAD",
		Operator: "ends with",
		Value:    "wininet.dll",
		Path:     []string{"FILE_PATH"},
		Filters: ExfilEventFilters{
			Tags:      []string{"server"},
			Platforms: []string{"windows"},
		},
	}
	s.NoError(s.org.ExfilRuleWatchAdd(ruleName, ruleWatch))

	rules, err = s.org.ExfilRules()
	s.NoError(err)
	s.NotEmpty(rules.Watch)
	rule, found := rules.Watch[ruleName]
	s.True(found)
	s.NotEmpty(rule.CreatedBy)
	s.NotZero(rule.LastUpdated)
	s.Equal(ruleWatch.Event, rule.Event)
	s.Equal(ruleWatch.Path, rule.Path)
	s.Equal(ruleWatch.Operator, rule.Operator)
	s.Equal(ruleWatch.Filters, rule.Filters)

	err = s.org.ExfilRuleWatchDelete(ruleName)
	s.NoError(err)

	rules, err = s.org.ExfilRules()
	s.NoError(err)
	s.Empty(rules.Watch)
}
