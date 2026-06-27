package limacharlie

import (
	"fmt"
	"time"
)

// GetAuditLogsOptions are the parameters for GetAuditLogs.
type GetAuditLogsOptions struct {
	// Start is the start of the window to query, in unix seconds.
	Start int64
	// End is the end of the window to query, in unix seconds.
	End int64
	// Limit caps the total number of audit entries returned (0 = no cap).
	Limit int
	// EventType restricts the results to a single audit event type.
	EventType string
	// SID restricts the results to a single sensor ID.
	SID string
}

// auditLogsResponse mirrors the paginated envelope of GET insight/{oid}/audit.
type auditLogsResponse struct {
	Events     []Dict `json:"events"`
	NextCursor string `json:"next_cursor"`
}

// GetAuditLogs fetches audit logs for the organization, following cursor
// pagination internally and returning the full aggregated slice.
//
// It mirrors python-limacharlie Organization.get_audit_logs (organization.py
// ~L1045): it issues GETs against insight/{oid}/audit starting with cursor "-"
// and follows next_cursor until it is empty. If Limit is set, iteration stops
// once that many entries have been collected.
func (org *Organization) GetAuditLogs(opts GetAuditLogsOptions) ([]Dict, error) {
	results := []Dict{}
	cursor := "-"

	for cursor != "" {
		query := Dict{
			"start":  fmt.Sprintf("%d", opts.Start),
			"end":    fmt.Sprintf("%d", opts.End),
			"cursor": cursor,
		}
		if opts.Limit != 0 {
			query["limit"] = fmt.Sprintf("%d", opts.Limit)
		}
		if opts.EventType != "" {
			query["event_type"] = opts.EventType
		}
		if opts.SID != "" {
			query["sid"] = opts.SID
		}

		resp := auditLogsResponse{}
		if err := org.GenericGETRequest(fmt.Sprintf("insight/%s/audit", org.GetOID()), query, &resp); err != nil {
			return nil, fmt.Errorf("failed to get audit logs: %w", err)
		}

		for _, entry := range resp.Events {
			results = append(results, entry)
			if opts.Limit != 0 && len(results) >= opts.Limit {
				return results, nil
			}
		}

		cursor = resp.NextCursor
	}

	return results, nil
}

// GetQuotaUsage returns the enforced sensor quota usage for the organization.
//
// It mirrors python-limacharlie Organization.get_quota_usage (organization.py
// ~L189), issuing a GET against quota_usage/{oid}. The response is the weighted
// virtual-sensor count the platform uses to decide whether a sensor may come
// online, along with the configured quota and a per-category breakdown.
func (org *Organization) GetQuotaUsage() (Dict, error) {
	resp := Dict{}
	if err := org.GenericGETRequest(fmt.Sprintf("quota_usage/%s", org.GetOID()), Dict{}, &resp); err != nil {
		return nil, fmt.Errorf("failed to get quota usage: %w", err)
	}
	return resp, nil
}

// GetGroupLogs returns the audit logs for a group.
//
// It mirrors python-limacharlie Organization.get_group_logs (organization.py
// ~L927), issuing a GET against groups/{groupID}/logs.
func (org *Organization) GetGroupLogs(groupID string) (Dict, error) {
	resp := Dict{}
	if err := org.GenericGETRequest(fmt.Sprintf("groups/%s/logs", groupID), Dict{}, &resp); err != nil {
		return nil, fmt.Errorf("failed to get logs for group %q: %w", groupID, err)
	}
	return resp, nil
}

// ResolveARL resolves an Authenticated Resource Locator and returns the data.
//
// It mirrors python-limacharlie ARL.get (sdk/arl.py ~L22), issuing a GET
// against arl/{oid} with the arl URL passed as a query parameter.
func (org *Organization) ResolveARL(arl string) (Dict, error) {
	resp := Dict{}
	if err := org.GenericGETRequest(fmt.Sprintf("arl/%s", org.GetOID()), Dict{"arl": arl}, &resp); err != nil {
		return nil, fmt.Errorf("failed to resolve ARL %q: %w", arl, err)
	}
	return resp, nil
}

