package limacharlie

type IntegrityRule struct {
	Patterns []string            `json:"patterns"`
	Filters  IntegrityRuleFilter `json:"filters"`

	CreatedBy   string `json:"by"`
	LastUpdated uint64 `json:"updated"`
}

type IntegrityRuleFilter struct {
	Tags      []string `json:"tags" yaml:"tags"`
	Platforms []string `json:"platforms" yaml:"platforms"`
}

func (ir IntegrityRule) WithPatterns(patterns []string) IntegrityRule {
	ir.Patterns = append(ir.Patterns, patterns...)
	return ir
}

func (ir IntegrityRule) WithTags(tags []string) IntegrityRule {
	ir.Filters.Tags = append(ir.Filters.Tags, tags...)
	return ir
}

func (ir IntegrityRule) WithPlatforms(platforms []string) IntegrityRule {
	ir.Filters.Platforms = append(ir.Filters.Platforms, platforms...)
	return ir
}

type IntegrityRuleName = string
type IntegrityRulesByName = map[IntegrityRuleName]IntegrityRule

func (org Organization) integrity(responseData interface{}, action string, req Dict) error {
	reqData := req
	reqData["action"] = action
	return org.client.serviceRequest(responseData, "integrity", reqData, false)
}

func (org Organization) IntegrityRules() (IntegrityRulesByName, error) {
	resp := IntegrityRulesByName{}
	if err := org.integrity(&resp, "list_rules", Dict{}); err != nil {
		return IntegrityRulesByName{}, err
	}
	return resp, nil
}

func (org Organization) IntegrityRuleAdd(ruleName IntegrityRuleName, rule IntegrityRule) error {
	patterns := rule.Patterns
	if patterns == nil {
		patterns = []string{}
	}
	tags := rule.Filters.Tags
	if tags == nil {
		tags = []string{}
	}
	platforms := rule.Filters.Platforms
	if platforms == nil {
		platforms = []string{}
	}

	req := Dict{
		"name":      ruleName,
		"patterns":  patterns,
		"tags":      tags,
		"platforms": platforms,
	}
	resp := Dict{}
	err := org.integrity(&resp, "add_rule", req)
	return err
}

func (org Organization) IntegrityRuleDelete(ruleName string) error {
	req := Dict{
		"name": ruleName,
	}
	resp := Dict{}
	err := org.integrity(&resp, "remove_rule", req)
	return err
}
