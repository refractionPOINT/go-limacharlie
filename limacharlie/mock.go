package limacharlie

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockCall records an HTTP request made to the mock server.
type MockCall struct {
	Method string
	Path   string
	Body   string
	Time   time.Time
}

// MockServer simulates the LimaCharlie API for testing.
// It provides in-memory state for common resources and records all HTTP calls
// so tests can verify both state changes and request patterns.
//
// Usage:
//
//	ms := limacharlie.NewMockServer("test-oid")
//	defer ms.Close()
//
//	org, err := ms.NewOrganization()
//	// use org exactly like a real Organization...
//	rules, err := org.DRRules()
type MockServer struct {
	Server *httptest.Server
	OID    string

	mu sync.RWMutex

	// Organization state
	OrgInfo OrganizationInformation
	URLs    SiteURLs

	// D&R Rules: name -> rule data
	DRRules map[string]Dict

	// FP Rules: name -> FPRule
	FPRules map[FPRuleName]FPRule

	// Installation Keys: iid -> InstallationKey
	InstallationKeyStore map[string]InstallationKey

	// Outputs: name -> OutputConfig
	OutputStore map[OutputName]OutputConfig

	// Sensors: sid -> Sensor
	SensorStore map[string]*Sensor

	// Sensor tags: sid -> tag -> TagInfo
	SensorTags map[string]map[string]TagInfo

	// Sensor isolation: sid -> isolated
	SensorIsolation map[string]bool

	// Sensor online status: sid -> online
	SensorOnline map[string]bool

	// Users: list of emails
	UserEmails []string

	// User permissions: email -> []perms
	UserPermissions map[string][]string

	// User roles: email -> role
	UserRoles map[string]string

	// Resources: category -> set of names
	ResourceStore ResourcesByCategory

	// Ingestion keys: name -> key value
	IngestionKeyStore map[string]string

	// Payloads: name -> Payload metadata
	PayloadStore map[PayloadName]Payload

	// Payload data: name -> bytes
	PayloadData map[PayloadName][]byte

	// Extensions: set of subscribed extension names
	ExtensionStore map[ExtensionName]bool

	// Hive: "hiveName/partition" -> key -> HiveData
	HiveStore map[string]map[string]HiveData

	// Exfil rules
	ExfilEventRules map[ExfilRuleName]ExfilRuleEvent
	ExfilWatchRules map[ExfilRuleName]ExfilRuleWatch

	// Artifact rules
	ArtifactRuleStore map[ArtifactRuleName]ArtifactRule

	// Org values: name -> value
	OrgValues map[string]string

	// IOC summaries: "type/name" -> IOCSummaryResponse
	IOCSummaries map[string]IOCSummaryResponse

	// Hostnames: hostname -> []HostnameSearchResult
	HostnameResults map[string][]HostnameSearchResult

	// Billing
	BillingStatus  *BillingOrgStatus
	BillingDetails *BillingOrgDetails
	BillingPlans   []BillingPlan

	// Groups
	GroupStore map[string]*GroupInfo

	// WhoAmI response
	WhoAmIResponse *WhoAmIJsonResponse

	// Tags across the org
	AllTags []string

	// Custom handler overrides: path -> handler.
	// If set, the custom handler is called instead of the default for matching paths.
	// Checked via prefix match, so "/v1/rules/" would override all rule routes.
	CustomHandlers map[string]http.HandlerFunc

	// Call log
	calls []MockCall
}

// NewMockServer creates a new mock LimaCharlie API server with default state.
// The oid parameter sets the organization ID used by the mock.
func NewMockServer(oid string) *MockServer {
	ms := &MockServer{
		OID: oid,
		OrgInfo: OrganizationInformation{
			OID:  oid,
			Name: "Mock Organization",
		},
		URLs: SiteURLs{
			Lc:        "lc.mock.limacharlie.io",
			LcWss:     "lc-wss.mock.limacharlie.io",
			EDR:       "edr.mock.limacharlie.io",
			Logs:      "logs.mock.limacharlie.io",
			Artifacts: "artifacts.mock.limacharlie.io",
			Replay:    "replay.mock.limacharlie.io",
			Live:      "live.mock.limacharlie.io",
			Hooks:     "hooks.mock.limacharlie.io",
			Search:    "search.mock.limacharlie.io",
			Cases:     "cases.mock.limacharlie.io",
		},
		DRRules:              map[string]Dict{},
		FPRules:              map[FPRuleName]FPRule{},
		InstallationKeyStore: map[string]InstallationKey{},
		OutputStore:          map[OutputName]OutputConfig{},
		SensorStore:          map[string]*Sensor{},
		SensorTags:           map[string]map[string]TagInfo{},
		SensorIsolation:      map[string]bool{},
		SensorOnline:         map[string]bool{},
		UserEmails:           []string{},
		UserPermissions:      map[string][]string{},
		UserRoles:            map[string]string{},
		ResourceStore:        ResourcesByCategory{},
		IngestionKeyStore:    map[string]string{},
		PayloadStore:         map[PayloadName]Payload{},
		PayloadData:          map[PayloadName][]byte{},
		ExtensionStore:       map[ExtensionName]bool{},
		HiveStore:            map[string]map[string]HiveData{},
		ExfilEventRules:      map[ExfilRuleName]ExfilRuleEvent{},
		ExfilWatchRules:      map[ExfilRuleName]ExfilRuleWatch{},
		ArtifactRuleStore:    map[ArtifactRuleName]ArtifactRule{},
		OrgValues:            map[string]string{},
		IOCSummaries:         map[string]IOCSummaryResponse{},
		HostnameResults:      map[string][]HostnameSearchResult{},
		BillingStatus:        &BillingOrgStatus{},
		BillingDetails:       &BillingOrgDetails{},
		BillingPlans:         []BillingPlan{},
		GroupStore:           map[string]*GroupInfo{},
		AllTags:              []string{},
		CustomHandlers:       map[string]http.HandlerFunc{},
		calls:                []MockCall{},
	}

	mux := http.NewServeMux()
	ms.registerRoutes(mux)
	// Wrap mux with custom handler dispatch
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.mu.RLock()
		for prefix, h := range ms.CustomHandlers {
			if strings.HasPrefix(r.URL.Path, prefix) {
				ms.mu.RUnlock()
				h(w, r)
				return
			}
		}
		ms.mu.RUnlock()
		mux.ServeHTTP(w, r)
	})
	ms.Server = httptest.NewServer(handler)

	// Update URLs to point to mock server for services that the SDK
	// hits directly (like replay). Strip the scheme for consistency
	// with how the real URLs are returned (just host:port).
	mockHost := strings.TrimPrefix(ms.Server.URL, "http://")
	ms.URLs.Replay = mockHost
	ms.URLs.Artifacts = mockHost
	ms.URLs.Hooks = mockHost
	// Search calls need a scheme because the mock listens on http, while
	// production Search URLs arrive hostname-only and default to https.
	// search.go honors an already-qualified URL here.
	ms.URLs.Search = "http://" + mockHost

	return ms
}

