package limacharlie

// Feedback SDK for LimaCharlie.
//
// Wraps the ext-feedback extension for sending interactive feedback
// requests (approval, acknowledgement, free-form question) to external
// channels (Slack, Email, Telegram, Teams, or the built-in Web UI) and
// for managing channel configuration.
//
// Feedback requests go through the LimaCharlie extension request
// mechanism (the request_simple_approval, request_acknowledgement and
// request_question actions on ext-feedback). Channel configuration is
// stored in the "extension_config" hive under the "ext-feedback" record,
// partitioned by the organization OID.

import (
	"fmt"
)

// feedbackExtensionName is the extension that backs the feedback system.
// It is also the record key used in the "extension_config" hive for
// channel configuration.
const feedbackExtensionName = "ext-feedback"

// feedbackConfigHiveName is the hive that holds extension configuration
// records, including the ext-feedback channel list.
const feedbackConfigHiveName = "extension_config"

// FeedbackChannel describes a configured feedback channel. Channels define
// where feedback requests are delivered. Each channel has a unique name, a
// type (web, slack, email, telegram, ms_teams), and, for non-web types, the
// name of a Tailored Output that holds the channel credentials.
type FeedbackChannel struct {
	// Name is the unique channel name referenced when sending requests.
	Name string `json:"name"`
	// ChannelType is one of web, slack, email, telegram, ms_teams.
	ChannelType string `json:"channel_type"`
	// OutputName is the Tailored Output holding the channel credentials.
	// It is required for all channel types except "web".
	OutputName string `json:"output_name,omitempty"`
}

// Feedback is the feedback system client for an Organization. Obtain one via
// Organization.Feedback.
type Feedback struct {
	o *Organization
}

// Feedback returns a Feedback client bound to this organization.
func (o *Organization) Feedback() *Feedback {
	return &Feedback{o: o}
}

// RequestApprovalOptions holds the optional parameters for a simple approval
// (Approve/Deny) request. Required parameters are positional arguments on
// RequestApproval.
type RequestApprovalOptions struct {
	// CaseID is the case number to attach the response to. Required when the
	// destination is "case".
	CaseID string
	// PlaybookName is the playbook to trigger with the response. Required when
	// the destination is "playbook".
	PlaybookName string
	// ApprovedContent is JSON data included in the response payload when the
	// recipient approves.
	ApprovedContent Dict
	// DeniedContent is JSON data included in the response payload when the
	// recipient denies.
	DeniedContent Dict
	// TimeoutSeconds, when non-zero, auto-responds after this many seconds with
	// no human response (minimum 60). Requires TimeoutChoice.
	TimeoutSeconds int
	// TimeoutChoice is the choice to auto-select on timeout ("approved" or
	// "denied"). Required when TimeoutSeconds is set.
	TimeoutChoice string
	// TimeoutContent is JSON data for the timeout response payload (overrides
	// ApprovedContent/DeniedContent for the timeout).
	TimeoutContent Dict
}

// RequestApproval sends a simple Approve/Deny feedback request to a channel.
//
// The respondent sees Approve and Deny buttons; their choice is dispatched to
// the given feedbackDestination ("case" or "playbook"). For web channels, the
// response includes a shareable URL. The response dict carries the request_id
// and, for web channels, a url.
func (f *Feedback) RequestApproval(channel string, question string, feedbackDestination string, opts RequestApprovalOptions) (Dict, error) {
	data := Dict{
		"channel":              channel,
		"question":             question,
		"feedback_destination": feedbackDestination,
	}
	if opts.CaseID != "" {
		data["case_id"] = opts.CaseID
	}
	if opts.PlaybookName != "" {
		data["playbook_name"] = opts.PlaybookName
	}
	if opts.ApprovedContent != nil {
		data["approved_content"] = opts.ApprovedContent
	}
	if opts.DeniedContent != nil {
		data["denied_content"] = opts.DeniedContent
	}
	if opts.TimeoutSeconds != 0 {
		data["timeout_seconds"] = opts.TimeoutSeconds
	}
	if opts.TimeoutChoice != "" {
		data["timeout_choice"] = opts.TimeoutChoice
	}
	if opts.TimeoutContent != nil {
		data["timeout_content"] = opts.TimeoutContent
	}
	var resp Dict
	if err := f.o.ExtensionRequest(&resp, feedbackExtensionName, "request_simple_approval", data, false); err != nil {
		return nil, fmt.Errorf("failed to request approval: %w", err)
	}
	return resp, nil
}

