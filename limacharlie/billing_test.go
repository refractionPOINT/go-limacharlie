package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetUsageStats tests retrieving organization usage statistics
func TestGetUsageStats(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	stats, err := org.GetUsageStats()
	a.NoError(err, "GetUsageStats should succeed")
	a.NotNil(stats, "Usage stats should not be nil")

	t.Logf("Usage stats: OID=%s, TotalSensors=%d, OnlineSensors=%d",
		stats.OID, stats.TotalSensors, stats.OnlineSensors)
}

// TestBillingOrgStatus tests retrieving billing status
func TestBillingOrgStatus(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	status, err := org.GetBillingOrgStatus()
	a.NoError(err, "GetBillingOrgStatus should succeed")
	a.NotNil(status, "Billing status should not be nil")

	t.Logf("Billing status: IsPastDue=%v", status.IsPastDue)
}

// TestBillingOrgDetails tests retrieving detailed billing information
func TestBillingOrgDetails(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	details, err := org.GetBillingOrgDetails()
	a.NoError(err, "GetBillingOrgDetails should succeed")
	a.NotNil(details, "Billing details should not be nil")
	a.NotNil(details.Customer, "Customer details should not be nil")
	a.NotNil(details.Status, "Status should not be nil")

	t.Logf("Billing details retrieved successfully with customer and status data")
}

// TestGetBillingInvoiceURL tests generating invoice download URLs
func TestGetBillingInvoiceURL(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test with a recent month (invoices only exist for past months with usage)
	year := 2024
	month := 1

	response, err := org.GetBillingInvoiceURL(year, month, "")
	a.NoError(err, "GetBillingInvoiceURL should succeed")

	a.NotNil(response)
	a.Contains(response, "url", "Response should contain 'url' field")
	url, ok := response["url"].(string)
	a.True(ok, "url field should be a string")
	a.NotEmpty(url, "URL should not be empty")
	t.Logf("Invoice URL generated: %s", url)
}

// TestGetBillingInvoiceURLWithFormat tests invoice URL with format parameter
func TestGetBillingInvoiceURLWithFormat(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	year := 2024
	month := 1
	format := "json"

	response, err := org.GetBillingInvoiceURL(year, month, format)
	a.NoError(err)
	a.NotNil(response)
	// When format="json", server returns {"invoice": {...}} instead of {"url": "..."}
	a.Contains(response, "invoice", "Response should contain 'invoice' field when format=json")
	invoice, ok := response["invoice"].(map[string]interface{})
	a.True(ok, "invoice field should be a map")
	a.NotNil(invoice, "Invoice data should not be nil")
	t.Logf("Invoice data retrieved with format=json, invoice object has %d fields", len(invoice))
}

// TestGetBillingInvoiceURLValidation tests input validation
func TestGetBillingInvoiceURLValidation(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test invalid year
	_, err := org.GetBillingInvoiceURL(1999, 1, "")
	a.Error(err)
	a.Contains(err.Error(), "invalid year")

	// Test invalid month
	_, err = org.GetBillingInvoiceURL(2024, 13, "")
	a.Error(err)
	a.Contains(err.Error(), "invalid month")

	_, err = org.GetBillingInvoiceURL(2024, 0, "")
	a.Error(err)
	a.Contains(err.Error(), "invalid month")
}

// TestGetBillingAvailablePlans tests retrieving available billing plans
func TestGetBillingAvailablePlans(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	plans, err := org.GetBillingAvailablePlans()
	a.NoError(err)
	a.NotNil(plans)
	t.Logf("Retrieved %d billing plans", len(plans))
	for i, plan := range plans {
		t.Logf("Plan %d: %s - %s (%.2f %s)", i+1, plan.ID, plan.Name, plan.Price, plan.Currency)
	}
}

// TestGetBillingUserAuthRequirements tests retrieving user auth requirements
func TestGetBillingUserAuthRequirements(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	authReq, err := org.GetBillingUserAuthRequirements()
	a.NoError(err)
	a.NotNil(authReq)
	t.Logf("Auth requirements retrieved: %+v", authReq.Requirements)
}
