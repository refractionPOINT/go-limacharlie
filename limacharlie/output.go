package limacharlie

import (
	"fmt"
	"net/http"
	"time"
)

type OutputModuleType string

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

type OutputModuleStream string

var OutputStreams = struct {
	Event      OutputModuleStream
	Detect     OutputModuleStream
	Audit      OutputModuleStream
	Deployment OutputModuleStream
	Artifact   OutputModuleStream
}{
	Event:      "event",
	Detect:     "detect",
	Audit:      "audit",
	Deployment: "deployment",
	Artifact:   "artifact",
}

type GenericOutputConfig struct {
	Name   string             `json:"name"`
	Module OutputModuleType   `json:"module"`
	Stream OutputModuleStream `json:"type"`

	PrefixData        bool   `json:"is_prefix_data,omitempty"`
	DeleteInFailure   bool   `json:"is_delete_on_failure,omitempty"`
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
	TLS               bool   `json:"is_tls,omitempty"`
	StrictTLS         bool   `json:"is_strict_tls,omitempty"`
	NoHeader          bool   `json:"is_no_header,omitempty"`
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

func (c *Client) Outputs() (map[string]interface{}, error) {
	outputs := map[string]map[string]interface{}{}
	if err := c.reliableRequest(http.MethodGet, fmt.Sprintf("outputs/%s", c.options.OID), restRequest{
		nRetries: 3,
		timeout:  10 * time.Second,
		response: &outputs,
	}); err != nil {
		return nil, err
	}

	orgOutputs, ok := outputs[c.options.OID]
	if !ok {
		return nil, ResourceNotFoundError
	}
	return orgOutputs, nil
}

func (c *Client) OutputAdd(config interface{}) (map[string]interface{}, error) {
	resp := map[string]interface{}{}
	if err := c.reliableRequest(http.MethodPost, fmt.Sprintf("outputs/%s", c.options.OID), restRequest{
		nRetries: 3,
		timeout:  10 * time.Second,
		response: &resp,
		formData: config,
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) OutputDel(name string) (map[string]interface{}, error) {
	resp := map[string]interface{}{}
	if err := c.reliableRequest(http.MethodDelete, fmt.Sprintf("outputs/%s", c.options.OID), restRequest{
		nRetries: 3,
		timeout:  10 * time.Second,
		response: &resp,
		formData: map[string]string{
			"name": name,
		},
	}); err != nil {
		return nil, err
	}

	return resp, nil
}