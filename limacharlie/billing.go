package limacharlie

import (
	"fmt"
	"net/http"
	"net/url"
)

const (
	billingRootURL = "https://billing.limacharlie.io"
)

// BillingOrgStatus contains the billing status information for an organization
type BillingOrgStatus struct {
	OID            string                 `json:"oid,omitempty"`
	Status         string                 `json:"status,omitempty"`
	BillingEmail   string                 `json:"billing_email,omitempty"`
	PaymentMethod  string                 `json:"payment_method,omitempty"`
	TrialEndDate   int64                  `json:"trial_end_date,omitempty"`
	SubscriptionID string                 `json:"subscription_id,omitempty"`
	Extra          map[string]interface{} `json:"extra,omitempty"`
}

// BillingOrgDetails contains detailed billing information for an organization
type BillingOrgDetails struct {
	OID              string                 `json:"oid,omitempty"`
	Name             string                 `json:"name,omitempty"`
	Plan             string                 `json:"plan,omitempty"`
	Status           string                 `json:"status,omitempty"`
	BillingEmail     string                 `json:"billing_email,omitempty"`
	PaymentMethod    string                 `json:"payment_method,omitempty"`
	CurrentPeriodEnd int64                  `json:"current_period_end,omitempty"`
	UsageThisMonth   map[string]interface{} `json:"usage_this_month,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

// BillingInvoiceURL contains the URL to download an invoice
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
type BillingUserAuthRequirements struct {
	MFARequired bool                   `json:"mfa_required,omitempty"`
	MFAEnabled  bool                   `json:"mfa_enabled,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// SKUDefinition contains pricing information for a specific SKU
type SKUDefinition struct {
	SKU         string                 `json:"sku,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Price       float64                `json:"price,omitempty"`
	Currency    string                 `json:"currency,omitempty"`
	Unit        string                 `json:"unit,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
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

// GetBillingInvoiceURL retrieves the URL to download an invoice for a specific month
// year: the year of the invoice (e.g., 2023)
// month: the month of the invoice (1-12)
// format: optional format parameter (e.g., "pdf", "csv")
func (org *Organization) GetBillingInvoiceURL(year, month int, format string) (*BillingInvoiceURL, error) {
	if year < 2000 || year > 3000 {
		return nil, fmt.Errorf("invalid year: %d", year)
	}
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month: %d (must be 1-12)", month)
	}

	var invoiceURL BillingInvoiceURL
	urlPath := fmt.Sprintf("orgs/%s/invoice_url/%d/%02d", org.GetOID(), year, month)

	request := makeDefaultRequest(&invoiceURL).withURLRoot(billingRootURL + "/")

	if format != "" {
		values := url.Values{}
		values.Set("format", format)
		request = request.withURLValues(values)
	}

	if err := org.client.reliableRequest(http.MethodGet, urlPath, request); err != nil {
		return nil, err
	}

	invoiceURL.Year = fmt.Sprintf("%d", year)
	invoiceURL.Month = fmt.Sprintf("%02d", month)
	if format != "" {
		invoiceURL.Format = format
	}

	return &invoiceURL, nil
}

// GetBillingAvailablePlans retrieves the list of available billing plans for the user
func (org *Organization) GetBillingAvailablePlans() ([]BillingPlan, error) {
	var plans []BillingPlan
	url := "user/self/plans"

	request := makeDefaultRequest(&plans).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return plans, nil
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

// GetSKUDefinitions retrieves the SKU pricing definitions for the organization
func (org *Organization) GetSKUDefinitions() ([]SKUDefinition, error) {
	var skus []SKUDefinition
	url := fmt.Sprintf("orgs/%s/sku-definitions", org.GetOID())

	request := makeDefaultRequest(&skus).withURLRoot(billingRootURL + "/")

	if err := org.client.reliableRequest(http.MethodGet, url, request); err != nil {
		return nil, err
	}

	return skus, nil
}
