package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type NetPolicyName = string
type NetPolicyType string

var NetPolicyTypes = struct {
	Firewall NetPolicyType
	Service  NetPolicyType
	Capture  NetPolicyType
	DNS      NetPolicyType
}{
	Firewall: "firewall",
	Service:  "service",
	Capture:  "capture",
	DNS:      "dns",
}

type NetPolicy struct {
	CreatedBy string        `json:"created_by,omitempty" yaml:"created_by,omitempty"`
	ExpiresOn uint64        `json:"expires_on" yaml:"expires_on"`
	Name      string        `json:"name" yaml:"name"`
	OID       string        `json:"oid" yaml:"oid,omitempty"`
	Type      NetPolicyType `json:"type" yaml:"type"`
	Policy    Dict          `json:"policy" yaml:"policy"`
}

func (n NetPolicy) jsonMarhsalContent() ([]byte, error) {
	n.CreatedBy = ""
	return json.Marshal(n)
}

func (n NetPolicy) EqualsContent(other NetPolicy) bool {
	bytes, err := n.jsonMarhsalContent()
	if err != nil {
		return false
	}
	otherBytes, err := other.jsonMarhsalContent()
	if err != nil {
		return false
	}
	return string(bytes) == string(otherBytes)
}

func (n NetPolicy) WithName(name string) NetPolicy {
	n.Name = name
	return n
}

type NetPolicyFirewallApplicable struct {
	TimeDayStart uint   `json:"time_of_day_start" yaml:"time_of_day_start"`
	TimeDayEnd   uint   `json:"time_of_day_end" yaml:"time_of_day_end"`
	DayWeekStart uint   `json:"day_of_week_start" yaml:"day_of_week_start"`
	DayWeekEnd   uint   `json:"day_of_week_end" yaml:"day_of_week_end"`
	Timezone     string `json:"tz" yaml:"tz"`
}

func (n NetPolicy) WithFirewallPolicy(bpfFilter string, isAllow bool, tag string, applicableTimes []NetPolicyFirewallApplicable, sources []string) NetPolicy {
	n.Type = NetPolicyTypes.Firewall
	n.Policy = Dict{
		"tag":        tag,
		"bpf_filter": bpfFilter,
		"is_allow":   isAllow,
		"times":      applicableTimes,
		"sources":    sources,
	}
	return n
}

type NetPolicyServiceProtocolType uint8

var NetPolicyServiceProtocolTypes = struct {
	TCP NetPolicyServiceProtocolType
	UDP NetPolicyServiceProtocolType
}{
	TCP: 6,
	UDP: 17,
}

func (n NetPolicy) WithServicePolicy(serverPort uint16, serverSid string, clientTag string, protocolType NetPolicyServiceProtocolType) NetPolicy {
	n.Type = NetPolicyTypes.Service
	n.Policy = Dict{
		"server_port": serverPort,
		"server_sid":  serverSid,
		"tag_clients": clientTag,
		"protocol":    protocolType,
	}
	return n
}

type NetPoliciesByName = map[NetPolicyName]NetPolicy
type netPoliciesResponse struct {
	NetPolicies NetPoliciesByName `json:"policies"`
}

func (n NetPolicy) WithCapturePolicy(retentionDays uint64, tag string, bpfFilter string, ingestionKey string) NetPolicy {
	n.Type = NetPolicyTypes.Capture
	n.Policy = Dict{
		"days_retention": retentionDays,
		"tag":            tag,
		"bpf_filter":     bpfFilter,
		"ingest_key":     ingestionKey,
	}
	return n
}

func (n NetPolicy) WithDnsPolicyARecords(domain string, tag string, aRecords []string, includeSubdomains bool) NetPolicy {
	n.Type = NetPolicyTypes.DNS
	n.Policy = Dict{
		"domain":          domain,
		"tag":             tag,
		"to_a":            aRecords,
		"with_subdomains": includeSubdomains,
	}
	return n
}

func (n NetPolicy) WithDnsPolicyCNameRecord(domain string, tag string, cName string, includeSubdomains bool) NetPolicy {
	n.Type = NetPolicyTypes.DNS
	n.Policy = Dict{
		"domain":          domain,
		"tag":             tag,
		"to_cname":        cName,
		"with_subdomains": includeSubdomains,
	}
	return n
}

func (org Organization) netPolicyUrl() string {
	return fmt.Sprintf("net/policy?oid=%s", org.client.options.OID)
}

func (org Organization) NetPolicies() (NetPoliciesByName, error) {
	resp := netPoliciesResponse{}
	req := makeDefaultRequest(&resp).withURLRoot("/")
	if err := org.client.reliableRequest(http.MethodGet, org.netPolicyUrl(), req); err != nil {
		return NetPoliciesByName{}, err
	}
	return resp.NetPolicies, nil
}

func (org Organization) NetPolicyAdd(policy NetPolicy) error {
	resp := Dict{}
	req := makeDefaultRequest(&resp).withURLRoot("/").withFormData(policy)
	return org.client.reliableRequest(http.MethodPost, org.netPolicyUrl(), req)
}

func (org Organization) NetPolicyDelete(name NetPolicyName) error {
	resp := Dict{}
	req := makeDefaultRequest(&resp).withURLRoot("/")
	url := fmt.Sprintf("%s&name=%s", org.netPolicyUrl(), name)
	return org.client.reliableRequest(http.MethodDelete, url, req)
}