// Close shuts down the mock server.
func (ms *MockServer) Close() {
	ms.Server.Close()
}

// Calls returns all recorded HTTP calls. Thread-safe.
func (ms *MockServer) Calls() []MockCall {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	out := make([]MockCall, len(ms.calls))
	copy(out, ms.calls)
	return out
}

// ResetCalls clears the recorded call log.
func (ms *MockServer) ResetCalls() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.calls = nil
}

func (ms *MockServer) recordCall(r *http.Request, body string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.calls = append(ms.calls, MockCall{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   body,
		Time:   time.Now(),
	})
}

// NewClient creates a Client that talks to this mock server.
func (ms *MockServer) NewClient() (*Client, error) {
	return &Client{
		options: ClientOptions{
			OID:    ms.OID,
			APIKey: uuid.New().String(),
			JWT:    "mock-jwt-token",
		},
		logger:     &LCLoggerEmpty{},
		httpClient: ms.Server.Client(),
		baseURL:    ms.Server.URL,
		jwtURL:     ms.Server.URL + "/jwt",
	}, nil
}

// NewOrganization creates an Organization that talks to this mock server.
func (ms *MockServer) NewOrganization() (*Organization, error) {
	c, err := ms.NewClient()
	if err != nil {
		return nil, err
	}
	return &Organization{
		client: c,
		logger: c.logger,
	}, nil
}