// RenameOrg renames the organization.
//
// It mirrors python-limacharlie Organization.rename (organization.py ~L203),
// issuing a POST against orgs/{oid}/name with the new name as a query parameter.
func (org *Organization) RenameOrg(name string) (Dict, error) {
	resp := Dict{}
	if err := org.GenericPOSTRequest(fmt.Sprintf("orgs/%s/name", org.GetOID()), Dict{"name": name}, &resp); err != nil {
		return nil, fmt.Errorf("failed to rename org to %q: %w", name, err)
	}
	return resp, nil
}

// ExportSensors exports the full sensor manifest for the organization.
//
// It mirrors python-limacharlie Organization.export_sensors (organization.py
// ~L762), issuing a POST against export/{oid}/sensors.
func (org *Organization) ExportSensors() (Dict, error) {
	resp := Dict{}
	if err := org.GenericPOSTRequest(fmt.Sprintf("export/%s/sensors", org.GetOID()), Dict{}, &resp); err != nil {
		return nil, fmt.Errorf("failed to export sensors: %w", err)
	}
	return resp, nil
}

// ListAvailableExtensions lists all available extensions in the LimaCharlie
// marketplace.
//
// It mirrors python-limacharlie Extensions.get_all (sdk/extensions.py ~L63),
// issuing a GET against extension/definition.
func (org *Organization) ListAvailableExtensions() (Dict, error) {
	resp := Dict{}
	if err := org.GenericGETRequest("extension/definition", Dict{}, &resp); err != nil {
		return nil, fmt.Errorf("failed to list available extensions: %w", err)
	}
	return resp, nil
}

// MassTagResult summarizes the outcome of a MassTag/MassUntag operation.
type MassTagResult struct {
	// Selector is the sensor selector that was evaluated.
	Selector string
	// Tag is the tag that was applied or removed.
	Tag string
	// Matched is the number of sensors the selector matched.
	Matched int
	// Succeeded is the number of sensors successfully (un)tagged.
	Succeeded int
	// Errors maps a sensor ID to the error encountered for that sensor.
	Errors map[string]error
}

// MassTag adds a tag to all sensors matching a selector expression.
//
// It mirrors python-limacharlie Organization.mass_tag (organization.py ~L646).
// The Go SDK has no server-side mass-tag endpoint, so this resolves the
// selector via ListSensorsFromSelector and applies the tag per-sensor with
// Sensor.AddTag. A ttl of 0 means no expiry. The returned result reports how
// many sensors matched, how many were tagged, and any per-sensor errors.
func (org *Organization) MassTag(selector string, tag string, ttl int) (MassTagResult, error) {
	result := MassTagResult{
		Selector: selector,
		Tag:      tag,
		Errors:   map[string]error{},
	}

	sensors, err := org.ListSensorsFromSelector(selector)
	if err != nil {
		return result, fmt.Errorf("failed to list sensors for selector %q: %w", selector, err)
	}
	result.Matched = len(sensors)

	for sid, sensor := range sensors {
		if err := sensor.AddTag(tag, time.Duration(ttl)*time.Second); err != nil {
			result.Errors[sid] = err
			continue
		}
		result.Succeeded++
	}

	return result, nil
}

// MassUntag removes a tag from all sensors matching a selector expression.
//
// It mirrors python-limacharlie Organization.mass_untag (organization.py
// ~L670). Like MassTag, it resolves the selector via ListSensorsFromSelector
// and removes the tag per-sensor with Sensor.RemoveTag, returning a summary.
func (org *Organization) MassUntag(selector string, tag string) (MassTagResult, error) {
	result := MassTagResult{
		Selector: selector,
		Tag:      tag,
		Errors:   map[string]error{},
	}

	sensors, err := org.ListSensorsFromSelector(selector)
	if err != nil {
		return result, fmt.Errorf("failed to list sensors for selector %q: %w", selector, err)
	}
	result.Matched = len(sensors)

	for sid, sensor := range sensors {
		if err := sensor.RemoveTag(tag); err != nil {
			result.Errors[sid] = err
			continue
		}
		result.Succeeded++
	}

	return result, nil
}
