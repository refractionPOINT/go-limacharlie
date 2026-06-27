package limacharlie

// Cases SDK for LimaCharlie.
//
// Wraps the ext-cases REST API for SOC case lifecycle management,
// investigation tracking (entities, telemetry, artifacts), reporting,
// and configuration.
//
// The cases REST service lives on a separate host that is resolved from
// the organization's URL map (the "cases" key). Case creation is special:
// it goes through the LimaCharlie extension request mechanism
// (the "create_case" action on ext-cases) rather than the cases REST API.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// casesExtensionName is the extension used for case creation, which goes
// through the extension request mechanism rather than the cases REST API.
const casesExtensionName = "ext-cases"

// Cases is the cases system client for an Organization. Obtain one via
// Organization.Cases.
type Cases struct {
	o *Organization
}

// Cases returns a Cases client bound to this organization.
func (o *Organization) Cases() *Cases {
	return &Cases{o: o}
}

// oid returns the organization ID this client is bound to.
func (c *Cases) oid() string {
	return c.o.GetOID()
}

// request issues a request against the cases REST service. The service host
// is resolved from the organization's "cases" URL. The path is appended to
// "/api/v1/". When body is non-nil it is JSON-encoded and sent as the request
// body; queryParams, when non-nil, are sent as URL query parameters.
func (c *Cases) request(ctx context.Context, verb string, path string, queryParams Dict, body interface{}, resp interface{}) error {
	root, err := c.o.getServiceRoot("cases")
	if err != nil {
		return fmt.Errorf("failed to resolve cases service root: %w", err)
	}
	req := makeDefaultRequest(resp).withURLRoot(root)
	if queryParams != nil {
		req = req.withQueryData(queryParams)
	}
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal cases request body: %w", err)
		}
		req = req.withRawBody(raw, "application/json")
	}
	if err := c.o.client.reliableRequest(ctx, verb, "/api/v1/"+path, req); err != nil {
		return fmt.Errorf("cases request failed (%s %s): %w", verb, path, err)
	}
	return nil
}

// ------------------------------------------------------------------
// Cases
// ------------------------------------------------------------------

// CaseListFilters holds the optional filters, sorting, and pagination options
// for ListCases. All fields are optional; zero values are omitted from the
// request. List-valued filters are joined with commas, mirroring the Python SDK.
type CaseListFilters struct {
	// Status filters by case status (new, in_progress, resolved, closed).
	Status []string
	// Severity filters by case severity (critical, high, medium, low, info).
	Severity []string
	// Classification filters by classification (pending, true_positive, false_positive).
	Classification []string
	// Assignee filters by assignee email.
	Assignee string
	// Search is a full-text search across detection_cat and hostname on
	// linked CaseDetection records (not case-level fields).
	Search string
	// SensorID filters to cases with any detection from this sensor ID.
	SensorID string
	// Tag filters by tag (repeat for AND logic).
	Tag []string
	// Sort is the sort field (created_at, severity, case_number).
	Sort string
	// Order is the sort order (asc, desc).
	Order string
	// PageSize is the page size (1-200, default 50). Zero means unset.
	PageSize int
	// PageToken is the page token from a previous response.
	PageToken string
}

// ListCases lists cases with optional filtering and pagination.
func (c *Cases) ListCases(filters CaseListFilters) (Dict, error) {
	return c.ListCasesWithContext(context.Background(), filters)
}

