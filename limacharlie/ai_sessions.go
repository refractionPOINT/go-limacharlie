package limacharlie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file ports the AI sessions / usage surface of the Python SDK
// (limacharlie/sdk/ai.py and limacharlie/sdk/ai_session.py) to Go so the
// MCP server can drive AI sessions through the Go SDK.
//
// The ai-sessions service is a SEPARATE micro-service host, resolved per-org
// from the "ai" entry of the organization's URL map (see
// Organization.getServiceRoot). It is NOT served from api.limacharlie.io.
//
// SCOPE: only the REST endpoints are implemented here. The WebSocket
// attach/chat streaming protocol (the SessionAttachment class in
// ai_session.py and the owner-interactive / org read-only /v1/ws/* routes)
// is intentionally OUT OF SCOPE: it is not REST and is not needed for the
// MCP REST parity work.

// hiveSecretPrefix is the prefix marking a value that must be resolved from
// the "secret" hive before being sent. Mirrors ai.py's _HIVE_SECRET_PREFIX.
const hiveSecretPrefix = "hive://secret/"

// hiveAIAgentPrefix is the URI form of an ai_agent hive reference. start_session
// accepts both a bare record key and this prefixed form. Mirrors ai.py's
// _HIVE_PREFIX in start_session.
const hiveAIAgentPrefix = "hive://ai_agent/"

// profileScalarFields are the ai_agent hive fields copied verbatim into the
// request's "profile" section. Mirrors ai.py's _PROFILE_SCALAR_FIELDS; each
// maps one-to-one onto a field of the server's ProfileContent type.
var profileScalarFields = []string{
	"allowed_tools", "denied_tools", "permission_mode",
	"model", "max_turns", "max_budget_usd", "task_budget_tokens",
	"ttl_seconds", "one_shot", "plugins",
}

// AISessions is the accessor for the org's AI sessions / usage REST endpoints.
// It mirrors the Python SDK's AI class (limacharlie/sdk/ai.py), restricted to
// the AI session management and usage surface. Obtain one via Organization.AI.
type AISessions struct {
	org *Organization
}

// AI returns an accessor for the organization's AI sessions and usage
// endpoints. Mirrors instantiating the Python SDK's AI class.
func (org *Organization) AI() *AISessions {
	return &AISessions{org: org}
}

// serviceRoot resolves the scheme-prefixed ai-sessions base host for this org,
// with no trailing slash. Mirrors ai.py's _get_ai_url (sans the hardcoded
// production fallback: the Go SDK relies on the org's URL map carrying "ai").
func (a *AISessions) serviceRoot() (string, error) {
	root, err := a.org.getServiceRoot("ai")
	if err != nil {
		return "", fmt.Errorf("failed to resolve ai-sessions URL: %w", err)
	}
	return root, nil
}

// orgAuthHeaders builds the identity headers for the org-scoped ai-sessions
// endpoints. Mirrors ai.py's _org_auth_headers plus the api-key path used by
// _org_request / start_session:
//   - X-LC-OID is always set.
//   - X-LC-UID is set only when a UID is configured.
//   - Authorization: Bearer <APIKey> is set (overriding the default JWT) when
//     an API key is configured; the foundation applies extraHeaders last so
//     this wins over the default JWT Authorization header.
func (a *AISessions) orgAuthHeaders() map[string]string {
	opts := a.org.client.options
	headers := map[string]string{
		"X-LC-OID": a.org.GetOID(),
	}
	if opts.UID != "" {
		headers["X-LC-UID"] = opts.UID
	}
	if opts.APIKey != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", opts.APIKey)
	}
	return headers
}

// userAuthHeaders builds the headers for the user-scoped ai-sessions endpoints.
// Mirrors ai.py's _user_request: user-scoped routes identify the caller via
// their JWT's UID alone, so no X-LC-OID and no raw-API-key Authorization
// override are sent; the foundation's default JWT Authorization header is used.
func (a *AISessions) userAuthHeaders() map[string]string {
	return map[string]string{}
}

