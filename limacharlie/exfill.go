package limacharlie

type ExfilRuleName = string

type ExfilRulesType struct {
	Performance Dict                             `json:"perf"`
	Events      map[ExfilRuleName]ExfilRuleEvent `json:"list"`
	Watch       map[ExfilRuleName]ExfilRuleWatch `json:"watch"`
}

type ExfilRuleBase struct {
	LastUpdated uint64 `json:"updated"`
	CreatedBy   string `json:"by"`
}

type ExfilEventFilters struct {
	Tags      []string `json:"tags"`
	Platforms []string `json:"platforms"`
}

type ExfilRuleEvent struct {
	ExfilRuleBase

	Events  []string          `json:"events"`
	Filters ExfilEventFilters `json:"filters"`
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
	data := Dict{
		"name":      name,
		"events":    event.Events,
		"tags":      event.Filters.Tags,
		"platforms": event.Filters.Platforms,
	}
	resp := Dict{}
	return org.exfil(&resp, "add_event_rule", data)
}

func (org Organization) ExfilRuleEventDelete(name ExfilRuleName) error {
	resp := Dict{}
	return org.exfil(&resp, "remove_event_rule", Dict{"name": name})
}

type ExfilRuleWatch struct {
	ExfilRuleBase

	Event    string            `json:"event"`
	Value    string            `json:"value"`
	Path     []string          `json:"path"`
	Operator string            `json:"operator"`
	Filters  ExfilEventFilters `json:"filters"`
}

func (org Organization) ExfilRuleWatchAdd(name ExfilRuleName, watch ExfilRuleWatch) error {
	resp := Dict{}
	return org.exfil(&resp, "add_watch", Dict{
		"name":      name,
		"operator":  watch.Operator,
		"event":     watch.Event,
		"value":     watch.Value,
		"path":      watch.Path,
		"tags":      watch.Filters.Tags,
		"platforms": watch.Filters.Platforms,
	})
}

func (org Organization) ExfilRuleWatchDelete(name ExfilRuleName) error {
	resp := Dict{}
	return org.exfil(&resp, "remove_watch", Dict{"name": name})
}
