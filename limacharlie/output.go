package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OutputModuleType is the type of module
type OutputModuleType = string

// OutputTypes is all supported modules
var OutputTypes = struct {
	S3          OutputModuleType
	GCS         OutputModuleType
	SCP         OutputModuleType
	SFTP        OutputModuleType
	Slack       OutputModuleType
	Syslog      OutputModuleType
	Webhook     OutputModuleType
	WebhookBulk OutputModuleType
	SMTP        OutputModuleType
	Humio       OutputModuleType
	Kafka       OutputModuleType
}{
	S3:          "s3",
	GCS:         "gcs",
	SCP:         "scp",
	SFTP:        "sftp",
	Slack:       "slack",
	Syslog:      "syslog",
	Webhook:     "webhook",
	WebhookBulk: "webhook_bulk",
	SMTP:        "smtp",
	Humio:       "humio",
	Kafka:       "kafka",
}

// OutputDataType is the type of data
type OutputDataType = string

// OutputDataTypes is slice of all supported type of data
var OutputDataTypes = []OutputDataType{
	OutputType.Event,
	OutputType.Detect,
	OutputType.Audit,
	OutputType.Deployment,
	OutputType.Artifact,
}

// OutputType is all supported type of data
var OutputType = struct {
	Event      OutputDataType
	Detect     OutputDataType
	Audit      OutputDataType
	Deployment OutputDataType
	Artifact   OutputDataType
}{
	Event:      "event",
	Detect:     "detect",
	Audit:      "audit",
	Deployment: "deployment",
	Artifact:   "artifact",
}

// OutputConfig hold all the possible options used to configure an output
type OutputConfig struct {
	Name   string           `json:"name"`
	Module OutputModuleType `json:"module"`
	Type   OutputDataType   `json:"type"`

	PrefixData        bool   `json:"is_prefix_data,omitempty,string"`
	DeleteOnFailure   bool   `json:"is_delete_on_failure,omitempty,string"`
	InvestigationID   string `json:"inv_id,omitempty"`
	Tag               string `json:"tag,omitempty"`
	Category          string `json:"cat,omitempty"`
	SensorID          string `json:"sid,omitempty"`
	Flat              bool   `json:"is_flat,omitempty,string"`
	Directory         string `json:"dir,omitempty"`
	DestinationHost   string `json:"dest_host,omitempty"`
	SlackToken        string `json:"slack_api_token,omitempty"`
	SlackChannel      string `json:"slack_channel,omitempty"`
	Bucket            string `json:"bucket,omitempty"`
	UserName          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	TLS               bool   `json:"is_tls,omitempty,string"`
	StrictTLS         bool   `json:"is_strict_tls,omitempty,string"`
	NoHeader          bool   `json:"is_no_header,omitempty,string"`
	StructuredData    string `json:"structured_data,omitempty"`
	SecretKey         string `json:"secret_key,omitempty"`
	EventWhiteList    string `json:"event_white_list,omitempty"`
	EventBlackList    string `json:"event_black_list,omitempty"`
	SecondsPerFile    int    `json:"sec_per_file,omitempty,string"`
	DestinationEmail  string `json:"dest_email,omitempty"`
	FromEmail         string `json:"from_email,omitempty"`
	Readable          bool   `json:"is_readable,omitempty,string"`
	Subject           string `json:"subject,omitempty"`
	StartTLS          bool   `json:"is_starttls,omitempty,string"`
	AuthLogin         bool   `json:"is_authlogin,omitempty,string"`
	Indexing          bool   `json:"is_indexing,omitempty,string"`
	Compressing       bool   `json:"is_compression,omitempty,string"`
	CategoryBlackList string `json:"cat_black_list,omitempty"`
	CategoryWhiteList string `json:"cat_white_list,omitempty"`
	RegionName        string `json:"region_name,omitempty"`
	EndpointURL       string `json:"endpoint_url,omitempty"`
	AuthHeaderName    string `json:"auth_header_name,omitempty"`
	AuthHeaderValue   string `json:"auth_header_value,omitempty"`
	RoutingTopic      string `json:"routing_topic,omitempty"`
	LiteralTopic      string `json:"literal_topic,omitempty"`
	HumioRepo         string `json:"humio_repo,omitempty"`
	HumioToken        string `json:"humio_api_token,omitempty"`
}

func (o OutputConfig) Equals(other OutputConfig) bool {
	otherBytes, err := json.Marshal(other)
	if err != nil {
		return false
	}
	bytes, err := json.Marshal(o)
	if err != nil {
		return false
	}
	return string(otherBytes) == string(bytes)
}

// OutputsByName represents OutputConfig where the key is the name of the OutputConfig
type OutputName = string
type OutputsByName = map[OutputName]OutputConfig
type outputsByOrgID = map[string]OutputsByName
type genericOutputsByOrgID = map[string]GenericJSON

// OutputsGeneric fetches all outputs and returns it in outputs
func (org Organization) OutputsGeneric(outputs interface{}) error {
	request := makeDefaultRequest(&outputs).withTimeout(10 * time.Second)
	if err := org.outputs(http.MethodGet, request); err != nil {
		return err
	}
	return nil
}

// backward compat where type is returned in the for field
type outputResponse struct {
	OutputConfig

	Type OutputDataType `json:"for"`
}

// Outputs returns all outputs by name
func (org Organization) Outputs() (OutputsByName, error) {
	outputsByOrg := map[string]map[OutputName]outputResponse{}
	if err := org.OutputsGeneric(&outputsByOrg); err != nil {
		return OutputsByName{}, err
	}

	outByName, ok := outputsByOrg[org.client.options.OID]
	if !ok {
		return OutputsByName{}, ErrorResourceNotFound
	}

	cleanOutByName := OutputsByName{}
	for k, v := range outByName {
		outputConfig := v.OutputConfig
		outputConfig.Type = v.Type

		cleanOutByName[k] = outputConfig
	}

	return cleanOutByName, nil
}

// OutputAdd add an output to the LC organization
func (org Organization) OutputAdd(output OutputConfig) (OutputConfig, error) {
	resp := outputResponse{}
	request := makeDefaultRequest(&resp).withTimeout(10 * time.Second).withFormData(output)
	if err := org.outputs(http.MethodPost, request); err != nil {
		return OutputConfig{}, err
	}

	ret := resp.OutputConfig
	ret.Type = resp.Type
	return ret, nil
}

// OutputDel deletes an output from the LC organization
func (org Organization) OutputDel(name string) (GenericJSON, error) {
	resp := GenericJSON{}
	request := makeDefaultRequest(&resp).withTimeout(10 * time.Second).withFormData(map[string]string{"name": name})
	if err := org.outputs(http.MethodDelete, request); err != nil {
		return nil, err
	}
	return resp, nil
}

func (org Organization) outputs(verb string, request restRequest) error {
	return org.client.reliableRequest(verb, fmt.Sprintf("outputs/%s", org.client.options.OID), request)
}
