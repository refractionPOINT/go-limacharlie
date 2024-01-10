package limacharlie

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

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

type artifactExportResp struct {
	Payload string `json:"payload,omitempty"`
	Export  string `json:"export,omitempty"`
}

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

func (org Organization) ExportArtifact(artifactID string, deadline time.Time, optParams *interface{}) (io.ReadCloser, error) {
	resp := artifactExportResp{}
	var request restRequest
	if optParams != nil {
		request = makeDefaultRequest(&resp).withQueryData(optParams)
	} else {
		request = makeDefaultRequest(&resp)
	}
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/artifacts/originals/%s", org.GetOID(), artifactID), request); err != nil {
		return nil, err
	}
	if resp.Payload != "" {
		b64Dec, err := base64.StdEncoding.DecodeString(resp.Payload)
		if err != nil {
			return nil, err
		}
		gzR, err := gzip.NewReader(bytes.NewBuffer(b64Dec))
		if err != nil {
			return nil, err
		}

		return gzR, nil
	}
	c := http.Client{
		Timeout: 30 * time.Second,
	}
	defer c.CloseIdleConnections()

	var httpResp *http.Response
	for !time.Now().After(deadline) {
		req, err := http.NewRequest(http.MethodGet, resp.Export, &bytes.Buffer{})
		if err != nil {
			return nil, err
		}
		httpResp, err = c.Do(req)
		if err != nil {
			return nil, err
		}
		if httpResp.StatusCode == 200 {
			break
		}
		httpResp.Body.Close()
		if httpResp.StatusCode == 404 {
			time.Sleep(5 * time.Second)
			continue
		}
		return nil, fmt.Errorf("unexpected status: %d", httpResp.StatusCode)
	}

	return httpResp.Body, nil
}
