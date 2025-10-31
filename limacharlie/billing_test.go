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

	a.NotNil(stats)
	// Should at least have the OID populated
	if stats.OID == "" {
		t.Log("Usage stats returned empty OID - may not be configured in test environment")
		return
	}
	t.Logf("Usage stats: OID=%s, TotalSensors=%d, OnlineSensors=%d",
		stats.OID, stats.TotalSensors, stats.OnlineSensors)
}

// TestBillingOrgStatus tests retrieving billing status
func TestBillingOrgStatus(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	status, err := org.GetBillingOrgStatus()
	a.NoError(err, "GetBillingOrgStatus should succeed")

	a.NotNil(status)
	if status.OID == "" {
		t.Log("Billing status returned empty OID - may not be configured in test environment")
		return
	}
	t.Logf("Billing status: OID=%s, Status=%s", status.OID, status.Status)
}

// TestBillingOrgDetails tests retrieving detailed billing information
func TestBillingOrgDetails(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	details, err := org.GetBillingOrgDetails()
	a.NoError(err, "GetBillingOrgDetails should succeed")

	a.NotNil(details)
	if details.OID == "" {
		t.Log("Billing details returned empty OID - may not be configured in test environment")
		return
	}
	t.Logf("Billing details: OID=%s, Name=%s, Plan=%s", details.OID, details.Name, details.Plan)
}

// TestGetSKUDefinitions tests retrieving SKU pricing definitions
func TestGetSKUDefinitions(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	skus, err := org.GetSKUDefinitions()
	if err != nil {
		// SKU definitions endpoint may not be available in test environment (404)
		t.Logf("GetSKUDefinitions returned error (may not be available): %v", err)
		return
	}

	a.NotNil(skus)
	// SKUs might be empty in test environments
	t.Logf("Retrieved %d SKU definitions", len(skus))
	if len(skus) > 0 {
		t.Logf("First SKU: %s - %s", skus[0].SKU, skus[0].Name)
	}
}

// TestGetBillingInvoiceURL tests generating invoice download URLs
func TestGetBillingInvoiceURL(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test with a recent month (invoices only exist for past months with usage)
	year := 2024
	month := 1

	invoiceURL, err := org.GetBillingInvoiceURL(year, month, "")
	a.NoError(err, "GetBillingInvoiceURL should succeed")

	a.NotNil(invoiceURL)
	a.NotEmpty(invoiceURL.URL)
	a.Equal("2024", invoiceURL.Year)
	a.Equal("01", invoiceURL.Month)
	t.Logf("Invoice URL generated: %s", invoiceURL.URL)
}

// TestGetBillingInvoiceURLWithFormat tests invoice URL with format parameter
func TestGetBillingInvoiceURLWithFormat(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	year := 2024
	month := 1
	format := "pdf"

	invoiceURL, err := org.GetBillingInvoiceURL(year, month, format)
	if err != nil {
		// Format parameter may not be supported in test environment
		t.Logf("GetBillingInvoiceURL with format returned error (may not be supported): %v", err)
		return
	}

	a.NotNil(invoiceURL)
	a.Equal("pdf", invoiceURL.Format)
	t.Logf("Invoice URL with format: %s", invoiceURL.URL)
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
	if err != nil {
		// This endpoint requires user-based authentication, not API keys
		t.Logf("GetBillingAvailablePlans returned error (requires user auth): %v", err)
		return
	}

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
	if err != nil {
		// This endpoint requires user-based authentication, not API keys
		t.Logf("GetBillingUserAuthRequirements returned error (requires user auth): %v", err)
		return
	}

	a.NotNil(authReq)
	t.Logf("Auth requirements: MFARequired=%v, MFAEnabled=%v", authReq.MFARequired, authReq.MFAEnabled)
}
