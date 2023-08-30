package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OutputModuleType is the type of module
type OutputModuleType = string

// OutputTypes is all supported modules
var OutputTypes = struct {
	S3               OutputModuleType
	GCS              OutputModuleType
	Pubsub           OutputModuleType
	BigQuery         OutputModuleType
	SCP              OutputModuleType
	SFTP             OutputModuleType
	Slack            OutputModuleType
	Syslog           OutputModuleType
	Webhook          OutputModuleType
	WebhookBulk      OutputModuleType
	SMTP             OutputModuleType
	Humio            OutputModuleType
	Kafka            OutputModuleType
	AzureStorageBlob OutputModuleType
	AzureEventHub    OutputModuleType
	Elastic          OutputModuleType
	Tines            OutputModuleType
	Torq             OutputModuleType
	DataDog          OutputModuleType
}{
	S3:               "s3",
	GCS:              "gcs",
	Pubsub:           "pubsub",
	BigQuery:         "bigquery",
	SCP:              "scp",
	SFTP:             "sftp",
	Slack:            "slack",
	Syslog:           "syslog",
	Webhook:          "webhook",
	WebhookBulk:      "webhook_bulk",
	SMTP:             "smtp",
	Humio:            "humio",
	Kafka:            "kafka",
	AzureStorageBlob: "azure_storage_blog",
	AzureEventHub:    "azure_event_hub",
	Elastic:          "elastic",
	Tines:            "tines",
	Torq:             "torq",
	DataDog:          "datadog",
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
	Tailored   OutputDataType
}{
	Event:      "event",
	Detect:     "detect",
	Audit:      "audit",
	Deployment: "deployment",
	Artifact:   "artifact",
	Tailored:   "tailored",
}

