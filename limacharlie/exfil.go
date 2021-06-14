package limacharlie

import "encoding/json"

type ExfilRuleName = string

type ExfilRulesType struct {
	Performance Dict                             `json:"perf,omitempty" yaml:"perf,omitempty"`
	Events      map[ExfilRuleName]ExfilRuleEvent `json:"list,omitempty" yaml:"list,omitempty"`
	Watches     map[ExfilRuleName]ExfilRuleWatch `json:"watch,omitempty" yaml:"watch,omitempty"`
}

type ExfilEventFilters struct {
	Tags      []string `json:"tags" yaml:"tags"`
	Platforms []string `json:"platforms" yaml:"platforms"`
}

type ExfilRuleEvent struct {
	LastUpdated uint64 `json:"updated,omitempty" yaml:"updated,omitempty"`
	CreatedBy   string `json:"by,omitempty" yaml:"by,omitempty"`

	Events  []string          `json:"events" yaml:"events"`
	Filters ExfilEventFilters `json:"filters" yaml:"filters"`
}

func (r ExfilRuleEvent) jsonMarhsalContent() ([]byte, error) {
	if r.Filters.Platforms == nil {
		r.Filters.Platforms = []string{}
	}
	if r.Filters.Tags == nil {
		r.Filters.Tags = []string{}
	}
	return json.Marshal(Dict{
		"events":  r.Events,
		"filters": r.Filters,
	})
}

func (r ExfilRuleEvent) EqualsContent(other ExfilRuleEvent) bool {
	bytes, err := r.jsonMarhsalContent()
	if err != nil {
		return false
	}
	otherBytes, err := other.jsonMarhsalContent()
	if err != nil {
		return false
	}
	return string(bytes) == string(otherBytes)
}

func (org Organization) exfil(responseData interface{}, action string, req Dict) error {
	reqData := req
	reqData["action"] = action
	return org.client.serviceRequest(responseData, "exfil", reqData, false)
}

func (org Organization) ExfilRules() (ExfilRulesType, error) {
	rules := ExfilRulesType{}
	if err := org.exfil(&rules, "list_rules", Dict{}); err != nil {
		return ExfilRulesType{}, err
	}
	return rules, nil
}

func (org Organization) ExfilRuleEventAdd(name ExfilRuleName, event ExfilRuleEvent) error {
	tags := event.Filters.Tags
	if tags == nil {
		tags = []string{}
	}
	platforms := event.Filters.Platforms
	if platforms == nil {
		platforms = []string{}
	}
	data := Dict{
		"name":      name,
		"events":    event.Events,
		"tags":      tags,
		"platforms": platforms,
	}
	resp := Dict{}
	return org.exfil(&resp, "add_event_rule", data)
}

func (org Organization) ExfilRuleEventDelete(name ExfilRuleName) error {
	resp := Dict{}
	return org.exfil(&resp, "remove_event_rule", Dict{"name": name})
}

type ExfilRuleWatch struct {
	LastUpdated uint64 `json:"updated,omitempty" yaml:"updated,omitempty"`
	CreatedBy   string `json:"by,omitempty" yaml:"by,omitempty"`

	Event    string            `json:"event" yaml:"event"`
	Value    string            `json:"value" yaml:"value"`
	Path     []string          `json:"path" yaml:"path"`
	Operator string            `json:"operator" yaml:"operator"`
	Filters  ExfilEventFilters `json:"filters" yaml:"filters"`
}

func (r ExfilRuleWatch) jsonMarhsalContent() ([]byte, error) {
	if r.Path == nil {
		r.Path = []string{}
	}
	if r.Filters.Platforms == nil {
		r.Filters.Platforms = []string{}
	}
	if r.Filters.Tags == nil {
		r.Filters.Tags = []string{}
	}
	return json.Marshal(Dict{
		"event":    r.Event,
		"value":    r.Value,
		"path":     r.Path,
		"operator": r.Operator,
		"filters":  r.Filters,
	})
}

func (r ExfilRuleWatch) EqualsContent(other ExfilRuleWatch) bool {
	bytes, err := r.jsonMarhsalContent()
	if err != nil {
		return false
	}
	otherBytes, err := other.jsonMarhsalContent()
	if err != nil {
		return false
	}
	return string(bytes) == string(otherBytes)
}

func (org Organization) ExfilRuleWatchAdd(name ExfilRuleName, watch ExfilRuleWatch) error {
	tags := watch.Filters.Tags
	if tags == nil {
		tags = []string{}
	}
	platforms := watch.Filters.Platforms
	if platforms == nil {
		platforms = []string{}
	}
	resp := Dict{}
	return org.exfil(&resp, "add_watch", Dict{
		"name":      name,
		"operator":  watch.Operator,
		"event":     watch.Event,
		"value":     watch.Value,
		"path":      watch.Path,
		"tags":      tags,
		"platforms": platforms,
	})
}

func (org Organization) ExfilRuleWatchDelete(name ExfilRuleName) error {
	resp := Dict{}
	return org.exfil(&resp, "remove_watch", Dict{"name": name})
}