func (ms *MockServer) registerRoutes(mux *http.ServeMux) {
	// JWT endpoint
	mux.HandleFunc("/jwt", ms.handleJWT)

	// WhoAmI
	mux.HandleFunc(fmt.Sprintf("/v1/who"), ms.handleWhoAmI)

	// Organization info
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s", ms.OID), ms.handleOrgInfo)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/url", ms.OID), ms.handleOrgURLs)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/quota", ms.OID), ms.handleOrgQuota)
	mux.HandleFunc(fmt.Sprintf("/v1/online/%s", ms.OID), ms.handleOnline)

	// D&R Rules
	mux.HandleFunc(fmt.Sprintf("/v1/rules/%s", ms.OID), ms.handleDRRules)

	// FP Rules
	mux.HandleFunc(fmt.Sprintf("/v1/fp/%s", ms.OID), ms.handleFPRules)

	// Installation Keys
	mux.HandleFunc(fmt.Sprintf("/v1/installationkeys/%s/", ms.OID), ms.handleInstallationKeyByID)
	mux.HandleFunc(fmt.Sprintf("/v1/installationkeys/%s", ms.OID), ms.handleInstallationKeys)

	// Outputs
	mux.HandleFunc(fmt.Sprintf("/v1/outputs/%s", ms.OID), ms.handleOutputs)

	// Sensors
	mux.HandleFunc(fmt.Sprintf("/v1/sensors/%s", ms.OID), ms.handleSensorsList)
	mux.HandleFunc(fmt.Sprintf("/v1/tags/%s/", ms.OID), ms.handleSensorsWithTag)
	mux.HandleFunc(fmt.Sprintf("/v1/tags/%s", ms.OID), ms.handleAllTags)

	// Users
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/users/permissions", ms.OID), ms.handleUserPermissions)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/users/role", ms.OID), ms.handleUserRole)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/users", ms.OID), ms.handleUsers)

	// Resources
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/resources", ms.OID), ms.handleResources)

	// Ingestion Keys
	mux.HandleFunc(fmt.Sprintf("/v1/insight/%s/ingestion_keys", ms.OID), ms.handleIngestionKeys)

	// Payloads - specific payload by name (must come before general)
	mux.HandleFunc(fmt.Sprintf("/v1/payload/%s/", ms.OID), ms.handlePayloadByName)
	mux.HandleFunc(fmt.Sprintf("/v1/payload/%s", ms.OID), ms.handlePayloads)

	// Extensions
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/subscriptions", ms.OID), ms.handleExtensionsList)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/subscription/extension/", ms.OID), ms.handleExtension)

	// Hive
	mux.HandleFunc("/v1/hive/", ms.handleHive)
	mux.HandleFunc("/v1/hive", ms.handleHiveBatch)

	// Service requests (exfil, logging/artifacts)
	mux.HandleFunc(fmt.Sprintf("/v1/service/%s/", ms.OID), ms.handleServiceRequest)

	// Org Values
	mux.HandleFunc(fmt.Sprintf("/v1/configs/%s/", ms.OID), ms.handleOrgValues)

	// Insight Objects
	mux.HandleFunc(fmt.Sprintf("/v1/insight/%s/objects/", ms.OID), ms.handleInsightObjects)

	// Hostnames
	mux.HandleFunc(fmt.Sprintf("/v1/hostnames/%s", ms.OID), ms.handleHostnames)

	// Billing
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/billing/status", ms.OID), ms.handleBillingStatus)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/billing/details", ms.OID), ms.handleBillingDetails)
	mux.HandleFunc(fmt.Sprintf("/v1/orgs/%s/billing/invoice/", ms.OID), ms.handleBillingInvoice)
	mux.HandleFunc("/v1/plans", ms.handleBillingPlans)
	mux.HandleFunc("/v1/user/self/auth", ms.handleBillingAuth)

	// Groups
	mux.HandleFunc("/v1/groups/concurrent", ms.handleGroupsConcurrent)
	mux.HandleFunc("/v1/groups/", ms.handleGroupByID)
	mux.HandleFunc("/v1/groups", ms.handleGroups)

	// Extension schema
	mux.HandleFunc("/v1/extension/schema/", ms.handleExtensionSchema)

	// Sensor-specific routes (SID-based) - must be last due to catch-all pattern
	mux.HandleFunc("/v1/", ms.handleSensorRoutes)
}

// --- JSON helpers ---

func (ms *MockServer) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (ms *MockServer) writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Dict{"error": msg})
}

func readBody(r *http.Request) string {
	b, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(b))
	return string(b)
}

// --- Route handlers ---

func (ms *MockServer) handleJWT(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.writeJSON(w, jwtResponse{JWT: "mock-jwt-token-refreshed"})
}

func (ms *MockServer) handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if ms.WhoAmIResponse != nil {
		ms.writeJSON(w, ms.WhoAmIResponse)
		return
	}
	orgs := []string{ms.OID}
	perms := []string{"org.get", "org.manage"}
	ident := "mock-user@test.com"
	ms.writeJSON(w, WhoAmIJsonResponse{
		Organizations: &orgs,
		Permissions:   &perms,
		Identity:      &ident,
	})
}

func (ms *MockServer) handleOrgInfo(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.writeJSON(w, ms.OrgInfo)
}

func (ms *MockServer) handleOrgURLs(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.writeJSON(w, SiteConnectivityInfo{
		URLs:     ms.URLs,
		Certs:    map[string]string{},
		SiteName: "mock",
	})
}

func (ms *MockServer) handleOrgQuota(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.writeJSON(w, Dict{"success": true})
}

func (ms *MockServer) handleOnline(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if r.Method == http.MethodGet {
		count := int64(0)
		for _, online := range ms.SensorOnline {
			if online {
				count++
			}
		}
		ms.writeJSON(w, OnlineCount{Count: count})
		return
	}

	// POST - check which SIDs are online
	r.Body = io.NopCloser(strings.NewReader(body))
	r.ParseForm()
	result := map[string]bool{}
	for _, sid := range r.Form["sids"] {
		result[sid] = ms.SensorOnline[sid]
	}
	ms.writeJSON(w, result)
}

// --- D&R Rules ---

