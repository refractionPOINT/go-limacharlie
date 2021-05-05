package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type FPRuleOptions struct {
	// Replace rule if it already exists with this name.
	IsReplace bool
}

// FPRules get all false positive rules from a LC organization.
func (org Organization) FPRules() (Dict, error) {
	resp := Dict{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("fp/%s", org.client.options.OID), request); err != nil {
		return nil, err
	}
	return resp, nil
}

type fpAddRuleRequest struct {
	IsReplace bool   `json:"is_replace,string"`
	Name      string `json:"name"`
	Rule      string `json:"rule"`
}

// FPRuleAdd add a false positive rule to a LC organization
func (org Organization) FPRuleAdd(name string, detection interface{}, opts ...FPRuleOptions) error {
	reqOpt := FPRuleOptions{
		IsReplace: false,
	}
	for _, o := range opts {
		reqOpt = o
	}

	ruleBytes, err := json.Marshal(detection)
	if err != nil {
		return err
	}

	resp := Dict{}
	request := makeDefaultRequest(&resp).withFormData(fpAddRuleRequest{
		IsReplace: reqOpt.IsReplace,
		Name:      name,
		Rule:      string(ruleBytes),
	})
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("fp/%s", org.client.options.OID), request); err != nil {
		return err
	}
	return nil
}

// FPRuleDelete delete a false positive rule from a LC organization
func (org Organization) FPRuleDelete(name string) error {
	resp := Dict{}
	request := makeDefaultRequest(&resp).withFormData(Dict{
		"name": name,
	})
	if err := org.client.reliableRequest(http.MethodDelete, fmt.Sprintf("fp/%s", org.client.options.OID), request); err != nil {
		return err
	}
	return nil
}
