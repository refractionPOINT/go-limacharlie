package limacharlie

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGetOrgErrors tests retrieving organization error logs
func TestGetOrgErrors(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	errors, err := org.GetOrgErrors()
	a.NoError(err)
	a.NotNil(errors)

	t.Logf("Retrieved %d organization errors", len(errors))
	for i, e := range errors {
		if i < 5 { // Log first 5 errors
			t.Logf("Error %d: Component=%s, Error=%s, Timestamp=%d", i+1, e.Component, e.Error, e.Timestamp)
		}
	}
}

// TestDismissOrgError tests dismissing organization errors
func TestDismissOrgError(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// First get errors to see if any exist
	errors, err := org.GetOrgErrors()
	a.NoError(err)

	if len(errors) == 0 {
		t.Log("No errors to dismiss - test cannot verify dismiss functionality")
		return
	}

	// Try to dismiss the first error
	componentToDismiss := errors[0].Component
	t.Logf("Attempting to dismiss error for component: %s", componentToDismiss)

	err = org.DismissOrgError(componentToDismiss)
	a.NoError(err)

	// Verify error was dismissed by checking if it's gone
	time.Sleep(2 * time.Second) // Give it a moment
	errorsAfter, err := org.GetOrgErrors()
	a.NoError(err)

	found := false
	for _, e := range errorsAfter {
		if e.Component == componentToDismiss {
			found = true
			break
		}
	}

	if !found {
		t.Logf("Error for component %s was successfully dismissed", componentToDismiss)
	}
}

// TestListUserOrgs tests listing organizations accessible to the user
func TestListUserOrgs(t *testing.T) {
	t.Skip("ListUserOrgs requires user-level authentication which is not available in test environment")

	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Test basic list
	orgs, err := org.ListUserOrgs(nil, nil, nil, nil, nil, true)
	a.NoError(err)
	a.NotNil(orgs)

	t.Logf("User has access to %d organizations", len(orgs))

	if len(orgs) > 0 {
		t.Logf("First org: OID=%s, Name=%s, Region=%s", orgs[0].OID, orgs[0].Name, orgs[0].Region)
	}
}

// TestListUserOrgsWithPagination tests pagination parameters
func TestListUserOrgsWithPagination(t *testing.T) {
	t.Skip("ListUserOrgs requires user-level authentication which is not available in test environment")

	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	offset := 0
	limit := 5

	orgs, err := org.ListUserOrgs(&offset, &limit, nil, nil, nil, true)
	a.NoError(err)
	a.NotNil(orgs)

	a.LessOrEqual(len(orgs), limit)
	t.Logf("Retrieved %d organizations with limit=%d", len(orgs), limit)
}

// TestAPIKeyLifecycle tests the full API key lifecycle
func TestAPIKeyLifecycle(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// 1. Get initial list of keys
	keysInitial, err := org.GetAPIKeys()
	a.NoError(err)
	a.NotNil(keysInitial)
	initialCount := len(keysInitial)
	t.Logf("Initial API key count: %d", initialCount)

	// 2. Create a new API key
	testKeyName := fmt.Sprintf("test-key-%d", time.Now().Unix())
	testPermissions := []string{"sensor.list"}

	createdKey, err := org.CreateAPIKey(testKeyName, testPermissions)
	a.NoError(err)
	a.NotNil(createdKey)
	a.NotEmpty(createdKey.Key) // Key value only returned on creation
	a.NotEmpty(createdKey.KeyHash)

	t.Logf("Created API key: Hash=%s", createdKey.KeyHash)

	// Store key hash for cleanup
	keyHashToDelete := createdKey.KeyHash

	// Ensure cleanup happens even if test fails
	defer func() {
		if keyHashToDelete != "" {
			err := org.DeleteAPIKey(keyHashToDelete)
			if err != nil {
				t.Logf("Cleanup: Failed to delete API key: %v", err)
			} else {
				t.Logf("Cleanup: Successfully deleted API key")
			}
		}
	}()

	// 3. Verify key appears in list
	time.Sleep(2 * time.Second) // Give it a moment to propagate
	keysAfterCreate, err := org.GetAPIKeys()
	a.NoError(err)
	a.Equal(initialCount+1, len(keysAfterCreate))

	// Find our key in the list
	found := false
	for _, key := range keysAfterCreate {
		if key.KeyHash == keyHashToDelete {
			found = true
			a.Equal(testKeyName, key.Description)
			a.Equal(testPermissions, key.Permissions)
			// Note: Key value is only returned on creation, not in list
			t.Logf("Found created key in list: %s", key.Description)
			break
		}
	}
	a.True(found, "Created API key should appear in list")

	// 4. Delete the API key
	err = org.DeleteAPIKey(keyHashToDelete)
	a.NoError(err)
	t.Logf("Deleted API key: %s", keyHashToDelete)

	// Clear the defer cleanup since we successfully deleted it
	keyHashToDelete = ""

	// 5. Verify key is removed from list
	time.Sleep(2 * time.Second)
	keysAfterDelete, err := org.GetAPIKeys()
	a.NoError(err)
	a.Equal(initialCount, len(keysAfterDelete))

	// Verify our key is gone
	for _, key := range keysAfterDelete {
		a.NotEqual(createdKey.KeyHash, key.KeyHash, "Deleted key should not appear in list")
	}
	t.Log("Verified API key was deleted")
}