// OutputConfig hold all the possible options used to configure an output
type OutputConfig struct {
	Name   string           `json:"name,omitempty"`
	Module OutputModuleType `json:"module"`
	Type   OutputDataType   `json:"type"`

	PrefixData        bool   `json:"is_prefix_data,omitempty,string" yaml:"is_prefix_data,omitempty"`
	DeleteOnFailure   bool   `json:"is_delete_on_failure,omitempty,string" yaml:"is_delete_on_failure,omitempty"`
	NoRouting         bool   `json:"is_no_routing,omitempty,string" yaml:"is_no_routing,omitempty"`
	NoSharding        bool   `json:"is_no_sharding,omitempty,string" yaml:"is_no_sharding,omitempty"`
	PayloadAsString   bool   `json:"is_payload_as_string,omitempty,string" yaml:"is_payload_as_string,omitempty"`
	InvestigationID   string `json:"inv_id,omitempty" yaml:"inv_id,omitempty"`
	Tag               string `json:"tag,omitempty" yaml:"tag,omitempty"`
	Category          string `json:"cat,omitempty" yaml:"cat,omitempty"`
	SensorID          string `json:"sid,omitempty" yaml:"sid,omitempty"`
	Flat              bool   `json:"is_flat,omitempty,string" yaml:"is_flat,omitempty"`
	Directory         string `json:"dir,omitempty" yaml:"dir,omitempty"`
	DestinationHost   string `json:"dest_host,omitempty" yaml:"dest_host,omitempty"`
	SlackToken        string `json:"slack_api_token,omitempty" yaml:"slack_api_token,omitempty"`
	SlackChannel      string `json:"slack_channel,omitempty" yaml:"slack_channel,omitempty"`
	Bucket            string `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	UserName          string `json:"username,omitempty" yaml:"username,omitempty"`
	Password          string `json:"password,omitempty" yaml:"password,omitempty"`
	TLS               bool   `json:"is_tls,omitempty,string" yaml:"is_tls,omitempty"`
	StrictTLS         bool   `json:"is_strict_tls,omitempty,string" yaml:"is_strict_tls,omitempty"`
	NoHeader          bool   `json:"is_no_header,omitempty,string" yaml:"is_no_header,omitempty"`
	StructuredData    string `json:"structured_data,omitempty" yaml:"structured_data,omitempty"`
	SecretKey         string `json:"secret_key,omitempty" yaml:"secret_key,omitempty"`
	EventWhiteList    string `json:"event_white_list,omitempty" yaml:"event_white_list,omitempty"`
	EventBlackList    string `json:"event_black_list,omitempty" yaml:"event_black_list,omitempty"`
	SecondsPerFile    int    `json:"sec_per_file,omitempty,string" yaml:"sec_per_file,omitempty"`
	SampleRate        int    `json:"sample_rate,omitempty,string" yaml:"sample_rate,omitempty"`
	DestinationEmail  string `json:"dest_email,omitempty" yaml:"dest_email,omitempty"`
	FromEmail         string `json:"from_email,omitempty" yaml:"from_email,omitempty"`
	Readable          bool   `json:"is_readable,omitempty,string" yaml:"is_readable,omitempty"`
	Subject           string `json:"subject,omitempty" yaml:"subject,omitempty"`
	StartTLS          bool   `json:"is_starttls,omitempty,string" yaml:"is_starttls,omitempty"`
	AuthLogin         bool   `json:"is_authlogin,omitempty,string" yaml:"is_authlogin,omitempty"`
	Indexing          bool   `json:"is_indexing,omitempty,string" yaml:"is_indexing,omitempty"`
	Compressing       bool   `json:"is_compression,omitempty,string" yaml:"is_compression,omitempty"`
	CategoryBlackList string `json:"cat_black_list,omitempty" yaml:"cat_black_list,omitempty"`
	CategoryWhiteList string `json:"cat_white_list,omitempty" yaml:"cat_white_list,omitempty"`
	RegionName        string `json:"region_name,omitempty" yaml:"region_name,omitempty"`
	EndpointURL       string `json:"endpoint_url,omitempty" yaml:"endpoint_url,omitempty"`
	AuthHeaderName    string `json:"auth_header_name,omitempty" yaml:"auth_header_name,omitempty"`
	AuthHeaderValue   string `json:"auth_header_value,omitempty" yaml:"auth_header_value,omitempty"`
	RoutingTopic      string `json:"routing_topic,omitempty" yaml:"routing_topic,omitempty"`
	LiteralTopic      string `json:"literal_topic,omitempty" yaml:"literal_topic,omitempty"`
	Topic             string `json:"topic,omitempty" yaml:"topic,omitempty"`
	Project           string `json:"project,omitempty" yaml:"project,omitempty"`
	Dataset           string `json:"dataset,omitempty" yaml:"dataset,omitempty"`
	Table             string `json:"table,omitempty" yaml:"table,omitempty"`
	HumioRepo         string `json:"humio_repo,omitempty" yaml:"humio_repo,omitempty"`
	HumioToken        string `json:"humio_api_token,omitempty" yaml:"humio_api_token,omitempty"`
	CustomTransform   string `json:"custom_transform,omitempty" yaml:"custom_transform,omitempty"`
	KeyID             string `json:"key_id,omitempty" yaml:"key_id,omitempty"`
	AttachmentText    string `json:"attachment_text,omitempty" yaml:"attachment_text,omitempty"`
	Message           string `json:"message,omitempty" yaml:"message,omitempty"`
	Color             string `json:"color,omitempty" yaml:"color,omitempty"`
	CloudID           string `json:"cloud_id,omitempty" yaml:"cloud_id,omitempty"`
	Index             string `json:"index,omitempty" yaml:"index,omitempty"`
	Addresses         string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	APIKey            string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
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

func (o *OutputConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {

	// Outputs have some fields as "string" which is not
	// supported by the YAML lib. So we use custom marshaling.
	// Since JSON supports it, we'll leverage it.
	genericVersion := map[string]interface{}{}
	if err := unmarshal(&genericVersion); err != nil {
		return err
	}

	// Some specific ",string" JSON fields expect to
	// Unmarshal to integers, but we might get empty
	// strings, so let's convert them.
	for _, fn := range []string{"sec_per_file", "sample_rate"} {
		e, ok := genericVersion[fn]
		if !ok {
			continue
		}
		s, ok := e.(string)
		if !ok || s != "" {
			continue
		}
		genericVersion[fn] = "0"
	}

	for key, val := range genericVersion {
		if strings.HasPrefix(key, "is_") {
			s, ok := val.(string)
			if ok {
				if strings.TrimSpace(s) == "" { // default to false
					genericVersion[key] = "false"
				}
			}
		}
	}

	rawJSON, err := json.Marshal(genericVersion)
	if err != nil {
		return err
	}

	newO := OutputConfig{}
	if err := json.Unmarshal(rawJSON, &newO); err != nil {
		return err
	}
	// When used for Sync, the format of the config file uses
	// the key "for" instead of "type". For backwards compatibility
	// we will do the swap when necessary.
	if newO.Type == "" {
		if t, ok := genericVersion["for"]; ok {
			newO.Type, _ = t.(string)
		}
	}
	*o = newO
	return nil
}

func (o OutputConfig) MarshalYAML() (interface{}, error) {
	rawJSON, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	genericVersion := map[string]interface{}{}
	if err := json.Unmarshal(rawJSON, &genericVersion); err != nil {
		return nil, err
	}
	return genericVersion, nil
}

type tempoutputResponse outputResponse

func (md *outputResponse) UnmarshalJSON(data []byte) error {
	// First get all the fields parsed in a dictionary.
	d := map[string]interface{}{}
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}

	// Filter out all the empty strings
	// which we want to go to the default empty value.
	for k, v := range d {
		if v != "" {
			continue
		}
		delete(d, k)
	}

	// Re-marshal to JSON so that we can
	// do another single-pass Unmarshal.
	t, err := json.Marshal(d)
	if err != nil {
		return err
	}

	// Finally extract to a temporary type
	// (to bypass this custom Unmarshaler).
	tmd := tempoutputResponse{}
	if err := json.Unmarshal(t, &tmd); err != nil {
		return err
	}
	*md = outputResponse(tmd)
	return nil
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
		return nil, err
	}

	outByName, ok := outputsByOrg[org.client.options.OID]
	if !ok {
		return nil, ErrorResourceNotFound
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