// RequestAcknowledgementOptions holds the optional parameters for an
// acknowledgement request. Required parameters are positional arguments on
// RequestAcknowledgement.
type RequestAcknowledgementOptions struct {
	// CaseID is the case number to attach the response to. Required when the
	// destination is "case".
	CaseID string
	// PlaybookName is the playbook to trigger with the response. Required when
	// the destination is "playbook".
	PlaybookName string
	// AcknowledgedContent is JSON data included in the response payload when
	// the recipient acknowledges.
	AcknowledgedContent Dict
	// TimeoutSeconds, when non-zero, auto-acknowledges after this many seconds
	// with no human response (minimum 60).
	TimeoutSeconds int
	// TimeoutContent is JSON data for the timeout response payload (overrides
	// AcknowledgedContent for the timeout).
	TimeoutContent Dict
}

// RequestAcknowledgement sends an acknowledgement request (single Acknowledge
// button) to a channel.
//
// When acknowledged (or on timeout), the response is dispatched to the given
// feedbackDestination ("case" or "playbook"). The response dict carries the
// request_id and, for web channels, a url.
func (f *Feedback) RequestAcknowledgement(channel string, question string, feedbackDestination string, opts RequestAcknowledgementOptions) (Dict, error) {
	data := Dict{
		"channel":              channel,
		"question":             question,
		"feedback_destination": feedbackDestination,
	}
	if opts.CaseID != "" {
		data["case_id"] = opts.CaseID
	}
	if opts.PlaybookName != "" {
		data["playbook_name"] = opts.PlaybookName
	}
	if opts.AcknowledgedContent != nil {
		data["acknowledged_content"] = opts.AcknowledgedContent
	}
	if opts.TimeoutSeconds != 0 {
		data["timeout_seconds"] = opts.TimeoutSeconds
	}
	if opts.TimeoutContent != nil {
		data["timeout_content"] = opts.TimeoutContent
	}
	var resp Dict
	if err := f.o.ExtensionRequest(&resp, feedbackExtensionName, "request_acknowledgement", data, false); err != nil {
		return nil, fmt.Errorf("failed to request acknowledgement: %w", err)
	}
	return resp, nil
}

// RequestQuestionOptions holds the optional parameters for a free-form
// question request. Required parameters are positional arguments on
// RequestQuestion.
type RequestQuestionOptions struct {
	// CaseID is the case number to attach the response to. Required when the
	// destination is "case".
	CaseID string
	// PlaybookName is the playbook to trigger with the response. Required when
	// the destination is "playbook".
	PlaybookName string
	// TimeoutSeconds, when non-zero, auto-answers after this many seconds with
	// no human response (minimum 60). Requires TimeoutContent.
	TimeoutSeconds int
	// TimeoutContent is JSON data used as the automatic answer on timeout.
	// Required when TimeoutSeconds is set for question requests.
	TimeoutContent Dict
}

// RequestQuestion sends a question with a free-form text input field to a
// channel.
//
// The respondent types a text answer which is dispatched to the given
// feedbackDestination ("case" or "playbook"). The response dict carries the
// request_id and, for web channels, a url.
func (f *Feedback) RequestQuestion(channel string, question string, feedbackDestination string, opts RequestQuestionOptions) (Dict, error) {
	data := Dict{
		"channel":              channel,
		"question":             question,
		"feedback_destination": feedbackDestination,
	}
	if opts.CaseID != "" {
		data["case_id"] = opts.CaseID
	}
	if opts.PlaybookName != "" {
		data["playbook_name"] = opts.PlaybookName
	}
	if opts.TimeoutSeconds != 0 {
		data["timeout_seconds"] = opts.TimeoutSeconds
	}
	if opts.TimeoutContent != nil {
		data["timeout_content"] = opts.TimeoutContent
	}
	var resp Dict
	if err := f.o.ExtensionRequest(&resp, feedbackExtensionName, "request_question", data, false); err != nil {
		return nil, fmt.Errorf("failed to request question: %w", err)
	}
	return resp, nil
}

