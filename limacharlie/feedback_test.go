package limacharlie

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

const feedbackTestOID = "00000000-0000-0000-0000-000000000001"

// feedbackCapture records what the extension request handler saw.
type feedbackCapture struct {
	method string
	path   string
	body   []byte
}

// registerFeedbackExtHandler installs a custom handler on the ext-feedback
// extension request path that captures the request and replies with respBody.
func registerFeedbackExtHandler(ms *MockServer, respBody string) *feedbackCapture {
	cap := &feedbackCapture{}
	ms.CustomHandlers["/v1/extension/request/ext-feedback"] = func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.body = b
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}
	return cap
}

func newFeedbackTestOrg(t *testing.T) (*MockServer, *Organization) {
	t.Helper()
	ms := NewMockServer(feedbackTestOID)
	org, err := ms.NewOrganization()
	require.NoError(t, err)
	return ms, org
}

// extData parses the form-encoded extension request body and returns the
// decoded "data" JSON payload along with the action.
func extData(t *testing.T, body []byte) (string, Dict) {
	t.Helper()
	form, err := url.ParseQuery(string(body))
	require.NoError(t, err)
	var data Dict
	require.NoError(t, json.Unmarshal([]byte(form.Get("data")), &data))
	return form.Get("action"), data
}

func TestFeedbackRequestApproval(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	cap := registerFeedbackExtHandler(ms, `{"data":{"request_id":"req-1","url":"https://web"}}`)

	resp, err := org.Feedback().RequestApproval("ops-slack", "Isolate host-01?", "case", RequestApprovalOptions{
		CaseID:          "42",
		ApprovedContent: Dict{"action": "isolate"},
		DeniedContent:   Dict{"action": "skip"},
		TimeoutSeconds:  300,
		TimeoutChoice:   "denied",
		TimeoutContent:  Dict{"reason": "timeout"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, http.MethodPost, cap.method)
	require.Equal(t, "/v1/extension/request/ext-feedback", cap.path)

	action, data := extData(t, cap.body)
	require.Equal(t, "request_simple_approval", action)
	require.Equal(t, "ops-slack", data["channel"])
	require.Equal(t, "Isolate host-01?", data["question"])
	require.Equal(t, "case", data["feedback_destination"])
	require.Equal(t, "42", data["case_id"])
	require.EqualValues(t, 300, data["timeout_seconds"])
	require.Equal(t, "denied", data["timeout_choice"])
	approved, ok := data["approved_content"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "isolate", approved["action"])
	denied, ok := data["denied_content"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "skip", denied["action"])
	tc, ok := data["timeout_content"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "timeout", tc["reason"])
	// playbook_name must be omitted when not set.
	require.NotContains(t, data, "playbook_name")
}

func TestFeedbackRequestApprovalMinimal(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	cap := registerFeedbackExtHandler(ms, `{"data":{"request_id":"req-1"}}`)

	_, err := org.Feedback().RequestApproval("web", "Approve?", "playbook", RequestApprovalOptions{
		PlaybookName: "remediate-host",
	})
	require.NoError(t, err)

	action, data := extData(t, cap.body)
	require.Equal(t, "request_simple_approval", action)
	require.Equal(t, "playbook", data["feedback_destination"])
	require.Equal(t, "remediate-host", data["playbook_name"])
	// Optional fields not set must be omitted.
	require.NotContains(t, data, "case_id")
	require.NotContains(t, data, "approved_content")
	require.NotContains(t, data, "timeout_seconds")
	require.NotContains(t, data, "timeout_choice")
}

func TestFeedbackRequestAcknowledgement(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	cap := registerFeedbackExtHandler(ms, `{"data":{"request_id":"req-2"}}`)

	_, err := org.Feedback().RequestAcknowledgement("email-oncall", "Ack incident #7", "case", RequestAcknowledgementOptions{
		CaseID:              "7",
		AcknowledgedContent: Dict{"status": "seen"},
		TimeoutSeconds:      600,
		TimeoutContent:      Dict{"status": "auto-ack"},
	})
	require.NoError(t, err)

	action, data := extData(t, cap.body)
	require.Equal(t, "request_acknowledgement", action)
	require.Equal(t, "email-oncall", data["channel"])
	require.Equal(t, "Ack incident #7", data["question"])
	require.Equal(t, "case", data["feedback_destination"])
	require.Equal(t, "7", data["case_id"])
	require.EqualValues(t, 600, data["timeout_seconds"])
	ack, ok := data["acknowledged_content"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "seen", ack["status"])
	require.NotContains(t, data, "timeout_choice")
}

func TestFeedbackRequestQuestion(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	cap := registerFeedbackExtHandler(ms, `{"data":{"request_id":"req-3"}}`)

	_, err := org.Feedback().RequestQuestion("ops-slack", "What is the root cause?", "playbook", RequestQuestionOptions{
		PlaybookName:   "collect-input",
		TimeoutSeconds: 300,
		TimeoutContent: Dict{"answer": "no response"},
	})
	require.NoError(t, err)

	action, data := extData(t, cap.body)
	require.Equal(t, "request_question", action)
	require.Equal(t, "ops-slack", data["channel"])
	require.Equal(t, "What is the root cause?", data["question"])
	require.Equal(t, "playbook", data["feedback_destination"])
	require.Equal(t, "collect-input", data["playbook_name"])
	require.EqualValues(t, 300, data["timeout_seconds"])
	tc, ok := data["timeout_content"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "no response", tc["answer"])
	// request_question has no approved/denied/acknowledged content fields.
	require.NotContains(t, data, "approved_content")
	require.NotContains(t, data, "acknowledged_content")
	require.NotContains(t, data, "timeout_choice")
}

// seedFeedbackConfig seeds the ext-feedback record in the extension_config
// hive with the given channel entries.
func seedFeedbackConfig(ms *MockServer, channels []interface{}) {
	storeKey := feedbackConfigHiveName + "/" + ms.OID
	if ms.HiveStore[storeKey] == nil {
		ms.HiveStore[storeKey] = map[string]HiveData{}
	}
	ms.HiveStore[storeKey][feedbackExtensionName] = HiveData{
		Data: Dict{"channels": channels},
		SysMtd: SysMtd{
			Etag: "etag-seed",
			GUID: "guid-seed",
		},
	}
}

func TestFeedbackListChannels(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	seedFeedbackConfig(ms, []interface{}{
		map[string]interface{}{"name": "web-default", "channel_type": "web"},
		map[string]interface{}{"name": "ops-slack", "channel_type": "slack", "output_name": "slack-soc"},
	})

	channels, err := org.Feedback().ListChannels()
	require.NoError(t, err)
	require.Len(t, channels, 2)

	require.Equal(t, "web-default", channels[0].Name)
	require.Equal(t, "web", channels[0].ChannelType)
	require.Empty(t, channels[0].OutputName)

	require.Equal(t, "ops-slack", channels[1].Name)
	require.Equal(t, "slack", channels[1].ChannelType)
	require.Equal(t, "slack-soc", channels[1].OutputName)
}

func TestFeedbackAddChannel(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	seedFeedbackConfig(ms, []interface{}{
		map[string]interface{}{"name": "web-default", "channel_type": "web"},
	})

	_, err := org.Feedback().AddChannel("tg-alerts", "telegram", "telegram-bot")
	require.NoError(t, err)

	// The channel must be persisted in the hive store.
	channels, err := org.Feedback().ListChannels()
	require.NoError(t, err)
	require.Len(t, channels, 2)
	require.Equal(t, "tg-alerts", channels[1].Name)
	require.Equal(t, "telegram", channels[1].ChannelType)
	require.Equal(t, "telegram-bot", channels[1].OutputName)
}

func TestFeedbackAddChannelWebNoOutput(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	seedFeedbackConfig(ms, []interface{}{})

	_, err := org.Feedback().AddChannel("web-default", "web", "")
	require.NoError(t, err)

	storeKey := feedbackConfigHiveName + "/" + ms.OID
	rec := ms.HiveStore[storeKey][feedbackExtensionName]
	rawChannels, ok := rec.Data["channels"].([]interface{})
	require.True(t, ok)
	require.Len(t, rawChannels, 1)
	entry := rawChannels[0].(map[string]interface{})
	require.Equal(t, "web-default", entry["name"])
	require.Equal(t, "web", entry["channel_type"])
	// web channels must not carry an output_name.
	require.NotContains(t, entry, "output_name")
}

func TestFeedbackAddChannelDuplicate(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	seedFeedbackConfig(ms, []interface{}{
		map[string]interface{}{"name": "ops-slack", "channel_type": "slack", "output_name": "slack-soc"},
	})

	_, err := org.Feedback().AddChannel("ops-slack", "slack", "slack-soc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestFeedbackRemoveChannel(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	seedFeedbackConfig(ms, []interface{}{
		map[string]interface{}{"name": "web-default", "channel_type": "web"},
		map[string]interface{}{"name": "ops-slack", "channel_type": "slack", "output_name": "slack-soc"},
	})

	_, err := org.Feedback().RemoveChannel("ops-slack")
	require.NoError(t, err)

	channels, err := org.Feedback().ListChannels()
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, "web-default", channels[0].Name)
}

func TestFeedbackRemoveChannelNotFound(t *testing.T) {
	ms, org := newFeedbackTestOrg(t)
	defer ms.Close()

	seedFeedbackConfig(ms, []interface{}{
		map[string]interface{}{"name": "web-default", "channel_type": "web"},
	})

	_, err := org.Feedback().RemoveChannel("does-not-exist")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