// orgRequest performs an authenticated request against an org-scoped
// ai-sessions endpoint. Mirrors ai.py's _org_request. path must be
// service-root-relative and begin with "/" (e.g. "/v1/org/sessions").
func (a *AISessions) orgRequest(ctx context.Context, verb string, path string, query Dict, response interface{}) error {
	root, err := a.serviceRoot()
	if err != nil {
		return err
	}
	req := makeDefaultRequest(response).
		withURLRoot(root).
		withExtraHeaders(a.orgAuthHeaders())
	if len(query) != 0 {
		req = req.withQueryData(query)
	}
	if err := a.org.client.reliableRequest(ctx, verb, path, req); err != nil {
		return fmt.Errorf("ai-sessions request %s %s failed: %w", verb, path, err)
	}
	return nil
}

// userRequest performs a request against a user-scoped ai-sessions endpoint.
// Mirrors ai.py's _user_request. path must begin with "/".
func (a *AISessions) userRequest(ctx context.Context, verb string, path string, query Dict, response interface{}) error {
	root, err := a.serviceRoot()
	if err != nil {
		return err
	}
	req := makeDefaultRequest(response).
		withURLRoot(root).
		withExtraHeaders(a.userAuthHeaders())
	if len(query) != 0 {
		req = req.withQueryData(query)
	}
	if err := a.org.client.reliableRequest(ctx, verb, path, req); err != nil {
		return fmt.Errorf("ai-sessions request %s %s failed: %w", verb, path, err)
	}
	return nil
}

// resolveSecret resolves a value that may be a hive://secret/<name> reference.
// Mirrors ai.py's _resolve_secret. Non-reference values are returned unchanged.
func (a *AISessions) resolveSecret(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, hiveSecretPrefix) {
		return value, nil
	}
	secretName := value[len(hiveSecretPrefix):]
	if secretName == "" {
		return "", fmt.Errorf("empty secret name in hive://secret/ reference")
	}
	record, err := NewHiveClient(a.org).Get(HiveArgs{
		HiveName:     "secret",
		PartitionKey: a.org.GetOID(),
		Key:          secretName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve secret %q: %w", secretName, err)
	}
	secret, ok := record.Data["secret"].(string)
	if !ok {
		return "", fmt.Errorf("secret %q has no string 'secret' field", secretName)
	}
	return secret, nil
}

// resolveMapSecrets resolves hive://secret/<name> references in all values of a
// string map. Mirrors ai.py's _resolve_map_secrets.
func (a *AISessions) resolveMapSecrets(m map[string]interface{}) (map[string]interface{}, error) {
	if len(m) == 0 {
		return m, nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			resolved, err := a.resolveSecret(s)
			if err != nil {
				return nil, err
			}
			out[k] = resolved
		} else {
			out[k] = v
		}
	}
	return out, nil
}

// StartSessionOptions carries the optional overrides for StartSession. A nil
// pointer field means "keep the ai_agent template value" (mirroring Python's
// None defaults); a non-nil pointer replaces the template field. Environment is
// merged with the template's environment (override wins on key collision); all
// other overrides replace the template value outright.
type StartSessionOptions struct {
	// Prompt replaces the prompt from the definition.
	Prompt *string
	// Name replaces the session name.
	Name *string
	// IdempotentKey is the deduplication key for the session.
	IdempotentKey *string
	// Data is appended to the prompt as YAML event data (for standalone
	// invocations that lack a D&R event).
	Data Dict

	// Model replaces the Anthropic model (e.g. "claude-sonnet-4-6").
	Model *string
	// MaxTurns replaces the maximum number of agent turns.
	MaxTurns *int
	// MaxBudgetUSD replaces the hard USD cost cap.
	MaxBudgetUSD *float64
	// TaskBudgetTokens replaces the per-task token budget.
	TaskBudgetTokens *int
	// TTLSeconds replaces the session time-to-live in seconds.
	TTLSeconds *int
	// OneShot, when set, forces one_shot on/off; nil keeps the template value.
	OneShot *bool
	// PermissionMode replaces the permission mode ("acceptEdits", "plan",
	// "bypassPermissions").
	PermissionMode *string
	// AllowedTools replaces the allowed tools list.
	AllowedTools []string
	// DeniedTools replaces the denied tools list.
	DeniedTools []string
	// Plugins replaces the enabled plugins list.
	Plugins []string
	// Environment is merged with the template's environment (override wins on
	// key collision). Values may be hive://secret/<name> references.
	Environment map[string]string

	// AnthropicKey replaces the Anthropic API key. Literal or
	// hive://secret/<name> reference.
	AnthropicKey *string
	// LCAPIKey replaces the LC API key. Literal or hive://secret/<name>.
	LCAPIKey *string
	// LCUID replaces the LC user ID. Literal or hive://secret/<name>.
	LCUID *string
}