func (ms *MockServer) handleDRRules(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		ms.writeJSON(w, ms.DRRules)
	case http.MethodPost:
		form := parseFormFromBody(body)
		name := form.Get("name")
		ms.DRRules[name] = Dict{
			"detect":     tryParseJSON(form.Get("detection")),
			"respond":    tryParseJSON(form.Get("response")),
			"is_enabled": form.Get("is_enabled") == "true",
			"namespace":  form.Get("namespace"),
		}
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		name := form.Get("name")
		delete(ms.DRRules, name)
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- FP Rules ---

func (ms *MockServer) handleFPRules(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		ms.writeJSON(w, ms.FPRules)
	case http.MethodPost:
		form := parseFormFromBody(body)
		name := form.Get("name")
		var det Dict
		json.Unmarshal([]byte(form.Get("rule")), &det)
		ms.FPRules[name] = FPRule{
			Detection: det,
			OID:       ms.OID,
			Name:      name,
		}
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		name := form.Get("name")
		delete(ms.FPRules, name)
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Installation Keys ---

func (ms *MockServer) handleInstallationKeys(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		keys := map[string]map[string]interface{}{}
		orgKeys := map[string]interface{}{}
		for iid, k := range ms.InstallationKeyStore {
			orgKeys[iid] = Dict{
				"desc":               k.Description,
				"iid":                k.ID,
				"key":                k.Key,
				"json_key":           k.JsonKey,
				"tags":               strings.Join(k.Tags, ","),
				"created":            k.CreatedAt,
				"use_public_root_ca": k.UsePublicCA,
			}
		}
		keys[ms.OID] = orgKeys
		ms.writeJSON(w, keys)
	case http.MethodPost:
		form := parseFormFromBody(body)
		iid := form.Get("iid")
		if iid == "" {
			iid = uuid.New().String()
		}
		tags := []string{}
		for _, t := range form["tags"] {
			if t != "" {
				tags = append(tags, t)
			}
		}
		ms.InstallationKeyStore[iid] = InstallationKey{
			ID:          iid,
			Description: form.Get("desc"),
			Tags:        tags,
			UsePublicCA: form.Get("use_public_root_ca") == "true",
			Key:         uuid.New().String(),
			JsonKey:     "{}",
			CreatedAt:   uint64(time.Now().Unix()),
		}
		ms.writeJSON(w, Dict{"iid": iid})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		iid := form.Get("iid")
		delete(ms.InstallationKeyStore, iid)
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleInstallationKeyByID(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Extract IID from path: /v1/installationkeys/{oid}/{iid}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		ms.writeError(w, 404, "not found")
		return
	}
	iid, _ := url.PathUnescape(parts[4])
	k, ok := ms.InstallationKeyStore[iid]
	if !ok {
		ms.writeError(w, 404, "not found")
		return
	}
	ms.writeJSON(w, Dict{
		"desc":               k.Description,
		"iid":                k.ID,
		"key":                k.Key,
		"json_key":           k.JsonKey,
		"tags":               strings.Join(k.Tags, ","),
		"created":            k.CreatedAt,
		"use_public_root_ca": k.UsePublicCA,
	})
}

// --- Outputs ---

func (ms *MockServer) handleOutputs(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		outputsByName := map[OutputName]interface{}{}
		for name, cfg := range ms.OutputStore {
			outputsByName[name] = Dict{
				"name":   cfg.Name,
				"module": cfg.Module,
				"for":    cfg.Type,
			}
		}
		ms.writeJSON(w, map[string]interface{}{ms.OID: outputsByName})
	case http.MethodPost:
		form := parseFormFromBody(body)
		name := form.Get("name")
		cfg := OutputConfig{
			Name:   name,
			Module: form.Get("module"),
			Type:   form.Get("type"),
		}
		ms.OutputStore[name] = cfg
		ms.writeJSON(w, Dict{
			"name":   name,
			"module": cfg.Module,
			"for":    cfg.Type,
		})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		name := form.Get("name")
		delete(ms.OutputStore, name)
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Sensors ---

func (ms *MockServer) handleSensorsList(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	sensors := []*Sensor{}
	for _, s := range ms.SensorStore {
		sensors = append(sensors, s)
	}

	compressed, _ := compressPayload(sensors)
	ms.writeJSON(w, rawSensorListPage{
		Sensors:           compressed,
		ContinuationToken: "",
	})
}

func (ms *MockServer) handleSensorsWithTag(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Extract tag from path: /v1/tags/{oid}/{tag}
	prefix := fmt.Sprintf("/v1/tags/%s/", ms.OID)
	tag, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, prefix))

	result := map[string][]string{}
	for sid, tags := range ms.SensorTags {
		for _, ti := range tags {
			if ti.Tag == tag {
				result[sid] = append(result[sid], tag)
			}
		}
	}
	ms.writeJSON(w, result)
}

func (ms *MockServer) handleAllTags(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	ms.writeJSON(w, Dict{"tags": ms.AllTags})
}

func (ms *MockServer) handleSensorRoutes(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	// Path: /v1/{sid} or /v1/{sid}/tags or /v1/{sid}/isolation
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	parts := strings.SplitN(path, "/", 2)
	sid := parts[0]
	subpath := ""
	if len(parts) > 1 {
		subpath = parts[1]
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch subpath {
	case "tags":
		ms.handleSensorTags(w, r, sid, body)
	case "isolation":
		ms.handleSensorIsolation(w, r, sid)
	default:
		ms.handleSensorDirect(w, r, sid, body)
	}
}

func (ms *MockServer) handleSensorDirect(w http.ResponseWriter, r *http.Request, sid string, body string) {
	switch r.Method {
	case http.MethodGet:
		s, ok := ms.SensorStore[sid]
		if !ok {
			ms.writeError(w, 404, "sensor not found")
			return
		}
		ms.writeJSON(w, sensorInfo{
			Info:     s,
			IsOnline: ms.SensorOnline[sid],
		})
	case http.MethodPost:
		// Tasking
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		delete(ms.SensorStore, sid)
		delete(ms.SensorTags, sid)
		delete(ms.SensorIsolation, sid)
		delete(ms.SensorOnline, sid)
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleSensorTags(w http.ResponseWriter, r *http.Request, sid string, body string) {
	switch r.Method {
	case http.MethodGet:
		tags := ms.SensorTags[sid]
		if tags == nil {
			tags = map[string]TagInfo{}
		}
		// TagInfo has a custom UnmarshalJSON that expects [sid, tag, by, addedTS] arrays
		tagArrays := map[string][]interface{}{}
		for _, ti := range tags {
			tagArrays[ti.Tag] = []interface{}{sid, ti.Tag, ti.By, ti.AddedTS}
		}
		ms.writeJSON(w, Dict{"tags": map[string]interface{}{sid: tagArrays}})
	case http.MethodPost:
		form := parseFormFromBody(body)
		tag := form.Get("tags")
		if ms.SensorTags[sid] == nil {
			ms.SensorTags[sid] = map[string]TagInfo{}
		}
		ms.SensorTags[sid][tag] = TagInfo{
			Tag:     tag,
			By:      "mock",
			AddedTS: time.Now().Format("2006-01-02 15:04:05"),
		}
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		tag := form.Get("tags")
		if ms.SensorTags[sid] != nil {
			delete(ms.SensorTags[sid], tag)
		}
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleSensorIsolation(w http.ResponseWriter, r *http.Request, sid string) {
	switch r.Method {
	case http.MethodPost:
		ms.SensorIsolation[sid] = true
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		ms.SensorIsolation[sid] = false
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Users ---

func (ms *MockServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		ms.writeJSON(w, Dict{"users": ms.UserEmails})
	case http.MethodPost:
		form := parseFormFromBody(body)
		email := form.Get("email")
		role := form.Get("role")
		found := false
		for _, e := range ms.UserEmails {
			if e == email {
				found = true
				break
			}
		}
		if !found {
			ms.UserEmails = append(ms.UserEmails, email)
		}
		if role != "" {
			ms.UserRoles[email] = role
		}
		ms.writeJSON(w, AddUserResponse{Success: true, Role: role})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		email := form.Get("email")
		newEmails := []string{}
		for _, e := range ms.UserEmails {
			if e != email {
				newEmails = append(newEmails, e)
			}
		}
		ms.UserEmails = newEmails
		delete(ms.UserPermissions, email)
		delete(ms.UserRoles, email)
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleUserPermissions(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		directUsers := []OrgUserInfo{}
		for _, email := range ms.UserEmails {
			directUsers = append(directUsers, OrgUserInfo{
				Email:       email,
				Permissions: ms.UserPermissions[email],
			})
		}
		ms.writeJSON(w, OrgUsersPermissions{
			UserPermissions: ms.UserPermissions,
			DirectUsers:     directUsers,
		})
	case http.MethodPost:
		form := parseFormFromBody(body)
		email := form.Get("email")
		perm := form.Get("perm")
		ms.UserPermissions[email] = append(ms.UserPermissions[email], perm)
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		email := form.Get("email")
		perm := form.Get("perm")
		perms := ms.UserPermissions[email]
		newPerms := []string{}
		for _, p := range perms {
			if p != perm {
				newPerms = append(newPerms, p)
			}
		}
		ms.UserPermissions[email] = newPerms
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleUserRole(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	form := parseFormFromBody(body)
	email := form.Get("email")
	role := form.Get("role")
	ms.UserRoles[email] = role
	ms.writeJSON(w, SetUserRoleResponse{Success: true, Role: role})
}

// --- Resources ---

func (ms *MockServer) handleResources(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		resourcesMap := map[string][]string{}
		for cat, names := range ms.ResourceStore {
			nameList := []string{}
			for name := range names {
				nameList = append(nameList, name)
			}
			resourcesMap[cat] = nameList
		}
		ms.writeJSON(w, Dict{"resources": resourcesMap})
	case http.MethodPost:
		form := parseFormFromBody(body)
		cat := form.Get("res_cat")
		name := form.Get("res_name")
		ms.ResourceStore.AddToCategory(cat, name)
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		form := parseFormFromBody(body)
		cat := form.Get("res_cat")
		name := form.Get("res_name")
		ms.ResourceStore.RemoveFromCategory(cat, name)
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Ingestion Keys ---

func (ms *MockServer) handleIngestionKeys(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		keys := Dict{}
		for name, key := range ms.IngestionKeyStore {
			keys[name] = key
		}
		ms.writeJSON(w, Dict{"keys": keys})
	case http.MethodPost:
		form := parseFormFromBody(body)
		name := form.Get("name")
		key := uuid.New().String()
		ms.IngestionKeyStore[name] = key
		ms.writeJSON(w, Dict{"name": name, "key": key})
	case http.MethodDelete:
		// name is in query string for delete
		name := r.URL.Query().Get("name")
		if name == "" {
			form := parseFormFromBody(body)
			name = form.Get("name")
		}
		delete(ms.IngestionKeyStore, name)
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Payloads ---

func (ms *MockServer) handlePayloads(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	ms.writeJSON(w, payloadsList{Payloads: ms.PayloadStore})
}

func (ms *MockServer) handlePayloadByName(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	prefix := fmt.Sprintf("/v1/payload/%s/", ms.OID)
	name, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, prefix))

	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		// Return a mock URL pointing back to our server for download
		ms.writeJSON(w, payloadGetPointer{URL: ms.Server.URL + "/mock-payload-download/" + url.PathEscape(name)})
	case http.MethodPost:
		// Return a mock URL for upload
		ms.writeJSON(w, payloadPutPointer{URL: ms.Server.URL + "/mock-payload-upload/" + url.PathEscape(name)})
	case http.MethodDelete:
		delete(ms.PayloadStore, name)
		delete(ms.PayloadData, name)
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Extensions ---

func (ms *MockServer) handleExtensionsList(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	d := Dict{}
	for name := range ms.ExtensionStore {
		d[name] = Dict{}
	}
	ms.writeJSON(w, d)
}

func (ms *MockServer) handleExtension(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	prefix := fmt.Sprintf("/v1/orgs/%s/subscription/extension/", ms.OID)
	name, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, prefix))

	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodPost:
		ms.ExtensionStore[name] = true
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodDelete:
		delete(ms.ExtensionStore, name)
		ms.writeJSON(w, Dict{"success": true})
	case http.MethodPatch:
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleExtensionSchema(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.writeJSON(w, Dict{"schema": Dict{}})
}

// --- Hive ---

func (ms *MockServer) handleHive(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	// Parse path: /v1/hive/{name}/{partition}[/{key}[/{target}]]
	path := strings.TrimPrefix(r.URL.Path, "/v1/hive/")
	parts := strings.SplitN(path, "/", 4)

	if len(parts) < 2 {
		ms.writeError(w, 400, "invalid hive path")
		return
	}
	hiveName := parts[0]
	partition := parts[1]
	storeKey := hiveName + "/" + partition

	ms.mu.Lock()
	defer ms.mu.Unlock()

	if len(parts) == 2 {
		// List: GET /v1/hive/{name}/{partition}
		records := ms.HiveStore[storeKey]
		if records == nil {
			records = map[string]HiveData{}
		}
		ms.writeJSON(w, records)
		return
	}

	key, _ := url.PathUnescape(parts[2])
	target := ""
	if len(parts) == 4 {
		target = parts[3]
	}

	switch r.Method {
	case http.MethodGet:
		records := ms.HiveStore[storeKey]
		if records == nil {
			ms.writeError(w, 404, "RECORD_NOT_FOUND")
			return
		}
		hd, ok := records[key]
		if !ok {
			ms.writeError(w, 404, "RECORD_NOT_FOUND")
			return
		}
		if target == "mtd" {
			hd.Data = nil
		}
		ms.writeJSON(w, hd)
	case http.MethodPost:
		if target == "rename" {
			form := parseFormFromBody(body)
			newName := form.Get("new_name")
			if ms.HiveStore[storeKey] == nil {
				ms.writeError(w, 404, "RECORD_NOT_FOUND")
				return
			}
			hd, ok := ms.HiveStore[storeKey][key]
			if !ok {
				ms.writeError(w, 404, "RECORD_NOT_FOUND")
				return
			}
			delete(ms.HiveStore[storeKey], key)
			ms.HiveStore[storeKey][newName] = hd
			guid := uuid.New().String()
			ms.writeJSON(w, HiveResp{
				Guid: guid,
				Hive: HiveInfo{Name: hiveName, Partition: partition},
				Name: newName,
			})
			return
		}

		form := parseFormFromBody(body)

		var data map[string]interface{}
		var usrMtd UsrMtd
		var sysMtd SysMtd

		// Check for compressed data first (from Add)
		if gzdata := form.Get("gzdata"); gzdata != "" {
			b64Dec, err := base64.StdEncoding.DecodeString(gzdata)
			if err == nil {
				gz, err := gzip.NewReader(bytes.NewReader(b64Dec))
				if err == nil {
					json.NewDecoder(gz).Decode(&data)
					gz.Close()
				}
			}
		}

		// Parse usr_mtd
		if umStr := form.Get("usr_mtd"); umStr != "" {
			json.Unmarshal([]byte(umStr), &usrMtd)
		}

		// Parse sys_mtd (for updates)
		if smStr := form.Get("sys_mtd"); smStr != "" {
			json.Unmarshal([]byte(smStr), &sysMtd)
		}

		// Parse uncompressed data (for Update)
		if dataStr := form.Get("data"); dataStr != "" && data == nil {
			json.Unmarshal([]byte(dataStr), &data)
		}

		if ms.HiveStore[storeKey] == nil {
			ms.HiveStore[storeKey] = map[string]HiveData{}
		}

		guid := uuid.New().String()
		etag := uuid.New().String()

		ms.HiveStore[storeKey][key] = HiveData{
			Data:   data,
			UsrMtd: usrMtd,
			SysMtd: SysMtd{
				Etag:       etag,
				GUID:       guid,
				CreatedBy:  "mock",
				CreatedAt:  time.Now().Unix(),
				LastAuthor: "mock",
				LastMod:    time.Now().Unix(),
			},
		}
		ms.writeJSON(w, HiveResp{
			Guid: guid,
			Hive: HiveInfo{Name: hiveName, Partition: partition},
			Name: key,
		})
	case http.MethodDelete:
		if ms.HiveStore[storeKey] != nil {
			delete(ms.HiveStore[storeKey], key)
		}
		ms.writeJSON(w, Dict{"success": true})
	}
}

func (ms *MockServer) handleHiveBatch(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	// Minimal batch support
	ms.writeJSON(w, hiveBatchResponses{Responses: []BatchResponse{}})
}

// --- Service Requests (exfil, artifacts) ---

func (ms *MockServer) handleServiceRequest(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	// Extract service name from path: /v1/service/{oid}/{service}
	prefix := fmt.Sprintf("/v1/service/%s/", ms.OID)
	service := strings.TrimPrefix(r.URL.Path, prefix)

	form := parseFormFromBody(body)
	reqDataB64 := form.Get("request_data")
	reqDataBytes, _ := base64.StdEncoding.DecodeString(reqDataB64)
	reqData := Dict{}
	json.Unmarshal(reqDataBytes, &reqData)

	action, _ := reqData["action"].(string)

	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch service {
	case "exfil":
		ms.handleExfilService(w, action, reqData)
	case "logging":
		ms.handleLoggingService(w, action, reqData)
	default:
		ms.writeJSON(w, Dict{})
	}
}

func (ms *MockServer) handleExfilService(w http.ResponseWriter, action string, data Dict) {
	switch action {
	case "list_rules":
		ms.writeJSON(w, ExfilRulesType{
			Events:  ms.ExfilEventRules,
			Watches: ms.ExfilWatchRules,
		})
	case "add_event_rule":
		name, _ := data["name"].(string)
		events := []string{}
		if ev, ok := data["events"].([]interface{}); ok {
			for _, e := range ev {
				if s, ok := e.(string); ok {
					events = append(events, s)
				}
			}
		}
		tags := toStringSlice(data["tags"])
		platforms := toStringSlice(data["platforms"])
		ms.ExfilEventRules[name] = ExfilRuleEvent{
			Events: events,
			Filters: ExfilEventFilters{
				Tags:      tags,
				Platforms: platforms,
			},
		}
		ms.writeJSON(w, Dict{"success": true})
	case "remove_event_rule":
		name, _ := data["name"].(string)
		delete(ms.ExfilEventRules, name)
		ms.writeJSON(w, Dict{"success": true})
	case "add_watch":
		name, _ := data["name"].(string)
		path := toStringSlice(data["path"])
		tags := toStringSlice(data["tags"])
		platforms := toStringSlice(data["platforms"])
		event, _ := data["event"].(string)
		value, _ := data["value"].(string)
		operator, _ := data["operator"].(string)
		ms.ExfilWatchRules[name] = ExfilRuleWatch{
			Event:    event,
			Value:    value,
			Path:     path,
			Operator: operator,
			Filters: ExfilEventFilters{
				Tags:      tags,
				Platforms: platforms,
			},
		}
		ms.writeJSON(w, Dict{"success": true})
	case "remove_watch":
		name, _ := data["name"].(string)
		delete(ms.ExfilWatchRules, name)
		ms.writeJSON(w, Dict{"success": true})
	default:
		ms.writeJSON(w, Dict{})
	}
}

func (ms *MockServer) handleLoggingService(w http.ResponseWriter, action string, data Dict) {
	switch action {
	case "list_rules":
		ms.writeJSON(w, ms.ArtifactRuleStore)
	case "add_rule":
		name, _ := data["name"].(string)
		patterns := toStringSlice(data["patterns"])
		tags := toStringSlice(data["tags"])
		platforms := toStringSlice(data["platforms"])
		isDeleteAfter, _ := data["is_delete_after"].(bool)
		isIgnoreCert, _ := data["is_ignore_cert"].(bool)
		var daysRetention uint
		if dr, ok := data["days_retention"].(float64); ok {
			daysRetention = uint(dr)
		}
		ms.ArtifactRuleStore[name] = ArtifactRule{
			Patterns:       patterns,
			IsDeleteAfter:  isDeleteAfter,
			IsIgnoreCert:   isIgnoreCert,
			DaysRetentions: daysRetention,
			Filters: ArtifactRuleFilter{
				Tags:      tags,
				Platforms: platforms,
			},
		}
		ms.writeJSON(w, Dict{"success": true})
	case "remove_rule":
		name, _ := data["name"].(string)
		delete(ms.ArtifactRuleStore, name)
		ms.writeJSON(w, Dict{"success": true})
	default:
		ms.writeJSON(w, Dict{})
	}
}

// --- Org Values ---

func (ms *MockServer) handleOrgValues(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	prefix := fmt.Sprintf("/v1/configs/%s/", ms.OID)
	name, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, prefix))

	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		val, ok := ms.OrgValues[name]
		if !ok {
			ms.writeError(w, 404, "not found")
			return
		}
		ms.writeJSON(w, OrgValueInfo{Name: name, Value: val})
	case http.MethodPost:
		form := parseFormFromBody(body)
		ms.OrgValues[name] = form.Get("value")
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Insight Objects ---

func (ms *MockServer) handleInsightObjects(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	prefix := fmt.Sprintf("/v1/insight/%s/objects/", ms.OID)
	objType := strings.TrimPrefix(r.URL.Path, prefix)
	name := r.URL.Query().Get("name")

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	key := objType + "/" + name
	if summary, ok := ms.IOCSummaries[key]; ok {
		ms.writeJSON(w, summary)
		return
	}
	ms.writeJSON(w, IOCSummaryResponse{
		Type: InsightObjectType(objType),
		Name: name,
	})
}

// --- Hostnames ---

func (ms *MockServer) handleHostnames(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	hostname := r.URL.Query().Get("hostname")

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	results, ok := ms.HostnameResults[hostname]
	if !ok {
		results = []HostnameSearchResult{}
	}
	ms.writeJSON(w, HostnameSearchResponse{Results: results})
}

// --- Billing ---

func (ms *MockServer) handleBillingStatus(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.writeJSON(w, ms.BillingStatus)
}

func (ms *MockServer) handleBillingDetails(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.writeJSON(w, ms.BillingDetails)
}

func (ms *MockServer) handleBillingInvoice(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.writeJSON(w, Dict{"url": "https://mock-invoice-url.example.com/invoice.pdf"})
}

func (ms *MockServer) handleBillingPlans(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.writeJSON(w, Dict{"plans": ms.BillingPlans})
}

func (ms *MockServer) handleBillingAuth(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.writeJSON(w, BillingUserAuthRequirements{})
}

// --- Groups ---

func (ms *MockServer) handleGroups(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)
	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		groups := []GroupListItem{}
		for gid, info := range ms.GroupStore {
			groups = append(groups, GroupListItem{GID: gid, Name: info.Name})
		}
		ms.writeJSON(w, Dict{"groups": groups})
	case http.MethodPost:
		form := parseFormFromBody(body)
		name := form.Get("name")
		gid := uuid.New().String()
		ms.GroupStore[gid] = &GroupInfo{
			GroupID: gid,
			Name:    name,
		}
		ms.writeJSON(w, GroupCreateResponse{
			Success: true,
			Data:    GroupCreateData{GID: gid},
		})
	}
}

func (ms *MockServer) handleGroupsConcurrent(w http.ResponseWriter, r *http.Request) {
	ms.recordCall(r, readBody(r))
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	groups := []GroupInfo{}
	for _, info := range ms.GroupStore {
		groups = append(groups, *info)
	}
	ms.writeJSON(w, Dict{"groups": groups})
}

func (ms *MockServer) handleGroupByID(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	ms.recordCall(r, body)

	// Parse path: /v1/groups/{gid}[/sub]
	path := strings.TrimPrefix(r.URL.Path, "/v1/groups/")
	parts := strings.SplitN(path, "/", 2)
	gid := parts[0]
	subpath := ""
	if len(parts) > 1 {
		subpath = parts[1]
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	switch subpath {
	case "":
		switch r.Method {
		case http.MethodGet:
			info, ok := ms.GroupStore[gid]
			if !ok {
				ms.writeError(w, 404, "group not found")
				return
			}
			ms.writeJSON(w, Dict{"group": info})
		case http.MethodDelete:
			delete(ms.GroupStore, gid)
			ms.writeJSON(w, Dict{"success": true})
		}
	case "users":
		form := parseFormFromBody(body)
		email := form.Get("member_email")
		info := ms.GroupStore[gid]
		if info == nil {
			ms.writeError(w, 404, "group not found")
			return
		}
		switch r.Method {
		case http.MethodPost:
			info.Members = append(info.Members, email)
		case http.MethodDelete:
			newMembers := []string{}
			for _, m := range info.Members {
				if m != email {
					newMembers = append(newMembers, m)
				}
			}
			info.Members = newMembers
		}
		ms.writeJSON(w, Dict{"success": true})
	case "owners":
		form := parseFormFromBody(body)
		email := form.Get("member_email")
		info := ms.GroupStore[gid]
		if info == nil {
			ms.writeError(w, 404, "group not found")
			return
		}
		switch r.Method {
		case http.MethodPost:
			info.Owners = append(info.Owners, email)
		case http.MethodDelete:
			newOwners := []string{}
			for _, o := range info.Owners {
				if o != email {
					newOwners = append(newOwners, o)
				}
			}
			info.Owners = newOwners
		}
		ms.writeJSON(w, Dict{"success": true})
	case "orgs":
		form := parseFormFromBody(body)
		oid := form.Get("oid")
		info := ms.GroupStore[gid]
		if info == nil {
			ms.writeError(w, 404, "group not found")
			return
		}
		switch r.Method {
		case http.MethodPost:
			info.Orgs = append(info.Orgs, GroupOrg{OrgID: oid})
			ms.writeJSON(w, Dict{"success": true})
		case http.MethodDelete:
			newOrgs := []GroupOrg{}
			for _, o := range info.Orgs {
				if o.OrgID != oid {
					newOrgs = append(newOrgs, o)
				}
			}
			info.Orgs = newOrgs
			ms.writeJSON(w, Dict{"success": true})
		}
	case "permissions":
		ms.writeJSON(w, Dict{"success": true})
	}
}

// --- Utility functions ---

func parseFormFromBody(body string) url.Values {
	vals, _ := url.ParseQuery(body)
	return vals
}

func tryParseJSON(s string) interface{} {
	var result interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return s
	}
	return result
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}
	if s, ok := v.([]string); ok {
		return s
	}
	if arr, ok := v.([]interface{}); ok {
		result := []string{}
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return []string{}
}

func compressPayload(data interface{}) (string, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	b64W := base64.NewEncoder(base64.StdEncoding, &buf)
	gzW := gzip.NewWriter(b64W)
	if _, err := gzW.Write(jsonBytes); err != nil {
		return "", err
	}
	if err := gzW.Close(); err != nil {
		return "", err
	}
	if err := b64W.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
