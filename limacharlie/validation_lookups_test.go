package limacharlie

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newReplayCaptureOrg returns an Organization whose replay endpoint is a local
// server answering with a validation success, plus a function returning the
// decoded body of the last request that server received.
func newReplayCaptureOrg(t *testing.T) (*Organization, func() map[string]interface{}) {
	t.Helper()
	var last []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	org := &Organization{
		client:     &Client{options: ClientOptions{OID: "00000000-0000-4000-8000-000000000001", JWT: "test-jwt"}},
		cachedURLs: &SiteConnectivityInfo{URLs: SiteURLs{Replay: srv.URL}},
	}
	return org, func() map[string]interface{} {
		t.Helper()
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(last, &body), "request body: %s", last)
		return body
	}
}

// A rule using the same kinds of nesting the generator produces: lookups under
// and/or, in a list of values, inside a metadata rule, plus a non-lookup resource.
func lookupRule() Dict {
	return Dict{
		"detect": Dict{
			"event": "DNS_REQUEST",
			"op":    "and",
			"rules": List{
				Dict{"op": "lookup", "path": "event/DOMAIN_NAME", "resource": "hive://lookup/bad-domains"},
				Dict{
					"op": "or",
					"rules": []Dict{
						{"op": "lookup", "path": "event/DOMAIN_NAME", "resource": "hive://lookup/other-domains"},
						{"op": "lookup", "path": "event/DOMAIN_NAME", "resource": "lcr://lookup/managed-list"},
					},
				},
				// Same lookup referenced twice, and a non-string resource.
				Dict{"op": "lookup", "path": "event/IP_ADDRESS", "resource": "hive://lookup/bad-domains"},
				Dict{"op": "is", "path": "event/X", "resource": 42},
			},
		},
		"respond": List{
			Dict{
				"action": "report",
				"name":   "bad-domain",
				"metadata_rules": Dict{
					"op":       "lookup",
					"path":     "detect/event/DOMAIN_NAME",
					"resource": "hive://lookup/metadata-list",
				},
			},
		},
	}
}

func TestValidateDRRule_StubsEveryReferencedLookup(t *testing.T) {
	org, body := newReplayCaptureOrg(t)

	resp, err := org.ValidateDRRule(lookupRule())
	require.NoError(t, err)
	require.True(t, resp.Success)

	b := body()
	require.Equal(t, map[string]interface{}{
		"bad-domains":   map[string]interface{}{},
		"other-domains": map[string]interface{}{},
		"metadata-list": map[string]interface{}{},
	}, b["lookups"], "only hive://lookup/ resources are stubbed, each exactly once, as empty lookups")

	// The rule itself reaches the service unchanged.
	ruleSource := b["rule_source"].(map[string]interface{})
	want, err := json.Marshal(lookupRule())
	require.NoError(t, err)
	got, err := json.Marshal(ruleSource["rule"])
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got))
}

func TestValidateDRRuleWithLookups_CallerMockWins(t *testing.T) {
	org, body := newReplayCaptureOrg(t)

	supplied := map[string]Dict{
		"bad-domains":  {"evil.example.com": Dict{"tier": "high"}},
		"not-in-rule":  {"x": Dict{}},
		"nil-contents": nil,
	}
	_, err := org.ValidateDRRuleWithLookups(context.Background(), lookupRule(), supplied)
	require.NoError(t, err)

	require.Equal(t, map[string]interface{}{
		"bad-domains":   map[string]interface{}{"evil.example.com": map[string]interface{}{"tier": "high"}},
		"not-in-rule":   map[string]interface{}{"x": map[string]interface{}{}},
		"nil-contents":  map[string]interface{}{},
		"other-domains": map[string]interface{}{},
		"metadata-list": map[string]interface{}{},
	}, body()["lookups"])
	require.Len(t, supplied, 3, "the caller's map must not be modified")
}

func TestValidateDRRule_NoLookupsKeyWithoutLookups(t *testing.T) {
	org, body := newReplayCaptureOrg(t)

	rule := Dict{
		"detect":  Dict{"event": "NEW_PROCESS", "op": "is", "path": "event/FILE_PATH", "value": "*/cmd.exe"},
		"respond": List{Dict{"action": "report", "name": "x"}},
	}
	_, err := org.ValidateDRRule(rule)
	require.NoError(t, err)
	require.NotContains(t, body(), "lookups")

	// A rule that only uses a non-hive resource also sends nothing.
	rule["detect"] = Dict{"event": "NEW_PROCESS", "op": "lookup", "path": "event/FILE_PATH", "resource": "lcr://lookup/managed"}
	_, err = org.ValidateDRRule(rule)
	require.NoError(t, err)
	require.NotContains(t, body(), "lookups")
}

func TestReplayDRRule_SendsLookupsOnlyWhenSet(t *testing.T) {
	org, body := newReplayCaptureOrg(t)
	req := ReplayDRRuleRequest{Rule: lookupRule(), Events: []Dict{{"routing": Dict{}, "event": Dict{}}}}

	_, err := org.ReplayDRRule(req)
	require.NoError(t, err)
	require.NotContains(t, body(), "lookups", "replay does not auto-stub lookups")

	req.Lookups = map[string]Dict{"bad-domains": {"evil.example.com": Dict{}}}
	_, err = org.ReplayDRRule(req)
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"bad-domains": map[string]interface{}{"evil.example.com": map[string]interface{}{}},
	}, body()["lookups"])
}