// StartSession starts an AI session using an ai_agent Hive definition as a
// template. Mirrors ai.py's start_session (ai.py:93-286).
//
// definitionName accepts either a bare ai_agent record key ("my-agent") or the
// "hive://ai_agent/<name>" URI form used by the D&R "start ai agent" action;
// the prefix is stripped and both resolve to the same record. Fields supplied
// via opts override the corresponding template fields (a nil pointer / empty
// slice leaves the template value untouched); Environment is merged.
//
// hive://secret/<name> references in the template and in overrides are resolved
// automatically (via NewHiveClient reading the "secret" and "ai_agent" hives)
// before the request is sent. The request is POSTed to v1/api/sessions.
func (a *AISessions) StartSession(ctx context.Context, definitionName string, opts *StartSessionOptions) (Dict, error) {
	if opts == nil {
		opts = &StartSessionOptions{}
	}

	// Accept both a bare record key and the hive://ai_agent/<name> URI form.
	definitionName = strings.TrimPrefix(definitionName, hiveAIAgentPrefix)

	// Fetch the ai_agent definition; treat its fields as the template.
	record, err := NewHiveClient(a.org).Get(HiveArgs{
		HiveName:     "ai_agent",
		PartitionKey: a.org.GetOID(),
		Key:          definitionName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get ai_agent definition %q: %w", definitionName, err)
	}
	defn := record.Data
	if defn == nil {
		defn = map[string]interface{}{}
	}

	// Credential resolution: override wins, otherwise pull from the hive
	// record's *_secret field. Override values are themselves passed through
	// secret resolution so a caller can still supply a hive://secret/... ref.
	anthropicKeyFinal, err := a.resolveSecret(pickString(opts.AnthropicKey, dictStr(defn, "anthropic_secret")))
	if err != nil {
		return nil, err
	}
	lcAPIKeyFinal, err := a.resolveSecret(pickString(opts.LCAPIKey, dictStr(defn, "lc_api_key_secret")))
	if err != nil {
		return nil, err
	}
	lcUIDFinal, err := a.resolveSecret(pickString(opts.LCUID, dictStr(defn, "lc_uid_secret")))
	if err != nil {
		return nil, err
	}

	// Fall back to the caller's own API key if nothing else produced one.
	if lcAPIKeyFinal == "" {
		lcAPIKeyFinal = a.org.client.options.APIKey
	}

	// Build the profile section by copying template fields verbatim.
	profile := Dict{}
	for _, field := range profileScalarFields {
		if v, ok := defn[field]; ok {
			profile[field] = v
		}
	}

	// Apply scalar / list overrides. A nil override means "keep template".
	if opts.Model != nil {
		profile["model"] = *opts.Model
	}
	if opts.MaxTurns != nil {
		profile["max_turns"] = *opts.MaxTurns
	}
	if opts.MaxBudgetUSD != nil {
		profile["max_budget_usd"] = *opts.MaxBudgetUSD
	}
	if opts.TaskBudgetTokens != nil {
		profile["task_budget_tokens"] = *opts.TaskBudgetTokens
	}
	if opts.TTLSeconds != nil {
		profile["ttl_seconds"] = *opts.TTLSeconds
	}
	if opts.OneShot != nil {
		profile["one_shot"] = *opts.OneShot
	}
	if opts.PermissionMode != nil {
		profile["permission_mode"] = *opts.PermissionMode
	}
	if opts.AllowedTools != nil {
		profile["allowed_tools"] = opts.AllowedTools
	}
	if opts.DeniedTools != nil {
		profile["denied_tools"] = opts.DeniedTools
	}
	if opts.Plugins != nil {
		profile["plugins"] = opts.Plugins
	}

	// Environment: merge template + overrides (override wins on key collision).
	templateEnv := dictStrMap(defn["environment"])
	if len(templateEnv) != 0 || len(opts.Environment) != 0 {
		mergedEnv := map[string]interface{}{}
		for k, v := range templateEnv {
			mergedEnv[k] = v
		}
		for k, v := range opts.Environment {
			mergedEnv[k] = v
		}
		resolvedEnv, err := a.resolveMapSecrets(mergedEnv)
		if err != nil {
			return nil, err
		}
		profile["environment"] = resolvedEnv
	}

	// Resolve secrets in MCP server configs (template wins; not overridable).
	if servers, ok := defn["mcp_servers"].(map[string]interface{}); ok && len(servers) != 0 {
		resolvedServers := map[string]interface{}{}
		for srvName, srvCfgRaw := range servers {
			srvCfg, ok := srvCfgRaw.(map[string]interface{})
			if !ok {
				resolvedServers[srvName] = srvCfgRaw
				continue
			}
			srv := map[string]interface{}{}
			for k, v := range srvCfg {
				srv[k] = v
			}
			if headers := dictStrMap(srv["headers"]); len(headers) != 0 {
				resolved, err := a.resolveMapSecrets(headers)
				if err != nil {
					return nil, err
				}
				srv["headers"] = resolved
			}
			if env := dictStrMap(srv["env"]); len(env) != 0 {
				resolved, err := a.resolveMapSecrets(env)
				if err != nil {
					return nil, err
				}
				srv["env"] = resolved
			}
			resolvedServers[srvName] = srv
		}
		profile["mcp_servers"] = resolvedServers
	}

	// Build the final prompt, optionally appending supplied data.
	finalPrompt := pickString(opts.Prompt, dictStr(defn, "prompt"))
	if len(opts.Data) != 0 {
		yamlBytes, err := yaml.Marshal(opts.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal event data to yaml: %w", err)
		}
		finalPrompt += "\n\nEvent data:\n```yaml\n" + strings.TrimRight(string(yamlBytes), "\n") + "\n```"
	}

	// Build the request body.
	requestBody := Dict{
		"prompt":         finalPrompt,
		"anthropic_key":  anthropicKeyFinal,
		"trigger_source": "cli",
	}
	if lcAPIKeyFinal != "" {
		requestBody["lc_api_key"] = lcAPIKeyFinal
	}
	if lcUIDFinal != "" {
		requestBody["lc_uid"] = lcUIDFinal
	}
	if name := pickString(opts.Name, dictStr(defn, "name")); name != "" {
		requestBody["name"] = name
	}
	if opts.IdempotentKey != nil && *opts.IdempotentKey != "" {
		requestBody["idempotent_key"] = *opts.IdempotentKey
	}
	if len(profile) != 0 {
		requestBody["profile"] = profile
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal start_session body: %w", err)
	}

	root, err := a.serviceRoot()
	if err != nil {
		return nil, err
	}

	var resp Dict
	req := makeDefaultRequest(&resp).
		withURLRoot(root).
		withRawBody(bodyBytes, "application/json").
		withExtraHeaders(a.orgAuthHeaders())
	if err := a.org.client.reliableRequest(ctx, http.MethodPost, "/v1/api/sessions", req); err != nil {
		return nil, fmt.Errorf("ai-sessions start_session failed: %w", err)
	}
	return resp, nil
}

// ListSessionsOptions filters and paginates a single page of org-scoped
// sessions. All fields are optional.
type ListSessionsOptions struct {
	// Status filters by session status ("running", "starting", "ended").
	Status string
	// Limit caps the results per page (1-200; server default when 0).
	Limit int
	// Cursor resumes pagination from a previous response's NextCursor.
	Cursor string
}

func (o *ListSessionsOptions) query() Dict {
	q := Dict{}
	if o == nil {
		return q
	}
	if o.Status != "" {
		q["status"] = o.Status
	}
	if o.Limit != 0 {
		q["limit"] = strconv.Itoa(o.Limit)
	}
	if o.Cursor != "" {
		q["cursor"] = o.Cursor
	}
	return q
}

// ListSessions fetches a single page of AI sessions for the organization.
// Mirrors ai.py's list_sessions_page (ai.py:402-427). The returned Dict carries
// a "sessions" list and a "next_cursor" string. GET v1/org/sessions.
func (a *AISessions) ListSessions(ctx context.Context, opts *ListSessionsOptions) (Dict, error) {
	var resp Dict
	if err := a.orgRequest(ctx, http.MethodGet, "/v1/org/sessions", opts.query(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetSession gets details of a specific AI session. Mirrors ai.py's get_session
// (ai.py:458-467). The returned Dict carries a "session" object.
// GET v1/org/sessions/{id}.
func (a *AISessions) GetSession(ctx context.Context, sessionID string) (Dict, error) {
	var resp Dict
	path := fmt.Sprintf("/v1/org/sessions/%s", sessionID)
	if err := a.orgRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetSessionHistory gets the conversation history of an AI session. Mirrors
// ai.py's get_session_history (ai.py:480-489). The returned Dict carries a
// "messages" list. GET v1/org/sessions/{id}/history.
func (a *AISessions) GetSessionHistory(ctx context.Context, sessionID string) (Dict, error) {
	var resp Dict
	path := fmt.Sprintf("/v1/org/sessions/%s/history", sessionID)
	if err := a.orgRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// TerminateSession terminates a running AI session. Mirrors ai.py's
// terminate_session (ai.py:469-478). The returned Dict carries
// "terminated: true". DELETE v1/org/sessions/{id}.
func (a *AISessions) TerminateSession(ctx context.Context, sessionID string) (Dict, error) {
	var resp Dict
	path := fmt.Sprintf("/v1/org/sessions/%s", sessionID)
	if err := a.orgRequest(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListUsageIdentities lists all API key identities with AI session usage data.
// Mirrors ai.py's list_usage_identities (ai.py:514-520). The returned Dict
// carries an "identities" list of strings. GET v1/org/usage/identities.
func (a *AISessions) ListUsageIdentities(ctx context.Context) (Dict, error) {
	var resp Dict
	if err := a.orgRequest(ctx, http.MethodGet, "/v1/org/usage/identities", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUsage gets hourly token and cost usage for a specific API key identity.
// Mirrors ai.py's get_usage (ai.py:522-531). The returned Dict carries an
// "identity" string and a "usage" list of data points.
// GET v1/org/usage/identities/{identity}.
func (a *AISessions) GetUsage(ctx context.Context, identity string) (Dict, error) {
	var resp Dict
	path := fmt.Sprintf("/v1/org/usage/identities/%s", identity)
	if err := a.orgRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListUserSessions fetches a single page of user-owned (chat) sessions. Mirrors
// ai.py's list_user_sessions_page (ai.py:650-675). Unlike ListSessions, this
// hits the user-scoped route and sees sessions created via the "ai chat" flow
// rather than org-scoped sessions. GET v1/sessions.
func (a *AISessions) ListUserSessions(ctx context.Context, opts *ListSessionsOptions) (Dict, error) {
	var resp Dict
	if err := a.userRequest(ctx, http.MethodGet, "/v1/sessions", opts.query(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUserSession gets details of a user-owned (chat) session. Mirrors ai.py's
// get_user_session (ai.py:711-720). The returned Dict carries a "session"
// object. GET v1/sessions/{id}.
func (a *AISessions) GetUserSession(ctx context.Context, sessionID string) (Dict, error) {
	var resp Dict
	path := fmt.Sprintf("/v1/sessions/%s", sessionID)
	if err := a.userRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUserSessionHistory gets the conversation history of a user-owned (chat)
// session. Mirrors ai.py's get_user_session_history (ai.py:733-742). The
// returned Dict carries a "messages" list. GET v1/sessions/{id}/history.
func (a *AISessions) GetUserSessionHistory(ctx context.Context, sessionID string) (Dict, error) {
	var resp Dict
	path := fmt.Sprintf("/v1/sessions/%s/history", sessionID)
	if err := a.userRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// pickString returns *override when it is non-nil, otherwise fallback. Mirrors
// Python's "override if override is not None else template" idiom.
func pickString(override *string, fallback string) string {
	if override != nil {
		return *override
	}
	return fallback
}

// dictStr returns the string value at key, or "" if absent / not a string.
func dictStr(d map[string]interface{}, key string) string {
	if d == nil {
		return ""
	}
	if s, ok := d[key].(string); ok {
		return s
	}
	return ""
}

// dictStrMap coerces an interface{} (as decoded from a hive record) into a
// map[string]interface{}, returning nil when it is not a map.
func dictStrMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(Dict); ok {
		return m
	}
	return nil
}
