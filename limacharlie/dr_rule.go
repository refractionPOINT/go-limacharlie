package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type NewDRRuleOptions struct {
	// Replace rule if it already exists with this name.
	IsReplace bool
	// Rule namespace, defaults to "general".
	Namespace string
	// Rule is enabled.
	IsEnabled bool
	// Number of seconds before rule auto-deletes.
	TTL int64
}

type DRRuleFilter func(map[string]string)

type drAddRuleRequest struct {
	Name      string `json:"name"`
	IsReplace bool   `json:"is_replace"`
	Detection string `json:"detection"`
	Response  string `json:"response"`
	IsEnabled bool   `json:"is_enabled"`
	ExpireOn  int64  `json:"expire_on,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type CoreDRRule struct {
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Detect    Dict   `json:"detect" yaml:"detect"`
	Response  List   `json:"respond" yaml:"respond"`
	IsEnabled bool   `json:"is_enabled"`
}

// DRRuleAdd add a D&R Rule to an LC organization
func (org Organization) DRRuleAdd(name string, detection interface{}, response interface{}, opt ...NewDRRuleOptions) error {
	resp := Dict{}
	reqOpt := NewDRRuleOptions{
		IsEnabled: true,
	}
	for _, o := range opt {
		reqOpt = o
		if reqOpt.TTL != 0 {
			reqOpt.TTL = time.Now().Unix() + reqOpt.TTL
		}
	}

	serialDet, err := json.Marshal(detection)
	if err != nil {
		return err
	}
	serialResp, err := json.Marshal(response)
	if err != nil {
		return err
	}

	request := makeDefaultRequest(&resp).withFormData(drAddRuleRequest{
		Name:      name,
		IsReplace: reqOpt.IsEnabled,
		Detection: string(serialDet),
		Response:  string(serialResp),
		IsEnabled: reqOpt.IsEnabled,
		ExpireOn:  reqOpt.TTL,
		Namespace: reqOpt.Namespace,
	})
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("rules/%s", org.client.options.OID), request); err != nil {
		return err
	}
	return nil
}

// DRRules get all D&R rules for an LC organization
func (org Organization) DRRules(filters ...DRRuleFilter) (map[string]Dict, error) {
	req := map[string]string{}

	for _, f := range filters {
		f(req)
	}

	resp := map[string]Dict{}

	request := makeDefaultRequest(&resp).withQueryData(req)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("rules/%s", org.client.options.OID), request); err != nil {
		return nil, err
	}
	return resp, nil
}

// DRRuleDelete delete a D&R rule from an LC organization
func (org Organization) DRRuleDelete(name string, filters ...DRRuleFilter) error {
	req := map[string]string{
		"name": name,
	}
	for _, f := range filters {
		f(req)
	}

	resp := Dict{}

	request := makeDefaultRequest(&resp).withFormData(req)
	if err := org.client.reliableRequest(http.MethodDelete, fmt.Sprintf("rules/%s", org.client.options.OID), request); err != nil {
		return err
	}
	return nil
}

func (d CoreDRRule) Equal(dr CoreDRRule) bool {
	if !d.IsInSameNamespace(dr) {
		return false
	}
	j1, err := json.Marshal(d.Detect)
	if err != nil {
		return false
	}
	j2, err := json.Marshal(dr.Detect)
	if err != nil {
		return false
	}
	if string(j1) != string(j2) {
		return false
	}
	j1, err = json.Marshal(d.Response)
	if err != nil {
		return false
	}
	j2, err = json.Marshal(dr.Response)
	if err != nil {
		return false
	}
	if string(j1) != string(j2) {
		return false
	}
	return true
}

func (d CoreDRRule) IsInSameNamespace(dr CoreDRRule) bool {
	if d.Namespace == "" {
		d.Namespace = "general"
	}
	if dr.Namespace == "" {
		dr.Namespace = "general"
	}
	return d.Namespace == dr.Namespace
}

func WithNamespace(namespace string) func(map[string]string) {
	return func(m map[string]string) {
		m["namespace"] = namespace
	}
}