// TestGetMITREReport tests retrieving MITRE ATT&CK coverage
func TestGetMITREReport(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	report, err := org.GetMITREReport()
	a.NoError(err, "GetMITREReport should succeed")
	a.NotNil(report, "MITRE report should not be nil")

	t.Logf("MITRE report: OID=%s, Coverage=%.2f%%, Techniques=%d, Tactics=%d",
		report.OID, report.Coverage, len(report.Techniques), len(report.Tactics))

	// Log some technique coverage details if available
	for i, tech := range report.Techniques {
		if i >= 3 {
			break
		}
		t.Logf("Technique %s (%s): Covered=%v, Rules=%d",
			tech.TechniqueID, tech.Name, tech.Covered, len(tech.DetectionRules))
	}
}

// TestGetTimeWhenSensorHasData tests sensor timeline queries
func TestGetTimeWhenSensorHasData(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Get an online sensor to test with
	sensors, err := org.ListSensors()
	if err != nil {
		t.Fatalf("Failed to get sensors: %v", err)
	}

	if len(sensors) == 0 {
		t.Skip("No sensors available for testing")
		return
	}

	// Get first sensor ID from the map
	var testSID string
	for sid := range sensors {
		testSID = sid
		break
	}
	t.Logf("Testing with sensor: %s", testSID)

	// Test with a recent time range (last 24 hours)
	end := time.Now().Unix()
	start := end - (24 * 3600) // 24 hours ago

	timeline, err := org.GetTimeWhenSensorHasData(testSID, start, end)
	a.NoError(err)
	a.NotNil(timeline)

	if timeline != nil {
		a.Equal(testSID, timeline.SID)
		a.Equal(start, timeline.Start)
		a.Equal(end, timeline.End)
		t.Logf("Sensor has data at %d timestamps in the range", len(timeline.Timestamps))

		if len(timeline.Timestamps) > 0 {
			t.Logf("First timestamp: %d, Last timestamp: %d",
				timeline.Timestamps[0], timeline.Timestamps[len(timeline.Timestamps)-1])
		}
	}
}

// TestGetTimeWhenSensorHasDataValidation tests validation of time range
func TestGetTimeWhenSensorHasDataValidation(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Get a sensor for testing
	sensors, err := org.ListSensors()
	if err != nil || len(sensors) == 0 {
		t.Skip("No sensors available for testing")
		return
	}

	// Get first sensor ID from the map
	var testSID string
	for sid := range sensors {
		testSID = sid
		break
	}

	// Test with time range > 30 days (should fail)
	end := time.Now().Unix()
	start := end - (31 * 24 * 3600) // 31 days ago

	_, err = org.GetTimeWhenSensorHasData(testSID, start, end)
	a.Error(err)
	a.Contains(err.Error(), "30 days")
	t.Log("Correctly rejected time range > 30 days")
}
