package limacharlie

import "encoding/json"

type YaraSource struct {
	Author      string `json:"by,omitempty" yaml:"by,omitempty"`
	Source      string `json:"source,omitempty" yaml:"source,omitempty"`
	Content     string `json:"content,omitempty" yaml:"content,omitempty"`
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

func (r YaraRule) EqualsContent(r2 YaraRule) bool {
	r.Author = ""
	r.LastUpdated = 0
	if len(r.Sources) == 0 {
		r.Sources = nil
	}
	if len(r.Filters.Platforms) == 0 {
		r.Filters.Platforms = nil
	}
	if len(r.Filters.Tags) == 0 {
		r.Filters.Tags = nil
	}
	r2.Author = ""
	r2.LastUpdated = 0
	if len(r2.Sources) == 0 {
		r2.Sources = nil
	}
	if len(r2.Filters.Platforms) == 0 {
		r2.Filters.Platforms = nil
	}
	if len(r2.Filters.Tags) == 0 {
		r2.Filters.Tags = nil
	}
	d1, err := json.Marshal(r)
	if err != nil {
		return false
	}
	d2, err := json.Marshal(r2)
	if err != nil {
		return false
	}
	return string(d1) == string(d2)
}

func (s YaraSource) EqualsContent(s2 YaraSource) bool {
	return s.Source == s2.Source && s.Content == s2.Content
}

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
	for srcName, src := range resp {
		if src.Source != "<literal rules>" {
			continue
		}
		// Let's fetch the content of the rule since
		// it was not acquired from a remote repo, it just is.
		data, err := org.YaraGetSource(srcName)
		if err != nil {
			return nil, err
		}
		src.Content = data
		src.Source = ""
		resp[srcName] = src
	}
	return resp, nil
}

func (org Organization) YaraSourceAdd(sourceName string, source YaraSource) error {
	resp := Dict{}
	err := org.yara(&resp, "add_source", Dict{
		"name":    sourceName,
		"source":  source.Source,
		"content": source.Content,
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
