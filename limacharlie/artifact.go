package limacharlie

type ArtifactRuleName = string
type ArtifactRule struct {
	By          string `json:"by"`
	LastUpdated uint64 `json:"updated"`

	IsIgnoreCert   bool               `json:"is_ignore_cert"`
	IsDeleteAfter  bool               `json:"is_delete_after"`
	DaysRetentions uint               `json:"days_retention"`
	Patterns       []string           `json:"patterns"`
	Filters        ArtifactRuleFilter `json:"filters"`
}

type ArtifactRuleFilter struct {
	Tags      []string `json:"tags"`
	Platforms []string `json:"platforms"`
}
type ArtifactRulesByName = map[ArtifactRuleName]ArtifactRule

func (org Organization) artifact(responseData interface{}, action string, req Dict) error {
	reqData := req
	reqData["action"] = action
	return org.client.serviceRequest(responseData, "logging", reqData, false)
}

func (org Organization) ArtifactsRules() (ArtifactRulesByName, error) {
	resp := ArtifactRulesByName{}
	if err := org.artifact(&resp, "list_rules", Dict{}); err != nil {
		return ArtifactRulesByName{}, err
	}
	return resp, nil
}

func (org Organization) ArtifactRuleAdd(ruleName ArtifactRuleName, rule ArtifactRule) error {
	resp := Dict{}
	if err := org.artifact(&resp, "add_rule", Dict{
		"name":            ruleName,
		"patterns":        rule.Patterns,
		"is_delete_after": rule.IsDeleteAfter,
		"is_ignore_cert":  rule.IsIgnoreCert,
		"days_retention":  rule.DaysRetentions,
		"tags":            rule.Filters.Tags,
		"platforms":       rule.Filters.Platforms,
	}); err != nil {
		return err
	}
	return nil
}

func (org Organization) ArtifactRuleDelete(ruleName ArtifactRuleName) error {
	resp := Dict{}
	if err := org.artifact(&resp, "remove_rule", Dict{"name": ruleName}); err != nil {
		return err
	}
	return nil
}
