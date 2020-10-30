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
	Flat              bool   `json:"is_flat,omitempty"`
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
	SecondsPerFile    int    `json:"sec_per_file,omitempty"`
	DestinationEmail  string `json:"dest_email,omitempty"`
	FromEmail         string `json:"from_email,omitempty"`
	Readable          bool   `json:"is_readable,omitempty"`
	Subject           string `json:"subject,omitempty"`
	StartTLS          bool   `json:"is_starttls,omitempty"`
	Indexing          bool   `json:"is_indexing,omitempty"`
	Compressing       bool   `json:"is_compression,omitempty"`
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

// OutputsByName represents OutputConfig where the key is the name of the OutputConfig
type OutputsByName = map[string]OutputConfig
type genericOutputsByOrgID = map[string]GenericJSON

// OutputsGeneric fetches all outputs and returns it in outputs
func (org Organization) OutputsGeneric(outputs interface{}) error {
	outputsByOrgID := genericOutputsByOrgID{}
	request := makeDefaultRequest(&outputsByOrgID).withTimeout(10 * time.Second)
	if err := org.client.outputs(http.MethodGet, request); err != nil {
		return err
	}

	orgOutputs, ok := outputsByOrgID[org.client.options.OID]
	if !ok {
		return ResourceNotFoundError
	}

	switch t := outputs.(type) {
	case *GenericJSON:
		*(outputs.(*GenericJSON)) = orgOutputs
		return nil
	case *OutputsByName:
		all := OutputsByName{}
		for k, v := range orgOutputs {
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("cannot marshal to json: %s", err)
			}
			c := OutputConfig{}
			if err := json.Unmarshal(jsonBytes, &c); err != nil {
				return fmt.Errorf("cannot unmarshal to OutputConfig: %s", err)
			}
			all[k] = c
		}
		*(outputs.(*OutputsByName)) = all
		return nil
	default:
		return fmt.Errorf("unsupported type, expected pointer, got %t", t)
	}
}

// Outputs returns all outputs by name
func (org Organization) Outputs() (OutputsByName, error) {
	outByName := OutputsByName{}
	if err := org.OutputsGeneric(&outByName); err != nil {
		return OutputsByName{}, err
	}
	return outByName, nil
}

// OutputAdd add an output to the LC organization
func (org Organization) OutputAdd(output OutputConfig) (OutputConfig, error) {
	resp := OutputConfig{}
	request := makeDefaultRequest(&resp).withTimeout(10 * time.Second).withFormData(output)
	if err := org.client.outputs(http.MethodPost, request); err != nil {
		return OutputConfig{}, err
	}
	return resp, nil
}

// OutputDel deletes an output from the LC organization
func (org Organization) OutputDel(name string) (GenericJSON, error) {
	resp := GenericJSON{}
	request := makeDefaultRequest(&resp).withTimeout(10 * time.Second).withFormData(map[string]string{"name": name})
	if err := org.client.outputs(http.MethodDelete, request); err != nil {
		return nil, err
	}
	return resp, nil
}