// ListCasesWithContext lists cases with optional filtering and pagination,
// using the provided context.
func (c *Cases) ListCasesWithContext(ctx context.Context, filters CaseListFilters) (Dict, error) {
	qp := Dict{"oids": c.oid()}
	if len(filters.Status) != 0 {
		qp["status"] = strings.Join(filters.Status, ",")
	}
	if len(filters.Severity) != 0 {
		qp["severity"] = strings.Join(filters.Severity, ",")
	}
	if len(filters.Classification) != 0 {
		qp["classification"] = strings.Join(filters.Classification, ",")
	}
	if filters.Assignee != "" {
		qp["assignee"] = filters.Assignee
	}
	if filters.Search != "" {
		qp["search"] = filters.Search
	}
	if filters.SensorID != "" {
		qp["sid"] = filters.SensorID
	}
	if len(filters.Tag) != 0 {
		qp["tag"] = strings.Join(filters.Tag, ",")
	}
	if filters.Sort != "" {
		qp["sort"] = filters.Sort
	}
	if filters.Order != "" {
		qp["order"] = filters.Order
	}
	if filters.PageSize != 0 {
		qp["page_size"] = strconv.Itoa(filters.PageSize)
	}
	if filters.PageToken != "" {
		qp["page_token"] = filters.PageToken
	}
	var resp Dict
	if err := c.request(ctx, http.MethodGet, "cases", qp, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetCase gets a single case with its full event timeline.
func (c *Cases) GetCase(caseNumber int) (Dict, error) {
	return c.GetCaseWithContext(context.Background(), caseNumber)
}

// GetCaseWithContext gets a single case with its full event timeline, using
// the provided context.
func (c *Cases) GetCaseWithContext(ctx context.Context, caseNumber int) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("cases/%d", caseNumber), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateCaseOptions holds the optional inputs for CreateCase.
type CreateCaseOptions struct {
	// Detection is an optional full LC detection dict. The backend extracts
	// detect_id, cat, source, routing.sid, routing.hostname, and
	// detect_mtd.level automatically. Omit to create an empty investigation case.
	Detection Dict
	// Severity is an optional case severity override
	// (critical, high, medium, low, info).
	Severity string
	// Summary is an optional case summary set at creation time (max 8192 chars).
	Summary string
}

// CreateCase creates a new case via the ext-cases extension.
//
// Case creation goes through the LimaCharlie extension request mechanism
// (the create_case action) rather than the cases REST API.
func (c *Cases) CreateCase(opts CreateCaseOptions) (Dict, error) {
	data := Dict{}
	if opts.Detection != nil {
		// Pass the detection dict directly: the extension data encoding
		// already JSON-serializes the full data dict, so encoding it again
		// here would double-encode it into a string the backend drops.
		data["detection"] = opts.Detection
	}
	if opts.Severity != "" {
		data["severity"] = opts.Severity
	}
	if opts.Summary != "" {
		data["summary"] = opts.Summary
	}
	var resp Dict
	if err := c.o.ExtensionRequest(&resp, casesExtensionName, "create_case", data, false); err != nil {
		return nil, fmt.Errorf("failed to create case: %w", err)
	}
	return resp, nil
}

// UpdateCase updates a case. Only the provided fields are changed.
//
// Accepted fields: status, severity, assignees, classification, summary,
// conclusion, tags. Detection-level fields live on CaseDetection records,
// not on the Case itself; use AddDetection / ListDetections to manage them.
func (c *Cases) UpdateCase(caseNumber int, fields Dict) (Dict, error) {
	return c.UpdateCaseWithContext(context.Background(), caseNumber, fields)
}

// UpdateCaseWithContext updates a case using the provided context.
func (c *Cases) UpdateCaseWithContext(ctx context.Context, caseNumber int, fields Dict) (Dict, error) {
	body := Dict{}
	for k, v := range fields {
		if v != nil {
			body[k] = v
		}
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPatch, fmt.Sprintf("cases/%d", caseNumber), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// BulkUpdate bulk-updates up to 200 cases. Only the provided fields are
// changed across all the given cases.
func (c *Cases) BulkUpdate(caseNumbers []int, fields Dict) (Dict, error) {
	return c.BulkUpdateWithContext(context.Background(), caseNumbers, fields)
}

// BulkUpdateWithContext bulk-updates up to 200 cases using the provided context.
func (c *Cases) BulkUpdateWithContext(ctx context.Context, caseNumbers []int, fields Dict) (Dict, error) {
	update := Dict{}
	for k, v := range fields {
		if v != nil {
			update[k] = v
		}
	}
	body := Dict{
		"oid":          c.oid(),
		"case_numbers": caseNumbers,
		"update":       update,
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPost, "cases/bulk-update", nil, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Merge merges source cases into a target case.
func (c *Cases) Merge(targetCaseNumber int, sourceCaseNumbers []int) (Dict, error) {
	return c.MergeWithContext(context.Background(), targetCaseNumber, sourceCaseNumbers)
}

// MergeWithContext merges source cases into a target case using the provided context.
func (c *Cases) MergeWithContext(ctx context.Context, targetCaseNumber int, sourceCaseNumbers []int) (Dict, error) {
	body := Dict{
		"oid":                 c.oid(),
		"target_case_number":  targetCaseNumber,
		"source_case_numbers": sourceCaseNumbers,
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPost, "cases/merge", nil, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Notes
// ------------------------------------------------------------------

// AddNoteOptions holds the optional inputs for AddNote.
type AddNoteOptions struct {
	// NoteType is the note category (general, analysis, remediation,
	// escalation, handoff, to_stakeholder, from_stakeholder).
	NoteType string
	// IsPublic, when non-nil, sets whether the note is visible to
	// stakeholders (default false).
	IsPublic *bool
}

// AddNote adds a note to a case. content has a max length of 8192 chars.
func (c *Cases) AddNote(caseNumber int, content string, opts AddNoteOptions) (Dict, error) {
	return c.AddNoteWithContext(context.Background(), caseNumber, content, opts)
}

// AddNoteWithContext adds a note to a case using the provided context.
func (c *Cases) AddNoteWithContext(ctx context.Context, caseNumber int, content string, opts AddNoteOptions) (Dict, error) {
	body := Dict{"content": content}
	if opts.NoteType != "" {
		body["note_type"] = opts.NoteType
	}
	if opts.IsPublic != nil {
		body["is_public"] = *opts.IsPublic
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPost, fmt.Sprintf("cases/%d/notes", caseNumber), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateNoteVisibility updates a note's public visibility. eventID is the
// event ID of the note (from the case event timeline).
func (c *Cases) UpdateNoteVisibility(caseNumber int, eventID string, isPublic bool) (Dict, error) {
	return c.UpdateNoteVisibilityWithContext(context.Background(), caseNumber, eventID, isPublic)
}

// UpdateNoteVisibilityWithContext updates a note's public visibility using the
// provided context.
func (c *Cases) UpdateNoteVisibilityWithContext(ctx context.Context, caseNumber int, eventID string, isPublic bool) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodPatch, fmt.Sprintf("cases/%d/notes/%s", caseNumber, eventID), Dict{"oid": c.oid()}, Dict{"is_public": isPublic}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Detections
// ------------------------------------------------------------------

// ListDetections lists detections linked to a case.
func (c *Cases) ListDetections(caseNumber int) (Dict, error) {
	return c.ListDetectionsWithContext(context.Background(), caseNumber)
}

// ListDetectionsWithContext lists detections linked to a case using the
// provided context.
func (c *Cases) ListDetectionsWithContext(ctx context.Context, caseNumber int) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("cases/%d/detections", caseNumber), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddDetection links a detection to a case. detection is a full LC detection
// dict; the backend extracts detect_id, cat, source, routing, and detect_mtd
// automatically.
func (c *Cases) AddDetection(caseNumber int, detection Dict) (Dict, error) {
	return c.AddDetectionWithContext(context.Background(), caseNumber, detection)
}

// AddDetectionWithContext links a detection to a case using the provided context.
func (c *Cases) AddDetectionWithContext(ctx context.Context, caseNumber int, detection Dict) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodPost, fmt.Sprintf("cases/%d/detections", caseNumber), Dict{"oid": c.oid()}, Dict{"detection": detection}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RemoveDetection removes a detection link from a case.
func (c *Cases) RemoveDetection(caseNumber int, detectionID string) (Dict, error) {
	return c.RemoveDetectionWithContext(context.Background(), caseNumber, detectionID)
}

// RemoveDetectionWithContext removes a detection link from a case using the
// provided context.
func (c *Cases) RemoveDetectionWithContext(ctx context.Context, caseNumber int, detectionID string) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodDelete, fmt.Sprintf("cases/%d/detections/%s", caseNumber, detectionID), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Entities (IOCs)
// ------------------------------------------------------------------

// EntityOptions holds the optional inputs for AddEntity and UpdateEntity.
type EntityOptions struct {
	// Note is an analyst note (max 2048 chars).
	Note string
	// Verdict is the verdict assessment.
	Verdict string
}

// ListEntities lists entities on a case.
func (c *Cases) ListEntities(caseNumber int) (Dict, error) {
	return c.ListEntitiesWithContext(context.Background(), caseNumber)
}

// ListEntitiesWithContext lists entities on a case using the provided context.
func (c *Cases) ListEntitiesWithContext(ctx context.Context, caseNumber int) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("cases/%d/entities", caseNumber), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddEntity adds an entity/IOC to a case. entityType is one of ip, domain,
// hash, url, user, email, file, process, registry, other. entityValue has a
// max length of 1024 chars.
func (c *Cases) AddEntity(caseNumber int, entityType string, entityValue string, opts EntityOptions) (Dict, error) {
	return c.AddEntityWithContext(context.Background(), caseNumber, entityType, entityValue, opts)
}

// AddEntityWithContext adds an entity/IOC to a case using the provided context.
func (c *Cases) AddEntityWithContext(ctx context.Context, caseNumber int, entityType string, entityValue string, opts EntityOptions) (Dict, error) {
	body := Dict{
		"entity_type":  entityType,
		"entity_value": entityValue,
	}
	if opts.Note != "" {
		body["note"] = opts.Note
	}
	if opts.Verdict != "" {
		body["verdict"] = opts.Verdict
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPost, fmt.Sprintf("cases/%d/entities", caseNumber), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateEntity updates an entity on a case.
func (c *Cases) UpdateEntity(caseNumber int, entityID string, opts EntityOptions) (Dict, error) {
	return c.UpdateEntityWithContext(context.Background(), caseNumber, entityID, opts)
}

// UpdateEntityWithContext updates an entity on a case using the provided context.
func (c *Cases) UpdateEntityWithContext(ctx context.Context, caseNumber int, entityID string, opts EntityOptions) (Dict, error) {
	body := Dict{}
	if opts.Note != "" {
		body["note"] = opts.Note
	}
	if opts.Verdict != "" {
		body["verdict"] = opts.Verdict
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPatch, fmt.Sprintf("cases/%d/entities/%s", caseNumber, entityID), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RemoveEntity removes an entity from a case.
func (c *Cases) RemoveEntity(caseNumber int, entityID string) (Dict, error) {
	return c.RemoveEntityWithContext(context.Background(), caseNumber, entityID)
}

// RemoveEntityWithContext removes an entity from a case using the provided context.
func (c *Cases) RemoveEntityWithContext(ctx context.Context, caseNumber int, entityID string) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodDelete, fmt.Sprintf("cases/%d/entities/%s", caseNumber, entityID), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SearchEntities searches for entities across cases by type and value.
func (c *Cases) SearchEntities(entityType string, entityValue string) (Dict, error) {
	return c.SearchEntitiesWithContext(context.Background(), entityType, entityValue)
}

// SearchEntitiesWithContext searches for entities across cases using the
// provided context.
func (c *Cases) SearchEntitiesWithContext(ctx context.Context, entityType string, entityValue string) (Dict, error) {
	qp := Dict{
		"oids":         c.oid(),
		"entity_type":  entityType,
		"entity_value": entityValue,
	}
	var resp Dict
	if err := c.request(ctx, http.MethodGet, "entities/search", qp, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Telemetry
// ------------------------------------------------------------------

// TelemetryOptions holds the optional inputs for AddTelemetry and UpdateTelemetry.
type TelemetryOptions struct {
	// Note is an analyst note (max 2048 chars).
	Note string
	// Verdict is the verdict assessment.
	Verdict string
}

// ListTelemetry lists telemetry references on a case.
func (c *Cases) ListTelemetry(caseNumber int) (Dict, error) {
	return c.ListTelemetryWithContext(context.Background(), caseNumber)
}

// ListTelemetryWithContext lists telemetry references on a case using the
// provided context.
func (c *Cases) ListTelemetryWithContext(ctx context.Context, caseNumber int) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("cases/%d/telemetry", caseNumber), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddTelemetry links a telemetry event reference to a case. event is a full
// LC event dict; the backend extracts routing.this (atom), routing.sid, and
// routing.event_type automatically.
func (c *Cases) AddTelemetry(caseNumber int, event Dict, opts TelemetryOptions) (Dict, error) {
	return c.AddTelemetryWithContext(context.Background(), caseNumber, event, opts)
}

// AddTelemetryWithContext links a telemetry event reference to a case using
// the provided context.
func (c *Cases) AddTelemetryWithContext(ctx context.Context, caseNumber int, event Dict, opts TelemetryOptions) (Dict, error) {
	body := Dict{"event": event}
	if opts.Note != "" {
		body["note"] = opts.Note
	}
	if opts.Verdict != "" {
		body["verdict"] = opts.Verdict
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPost, fmt.Sprintf("cases/%d/telemetry", caseNumber), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateTelemetry updates a telemetry reference on a case.
func (c *Cases) UpdateTelemetry(caseNumber int, telemetryID string, opts TelemetryOptions) (Dict, error) {
	return c.UpdateTelemetryWithContext(context.Background(), caseNumber, telemetryID, opts)
}

// UpdateTelemetryWithContext updates a telemetry reference on a case using the
// provided context.
func (c *Cases) UpdateTelemetryWithContext(ctx context.Context, caseNumber int, telemetryID string, opts TelemetryOptions) (Dict, error) {
	body := Dict{}
	if opts.Note != "" {
		body["note"] = opts.Note
	}
	if opts.Verdict != "" {
		body["verdict"] = opts.Verdict
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPatch, fmt.Sprintf("cases/%d/telemetry/%s", caseNumber, telemetryID), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RemoveTelemetry removes a telemetry reference from a case.
func (c *Cases) RemoveTelemetry(caseNumber int, telemetryID string) (Dict, error) {
	return c.RemoveTelemetryWithContext(context.Background(), caseNumber, telemetryID)
}

// RemoveTelemetryWithContext removes a telemetry reference from a case using
// the provided context.
func (c *Cases) RemoveTelemetryWithContext(ctx context.Context, caseNumber int, telemetryID string) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodDelete, fmt.Sprintf("cases/%d/telemetry/%s", caseNumber, telemetryID), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Artifacts
// ------------------------------------------------------------------

// ArtifactOptions holds the optional inputs for AddArtifact.
type ArtifactOptions struct {
	// ArtifactType is an optional artifact type (e.g., pcap, memory_dump).
	ArtifactType string
	// Note is an analyst note (max 2048 chars).
	Note string
	// Verdict is the verdict assessment.
	Verdict string
}

// ListArtifacts lists artifacts on a case.
func (c *Cases) ListArtifacts(caseNumber int) (Dict, error) {
	return c.ListArtifactsWithContext(context.Background(), caseNumber)
}

// ListArtifactsWithContext lists artifacts on a case using the provided context.
func (c *Cases) ListArtifactsWithContext(ctx context.Context, caseNumber int) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("cases/%d/artifacts", caseNumber), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddArtifact adds a forensic artifact reference to a case. path is the
// artifact path or location and source is the artifact source identifier.
func (c *Cases) AddArtifact(caseNumber int, path string, source string, opts ArtifactOptions) (Dict, error) {
	return c.AddArtifactWithContext(context.Background(), caseNumber, path, source, opts)
}

// AddArtifactWithContext adds a forensic artifact reference to a case using
// the provided context.
func (c *Cases) AddArtifactWithContext(ctx context.Context, caseNumber int, path string, source string, opts ArtifactOptions) (Dict, error) {
	body := Dict{"path": path, "source": source}
	if opts.ArtifactType != "" {
		body["artifact_type"] = opts.ArtifactType
	}
	if opts.Note != "" {
		body["note"] = opts.Note
	}
	if opts.Verdict != "" {
		body["verdict"] = opts.Verdict
	}
	var resp Dict
	if err := c.request(ctx, http.MethodPost, fmt.Sprintf("cases/%d/artifacts", caseNumber), Dict{"oid": c.oid()}, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RemoveArtifact removes an artifact from a case.
func (c *Cases) RemoveArtifact(caseNumber int, artifactID string) (Dict, error) {
	return c.RemoveArtifactWithContext(context.Background(), caseNumber, artifactID)
}

// RemoveArtifactWithContext removes an artifact from a case using the provided
// context.
func (c *Cases) RemoveArtifactWithContext(ctx context.Context, caseNumber int, artifactID string) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodDelete, fmt.Sprintf("cases/%d/artifacts/%s", caseNumber, artifactID), Dict{"oid": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Export
// ------------------------------------------------------------------

// ExportCase exports a case with all its components in a single object.
//
// It fetches the case (with event timeline), detections, entities, telemetry,
// and artifacts, and returns them combined under the keys "detections",
// "entities", "telemetry", and "artifacts".
func (c *Cases) ExportCase(caseNumber int) (Dict, error) {
	return c.ExportCaseWithContext(context.Background(), caseNumber)
}

// ExportCaseWithContext exports a case with all its components using the
// provided context.
func (c *Cases) ExportCaseWithContext(ctx context.Context, caseNumber int) (Dict, error) {
	result, err := c.GetCaseWithContext(ctx, caseNumber)
	if err != nil {
		return nil, err
	}
	detections, err := c.ListDetectionsWithContext(ctx, caseNumber)
	if err != nil {
		return nil, err
	}
	entities, err := c.ListEntitiesWithContext(ctx, caseNumber)
	if err != nil {
		return nil, err
	}
	telemetry, err := c.ListTelemetryWithContext(ctx, caseNumber)
	if err != nil {
		return nil, err
	}
	artifacts, err := c.ListArtifactsWithContext(ctx, caseNumber)
	if err != nil {
		return nil, err
	}
	result["detections"] = detections
	result["entities"] = entities
	result["telemetry"] = telemetry
	result["artifacts"] = artifacts
	return result, nil
}

// ------------------------------------------------------------------
// Reports
// ------------------------------------------------------------------

// ReportSummary returns a comprehensive SOC report with MTTA/MTTR/TP-FP
// metrics over the [timeFrom, timeTo] range (RFC3339). groupBy is optional
// and segments the data (e.g., by severity or region); pass "" to omit it.
func (c *Cases) ReportSummary(timeFrom string, timeTo string, groupBy string) (Dict, error) {
	return c.ReportSummaryWithContext(context.Background(), timeFrom, timeTo, groupBy)
}

// ReportSummaryWithContext returns a comprehensive SOC report using the
// provided context.
func (c *Cases) ReportSummaryWithContext(ctx context.Context, timeFrom string, timeTo string, groupBy string) (Dict, error) {
	qp := Dict{
		"oids": c.oid(),
		"from": timeFrom,
		"to":   timeTo,
	}
	if groupBy != "" {
		qp["group_by"] = groupBy
	}
	var resp Dict
	if err := c.request(ctx, http.MethodGet, "reports/summary", qp, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Dashboard
// ------------------------------------------------------------------

// DashboardCounts returns real-time case counts by status/severity with SLA
// breaches.
func (c *Cases) DashboardCounts() (Dict, error) {
	return c.DashboardCountsWithContext(context.Background())
}

// DashboardCountsWithContext returns real-time case counts using the provided
// context.
func (c *Cases) DashboardCountsWithContext(ctx context.Context) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, "dashboard/counts", Dict{"oids": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Configuration
// ------------------------------------------------------------------

// GetConfig gets the org cases configuration.
func (c *Cases) GetConfig() (Dict, error) {
	return c.GetConfigWithContext(context.Background())
}

// GetConfigWithContext gets the org cases configuration using the provided
// context.
func (c *Cases) GetConfigWithContext(ctx context.Context) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("config/%s", c.oid()), nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SetConfig updates the org cases configuration.
func (c *Cases) SetConfig(config Dict) (Dict, error) {
	return c.SetConfigWithContext(context.Background(), config)
}

// SetConfigWithContext updates the org cases configuration using the provided
// context.
func (c *Cases) SetConfigWithContext(ctx context.Context, config Dict) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodPut, fmt.Sprintf("config/%s", c.oid()), nil, config, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Assignees
// ------------------------------------------------------------------

// ListAssignees returns the list of unique assignees across cases.
func (c *Cases) ListAssignees() (Dict, error) {
	return c.ListAssigneesWithContext(context.Background())
}

// ListAssigneesWithContext returns the list of unique assignees using the
// provided context.
func (c *Cases) ListAssigneesWithContext(ctx context.Context) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, "assignees", Dict{"oids": c.oid()}, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Orgs
// ------------------------------------------------------------------

// ListOrgs lists organizations subscribed to ext-cases that the caller can access.
func (c *Cases) ListOrgs() (Dict, error) {
	return c.ListOrgsWithContext(context.Background())
}

// ListOrgsWithContext lists organizations subscribed to ext-cases using the
// provided context.
func (c *Cases) ListOrgsWithContext(ctx context.Context) (Dict, error) {
	var resp Dict
	if err := c.request(ctx, http.MethodGet, "orgs", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
