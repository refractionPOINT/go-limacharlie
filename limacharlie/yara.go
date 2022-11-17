package limacharlie

type YaraSource struct {
	Author      string `json:"by,omitempty" yaml:"by,omitempty"`
	Source      string `json:"source,omitempty" yaml:"source,omitempty"`
	LastUpdated int64  `json:"updated,omitempty" yaml:"updated,omitempty"`
}

type YaraRule struct {
	Author      string         `json:"by,omitempty" yaml:"by,omitempty"`
	Filters     YaraRuleFilter `json:"filters,omitempty" yaml:"filters,omitempty"`
	Sources     []string       `json:"sources,omitempty" yaml:"sources,omitempty"`
	LastUpdated int64          `json:"updated,omitempty" yaml:"updated,omitempty"`
}

type YaraRuleFilter struct {
	Tags      []string `json:"tags" yaml:"tags"`
	Platforms []string `json:"platforms" yaml:"platforms"`
}

type YaraRuleName = string
type YaraSourceName = string

type YaraSources map[YaraSourceName]YaraSource
type YaraRules map[YaraRuleName]YaraRule

func (org Organization) yara(responseData interface{}, action string, req Dict) error {
	reqData := req
	reqData["action"] = action
	return org.client.serviceRequest(responseData, "yara", reqData, false)
}

func (org Organization) YaraListRules() (YaraRules, error) {
	resp := YaraRules{}
	if err := org.yara(&resp, "list_rules", Dict{}); err != nil {
		return nil, err
	}
	return resp, nil
}

func (org Organization) YaraRuleAdd(ruleName string, rule YaraRule) error {
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
		"sources":   rule.Sources,
		"tags":      tags,
		"platforms": platforms,
	}
	resp := Dict{}
	err := org.yara(&resp, "add_rule", req)
	return err
}

func (org Organization) YaraRuleDelete(ruleName string) error {
	req := Dict{
		"name": ruleName,
	}
	resp := Dict{}
	err := org.yara(&resp, "remove_rule", req)
	return err
}

func (org Organization) YaraListSources() (YaraSources, error) {
	resp := YaraSources{}
	if err := org.yara(&resp, "list_sources", Dict{}); err != nil {
		return nil, err
	}
	return resp, nil
}

func (org Organization) YaraSourceAdd(sourceName string, source YaraSource) error {
	resp := Dict{}
	err := org.yara(&resp, "add_source", Dict{
		"name":   sourceName,
		"source": source.Source,
	})
	return err
}

func (org Organization) YaraSourceDelete(ruleName string) error {
	resp := Dict{}
	err := org.yara(&resp, "remove_source", Dict{
		"name": ruleName,
	})
	return err
}

func (org Organization) YaraGetSource(sourceName string) (string, error) {
	resp := Dict{}
	if err := org.yara(&resp, "get_source", Dict{
		"source": sourceName,
	}); err != nil {
		return "", err
	}
	s, _ := resp["content"].(string)
	return s, nil
}