// getConfig reads the ext-feedback record from the extension_config hive. A
// missing record (RECORD_NOT_FOUND) is surfaced as an error by the hive client.
func (f *Feedback) getConfig() (*HiveData, error) {
	hive := NewHiveClient(f.o)
	return hive.Get(HiveArgs{
		HiveName:     feedbackConfigHiveName,
		PartitionKey: f.o.GetOID(),
		Key:          feedbackExtensionName,
	})
}

// setConfig writes the ext-feedback record back to the extension_config hive,
// preserving the record's existing metadata and using its etag for an
// optimistic-locked update (mirrors Hive.set in the Python SDK).
func (f *Feedback) setConfig(record *HiveData) (*HiveResp, error) {
	hive := NewHiveClient(f.o)
	enabled := record.UsrMtd.Enabled
	expiry := record.UsrMtd.Expiry
	etag := record.SysMtd.Etag
	comment := record.UsrMtd.Comment
	return hive.Add(HiveArgs{
		HiveName:     feedbackConfigHiveName,
		PartitionKey: f.o.GetOID(),
		Key:          feedbackExtensionName,
		Data:         record.Data,
		Enabled:      &enabled,
		Expiry:       &expiry,
		Tags:         record.UsrMtd.Tags,
		Comment:      &comment,
		ETag:         &etag,
	})
}

// channelsFromData extracts the channels list from a record's data as a slice
// of FeedbackChannel.
func channelsFromData(data Dict) []FeedbackChannel {
	channels := []FeedbackChannel{}
	raw, ok := data["channels"].([]interface{})
	if !ok {
		return channels
	}
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ch := FeedbackChannel{}
		if v, ok := m["name"].(string); ok {
			ch.Name = v
		}
		if v, ok := m["channel_type"].(string); ok {
			ch.ChannelType = v
		}
		if v, ok := m["output_name"].(string); ok {
			ch.OutputName = v
		}
		channels = append(channels, ch)
	}
	return channels
}

// ListChannels returns the feedback channels configured for the organization.
func (f *Feedback) ListChannels() ([]FeedbackChannel, error) {
	record, err := f.getConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read feedback config: %w", err)
	}
	return channelsFromData(record.Data), nil
}

// AddChannel adds a feedback channel to the organization's ext-feedback
// configuration. channelType is one of web, slack, email, telegram, ms_teams.
// outputName names the Tailored Output holding the channel credentials and is
// required for all types except "web" (pass an empty string for web).
//
// It returns an error if a channel with the same name already exists.
func (f *Feedback) AddChannel(name string, channelType string, outputName string) (*HiveResp, error) {
	record, err := f.getConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read feedback config: %w", err)
	}
	channels := channelsFromData(record.Data)
	for _, ch := range channels {
		if ch.Name == name {
			return nil, fmt.Errorf("channel %q already exists", name)
		}
	}
	entry := Dict{
		"name":         name,
		"channel_type": channelType,
	}
	if outputName != "" {
		entry["output_name"] = outputName
	}
	rawChannels, _ := record.Data["channels"].([]interface{})
	rawChannels = append(rawChannels, entry)
	if record.Data == nil {
		record.Data = Dict{}
	}
	record.Data["channels"] = rawChannels
	return f.setConfig(record)
}

// RemoveChannel removes a feedback channel from the organization's
// ext-feedback configuration. It does not delete the associated Tailored
// Output.
//
// It returns an error if no channel with the given name exists.
func (f *Feedback) RemoveChannel(name string) (*HiveResp, error) {
	record, err := f.getConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read feedback config: %w", err)
	}
	rawChannels, _ := record.Data["channels"].([]interface{})
	newChannels := []interface{}{}
	for _, item := range rawChannels {
		m, ok := item.(map[string]interface{})
		if ok {
			if v, _ := m["name"].(string); v == name {
				continue
			}
		}
		newChannels = append(newChannels, item)
	}
	if len(newChannels) == len(rawChannels) {
		return nil, fmt.Errorf("channel %q not found", name)
	}
	if record.Data == nil {
		record.Data = Dict{}
	}
	record.Data["channels"] = newChannels
	return f.setConfig(record)
}
