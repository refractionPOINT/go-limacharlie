package limacharlie

import (
	"fmt"
	"net/http"
)

const (
	billingRootURL = "https://billing.limacharlie.io"
)

// BillingOrgStatus contains the billing status information for an organization
type BillingOrgStatus struct {
	IsPastDue bool `json:"is_past_due,omitempty"`
}

// BillingOrgDetails contains detailed billing information for an organization
// The structure matches the actual billing service response which includes
// Stripe customer and subscription objects
type BillingOrgDetails struct {
	Customer        map[string]interface{} `json:"customer,omitempty"`         // Stripe Customer object
	Status          map[string]interface{} `json:"status,omitempty"`           // Contains "is_past_due" bool
	UpcomingInvoice map[string]interface{} `json:"upcoming_invoice,omitempty"` // Stripe Invoice object
	Unified         map[string]interface{} `json:"unified,omitempty"`          // Optional unified billing data
}

// BillingInvoiceURL contains the URL to download an invoice
// Deprecated: GetBillingInvoiceURL now returns map[string]interface{} to support
// different response formats (url, invoice, lines, csv). This struct is kept for
// backward compatibility but is no longer used by the API.
type BillingInvoiceURL struct {
	URL    string `json:"url,omitempty"`
	Year   string `json:"year,omitempty"`
	Month  string `json:"month,omitempty"`
	Format string `json:"format,omitempty"`
}

// BillingPlan contains information about an available billing plan
type BillingPlan struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Price       float64                `json:"price,omitempty"`
	Currency    string                 `json:"currency,omitempty"`
	Features    []string               `json:"features,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// BillingUserAuthRequirements contains authentication requirements for the user
// The structure matches the actual billing service response
type BillingUserAuthRequirements struct {
	Requirements map[string]interface{} `json:"requirements,omitempty"` // Contains "methods", "mfa", etc.
}

// GetBillingOrgStatus retrieves the billing status for the organization
func (org *Organization) GetBillingOrgStatus() (*BillingOrgStatus, error) {
	var status BillingOrgStatus
	url := fmt.Sprintf("orgs/%s/status", org.GetOID())

	request := makeDefaultRequest(&status).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetBillingOrgDetails retrieves detailed billing information for the organization
func (org *Organization) GetBillingOrgDetails() (*BillingOrgDetails, error) {
	var details BillingOrgDetails
	url := fmt.Sprintf("orgs/%s/details", org.GetOID())

	request := makeDefaultRequest(&details).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return &details, nil
}

// GetBillingInvoiceURL retrieves invoice information for a specific month.
// The response structure varies based on the format parameter:
// - format="" (default): Returns {"url": "..."}
// - format="json": Returns {"invoice": {...}} (full Stripe Invoice object)
// - format="simple_json": Returns {"lines": [...]}
// - format="simple_csv": Returns {"csv": "..."}
//
// Parameters:
// - year: the year of the invoice (e.g., 2023)
// - month: the month of the invoice (1-12)
// - format: optional format parameter ("json", "simple_json", "simple_csv", or "" for URL only)
func (org *Organization) GetBillingInvoiceURL(year, month int, format string) (map[string]interface{}, error) {
	if year < 2000 || year > 3000 {
		return nil, fmt.Errorf("invalid year: %d", year)
	}
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month: %d (must be 1-12)", month)
	}

	response := make(map[string]interface{})
	urlPath := fmt.Sprintf("orgs/%s/invoice_url/%d/%02d", org.GetOID(), year, month)
	if format != "" {
		urlPath = fmt.Sprintf("%s?format=%s", urlPath, format)
	}

	request := makeDefaultRequest(&response).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	return response, nil
}

// GetBillingAvailablePlans retrieves the list of available billing plans for the user
func (org *Organization) GetBillingAvailablePlans() ([]BillingPlan, error) {
	// Server wraps response in {"plans": [...]}
	var response struct {
		Plans []BillingPlan `json:"plans"`
	}
	url := "user/self/plans"

	request := makeDefaultRequest(&response).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return response.Plans, nil
}

// GetBillingUserAuthRequirements retrieves the authentication requirements for the current user
func (org *Organization) GetBillingUserAuthRequirements() (*BillingUserAuthRequirements, error) {
	var authReq BillingUserAuthRequirements
	url := "user/self/auth"

	request := makeDefaultRequest(&authReq).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return &authReq, nil
}
