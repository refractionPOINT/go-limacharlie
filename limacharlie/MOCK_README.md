# LimaCharlie Go SDK - Mock Server

The `MockServer` lets you test code that uses the LimaCharlie Go SDK without connecting to the real API. It runs an in-process HTTP server that simulates the LimaCharlie API with in-memory state, so CRUD operations work naturally and you can inspect exactly what your code did.

## Quick Start

```go
package mypackage

import (
    "testing"

    lc "github.com/refractionPOINT/go-limacharlie/limacharlie"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    // Create a mock server with a fake OID
    ms := lc.NewMockServer("00000000-0000-0000-0000-000000000001")
    defer ms.Close()

    // Get an Organization that talks to the mock instead of the real API
    org, err := ms.NewOrganization()
    require.NoError(t, err)

    // Use it exactly like a real Organization
    err = org.DRRuleAdd("my-rule", lc.Dict{
        "event": "NEW_PROCESS",
        "op":    "contains",
        "path":  "event/FILE_PATH",
        "value": "malware.exe",
    }, lc.List{
        lc.Dict{"action": "report", "name": "malware-detected"},
    })
    require.NoError(t, err)

    // Verify state
    rules, err := org.DRRules()
    require.NoError(t, err)
    require.Contains(t, rules, "my-rule")
}
```

## Core Concepts

### MockServer

`MockServer` wraps Go's `httptest.Server` and holds all simulated state in exported fields. You create it with `NewMockServer(oid)`, then call `NewClient()` or `NewOrganization()` to get SDK objects that are wired to the mock.

```go
ms := lc.NewMockServer("00000000-0000-0000-0000-000000000001")
defer ms.Close()

// For org-level operations (most common)
org, _ := ms.NewOrganization()

// For user-level operations (groups, etc.)
client, _ := ms.NewClient()
```

### Pre-populating State

All state stores are exported fields on `MockServer`. Set them before or between SDK calls to simulate existing data:

```go
ms := lc.NewMockServer(oid)
defer ms.Close()

// Simulate an existing sensor fleet
sid := "9cbed57a-6d6a-4af0-b881-803a99b177d9"
ms.SensorStore[sid] = &lc.Sensor{
    OID:      oid,
    SID:      sid,
    Hostname: "prod-web-01",
    Platform: lc.Platforms.Linux,
}
ms.SensorOnline[sid] = true
ms.SensorTags[sid] = map[string]lc.TagInfo{
    "production": {Tag: "production", By: "admin@corp.com"},
}

// Simulate existing D&R rules
ms.DRRules["existing-rule"] = lc.Dict{
    "detect":  lc.Dict{"event": "NEW_PROCESS"},
    "respond": lc.List{lc.Dict{"action": "report", "name": "test"}},
}

// Simulate existing users
ms.UserEmails = []string{"admin@corp.com", "analyst@corp.com"}
ms.UserPermissions["admin@corp.com"] = []string{"org.get", "org.manage"}

org, _ := ms.NewOrganization()
// org.DRRules() now returns "existing-rule", sensors are queryable, etc.
```

### Inspecting Calls

Every HTTP request to the mock is recorded. Use this to verify your code made the right API calls:

```go
ms.ResetCalls() // Clear previous calls

// ... run your code ...

calls := ms.Calls()
for _, call := range calls {
    fmt.Printf("%s %s\n", call.Method, call.Path)
}

// Example: verify a sensor was tasked
for _, call := range ms.Calls() {
    if call.Method == "POST" && strings.Contains(call.Path, sid) {
        // Found the tasking call. call.Body has the form-encoded body.
    }
}
```

Each `MockCall` contains:
- `Method` - HTTP method (GET, POST, DELETE, etc.)
- `Path` - Request path (e.g., `/v1/rules/00000000-...`)
- `Body` - Raw request body
- `Time` - When the call was made

## How-To Guides

### Testing D&R Rule Management

```go
func TestRuleSync(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    // Add rules
    err := org.DRRuleAdd("detect-powershell", lc.Dict{
        "event": "NEW_PROCESS",
        "op":    "ends with",
        "path":  "event/FILE_PATH",
        "value": "powershell.exe",
    }, lc.List{
        lc.Dict{"action": "report", "name": "powershell-execution"},
    }, lc.NewDRRuleOptions{
        IsReplace: true,
        Namespace: "general",
        IsEnabled: true,
    })
    require.NoError(t, err)

    // Verify
    rules, err := org.DRRules()
    require.NoError(t, err)
    require.Contains(t, rules, "detect-powershell")

    // Delete
    err = org.DRRuleDelete("detect-powershell")
    require.NoError(t, err)

    rules, _ = org.DRRules()
    require.Empty(t, rules)
}
```

### Testing Sensor Operations

```go
func TestSensorWorkflow(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()

    // Set up a sensor
    sid := uuid.New().String()
    ms.SensorStore[sid] = &lc.Sensor{
        OID:        oid,
        SID:        sid,
        Hostname:   "endpoint-42",
        InternalIP: "10.0.1.42",
        ExternalIP: "203.0.113.42",
        Platform:   lc.Platforms.Windows,
    }
    ms.SensorOnline[sid] = true

    org, _ := ms.NewOrganization()

    // Fetch sensor info
    sensor := org.GetSensor(sid)
    require.Equal(t, "endpoint-42", sensor.Hostname)

    // Tag it
    err := sensor.AddTag("investigate", 24*time.Hour)
    require.NoError(t, err)

    tags, _ := sensor.GetTags()
    require.Len(t, tags, 1)

    // Send a task
    err = sensor.Task("os_version")
    require.NoError(t, err)

    // Isolate from network
    err = sensor.IsolateFromNetwork()
    require.NoError(t, err)
    require.True(t, ms.SensorIsolation[sid])

    // Rejoin
    err = sensor.RejoinNetwork()
    require.NoError(t, err)
    require.False(t, ms.SensorIsolation[sid])

    // List all sensors
    sensors, err := org.ListSensors()
    require.NoError(t, err)
    require.Contains(t, sensors, sid)

    // Check online status
    active, err := org.ActiveSensors([]string{sid})
    require.NoError(t, err)
    require.True(t, active[sid])
}
```

### Testing Hive (Key-Value Store)

```go
func TestHiveOperations(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    hc := lc.NewHiveClient(org)
    enabled := true
    expiry := int64(0)

    // Create a record
    resp, err := hc.Add(lc.HiveArgs{
        HiveName:     "ip-rep",
        PartitionKey: oid,
        Key:          "1.2.3.4",
        Data:         lc.Dict{"reputation": "malicious", "source": "threat-intel"},
        Enabled:      &enabled,
        Expiry:       &expiry,
    })
    require.NoError(t, err)
    require.NotEmpty(t, resp.Guid)

    // Read it back
    record, err := hc.Get(lc.HiveArgs{
        HiveName:     "ip-rep",
        PartitionKey: oid,
        Key:          "1.2.3.4",
    })
    require.NoError(t, err)
    require.Equal(t, "malicious", record.Data["reputation"])
    require.NotEmpty(t, record.SysMtd.Etag) // ETags are generated

    // Transactional update (uses ETags internally)
    _, err = hc.UpdateTx(lc.HiveArgs{
        HiveName:     "ip-rep",
        PartitionKey: oid,
        Key:          "1.2.3.4",
    }, func(record *lc.HiveData) (*lc.HiveData, error) {
        record.Data["reputation"] = "suspicious"
        return record, nil
    })
    require.NoError(t, err)

    // List all records
    records, err := hc.List(lc.HiveArgs{
        HiveName:     "ip-rep",
        PartitionKey: oid,
    })
    require.NoError(t, err)
    require.Len(t, records, 1)

    // Remove
    _, err = hc.Remove(lc.HiveArgs{
        HiveName:     "ip-rep",
        PartitionKey: oid,
        Key:          "1.2.3.4",
    })
    require.NoError(t, err)
}
```

### Testing User & Permission Management

```go
func TestUserManagement(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    // Add users
    _, err := org.AddUser("analyst@corp.com", false, lc.UserRoleOperator)
    require.NoError(t, err)

    _, err = org.AddUser("admin@corp.com", false, lc.UserRoleAdministrator)
    require.NoError(t, err)

    // Grant specific permissions
    err = org.AddUserPermission("analyst@corp.com", "dr.list")
    require.NoError(t, err)

    err = org.AddUserPermission("analyst@corp.com", "sensor.task")
    require.NoError(t, err)

    // Verify
    perms, err := org.GetUsersPermissions()
    require.NoError(t, err)
    require.Contains(t, perms.UserPermissions["analyst@corp.com"], "dr.list")
    require.Contains(t, perms.UserPermissions["analyst@corp.com"], "sensor.task")

    // Change role
    _, err = org.SetUserRole("analyst@corp.com", lc.UserRoleAdministrator)
    require.NoError(t, err)
    require.Equal(t, lc.UserRoleAdministrator, ms.UserRoles["analyst@corp.com"])

    // Remove user
    err = org.RemoveUser("analyst@corp.com")
    require.NoError(t, err)

    users, _ := org.GetUsers()
    require.Len(t, users, 1)
}
```

### Testing Output Configuration

```go
func TestOutputSetup(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    // Add an output
    _, err := org.OutputAdd(lc.OutputConfig{
        Name:   "siem-syslog",
        Module: lc.OutputTypes.Syslog,
        Type:   lc.OutputType.Detect,
    })
    require.NoError(t, err)

    // List outputs
    outputs, err := org.Outputs()
    require.NoError(t, err)
    require.Contains(t, outputs, "siem-syslog")

    // Delete
    _, err = org.OutputDel("siem-syslog")
    require.NoError(t, err)
}
```

### Testing Exfil Rules (via Service Request)

```go
func TestExfilRules(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    // Add an event exfil rule
    err := org.ExfilRuleEventAdd("critical-events", lc.ExfilRuleEvent{
        Events: []string{"NEW_PROCESS", "DNS_REQUEST", "NEW_TCP4_CONNECTION"},
        Filters: lc.ExfilEventFilters{
            Tags:      []string{"servers"},
            Platforms: []string{"linux"},
        },
    })
    require.NoError(t, err)

    // Add a watch rule
    err = org.ExfilRuleWatchAdd("watch-evil", lc.ExfilRuleWatch{
        Event:    "NEW_PROCESS",
        Value:    "evil.exe",
        Path:     []string{"event", "FILE_PATH"},
        Operator: "contains",
        Filters:  lc.ExfilEventFilters{Tags: []string{}, Platforms: []string{}},
    })
    require.NoError(t, err)

    // Verify
    rules, err := org.ExfilRules()
    require.NoError(t, err)
    require.Len(t, rules.Events, 1)
    require.Len(t, rules.Watches, 1)
}
```

### Testing Billing

```go
func TestBillingCheck(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    // Pre-populate billing state
    ms.BillingStatus = &lc.BillingOrgStatus{IsPastDue: false}
    ms.BillingPlans = []lc.BillingPlan{
        {ID: "free", Name: "Free", Price: 0},
        {ID: "pro", Name: "Professional", Price: 9.99},
    }

    status, err := org.GetBillingOrgStatus()
    require.NoError(t, err)
    require.False(t, status.IsPastDue)

    plans, err := org.GetBillingAvailablePlans()
    require.NoError(t, err)
    require.Len(t, plans, 2)
}
```

### Testing Groups (User-Level Auth)

```go
func TestGroupManagement(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()

    // Groups use Client, not Organization
    client, _ := ms.NewClient()

    // Create a group
    resp, err := client.CreateGroup("security-team")
    require.NoError(t, err)
    gid := resp.Data.GID

    // Manage members
    g := client.GetGroup(gid)
    err = g.AddMember("analyst@corp.com", false)
    require.NoError(t, err)

    err = g.AddOwner("lead@corp.com", false)
    require.NoError(t, err)

    // Verify
    info, err := g.GetInfo()
    require.NoError(t, err)
    require.Contains(t, info.Members, "analyst@corp.com")
    require.Contains(t, info.Owners, "lead@corp.com")

    // Add org to group
    org, _ := ms.NewOrganization()
    _, err = org.AddToGroup(gid)
    require.NoError(t, err)
}
```

### Custom WhoAmI Response

Control what permissions and identity your code sees:

```go
func TestAuthorization(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()

    // Set up a limited permission set
    orgs := []string{oid}
    perms := []string{"dr.list", "sensor.list"}
    ident := "limited-key@corp.com"
    ms.WhoAmIResponse = &lc.WhoAmIJsonResponse{
        Organizations: &orgs,
        Permissions:   &perms,
        Identity:      &ident,
    }

    org, _ := ms.NewOrganization()

    // This should pass (dr.list is in our permissions)
    _, _, err := org.Authorize([]string{"dr.list"})
    require.NoError(t, err)

    // This should fail (org.manage is not in our permissions)
    _, _, err = org.Authorize([]string{"org.manage"})
    require.Error(t, err)
}
```

## Advanced Usage

### Custom Handler Overrides

For endpoints the mock doesn't cover or when you need special behavior, register a custom handler:

```go
func TestCustomEndpoint(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()

    // Override D&R rules to return a specific error
    ms.CustomHandlers["/v1/rules/"] = func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost {
            w.WriteHeader(http.StatusForbidden)
            w.Write([]byte(`{"error": "insufficient permissions"}`))
            return
        }
        // Fall through to default for GET
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{}`))
    }

    org, _ := ms.NewOrganization()

    // This should fail with a REST error
    err := org.DRRuleAdd("test", lc.Dict{}, lc.List{})
    require.Error(t, err)
}
```

Custom handlers are matched by path prefix, so `"/v1/rules/"` catches all rule endpoints. They take priority over the built-in handlers.

### Multiple Independent Mock Servers

Each `MockServer` has isolated state, so you can test multi-org scenarios:

```go
func TestMultiOrg(t *testing.T) {
    ms1 := lc.NewMockServer("00000000-0000-0000-0000-000000000001")
    ms2 := lc.NewMockServer("00000000-0000-0000-0000-000000000002")
    defer ms1.Close()
    defer ms2.Close()

    org1, _ := ms1.NewOrganization()
    org2, _ := ms2.NewOrganization()

    // Changes to org1 don't affect org2
    org1.DRRuleAdd("rule-a", lc.Dict{}, lc.List{})

    rules1, _ := org1.DRRules()
    rules2, _ := org2.DRRules()
    require.Len(t, rules1, 1)
    require.Empty(t, rules2)
}
```

### Testing Code That Takes an Organization

If your code accepts an `*Organization`, just pass the mock-created one:

```go
// Your production code
func DeployDetectionPack(org *lc.Organization, rules map[string]lc.CoreDRRule) error {
    for name, rule := range rules {
        if err := org.DRRuleAdd(name, rule.Detect, rule.Response); err != nil {
            return err
        }
    }
    return nil
}

// Your test
func TestDeployDetectionPack(t *testing.T) {
    ms := lc.NewMockServer(oid)
    defer ms.Close()
    org, _ := ms.NewOrganization()

    pack := map[string]lc.CoreDRRule{
        "rule-1": {Detect: lc.Dict{"event": "NEW_PROCESS"}, Response: lc.List{}},
        "rule-2": {Detect: lc.Dict{"event": "DNS_REQUEST"}, Response: lc.List{}},
    }

    err := DeployDetectionPack(org, pack)
    require.NoError(t, err)

    // Verify all rules were created
    rules, _ := org.DRRules()
    require.Len(t, rules, 2)

    // Verify API calls
    calls := ms.Calls()
    postCalls := 0
    for _, c := range calls {
        if c.Method == "POST" && strings.Contains(c.Path, "rules") {
            postCalls++
        }
    }
    require.Equal(t, 2, postCalls)
}
```

## What's Covered

| Area | Operations |
|------|-----------|
| **Organization** | GetInfo, GetURLs, GetSiteConnectivityInfo, GetOnlineCount, SetQuota, WhoAmI, Authorize |
| **D&R Rules** | List, Add (with namespace/replace/TTL options), Delete |
| **FP Rules** | List, Add (with replace), Delete |
| **Installation Keys** | List, Get by IID, Add (auto or custom IID), Delete |
| **Outputs** | List, Add, Delete |
| **Sensors** | List, ListFromSelector, Get, Tags (add/remove/list), Isolation (isolate/rejoin), Delete, Task, ActiveSensors, GetAllTags, GetSensorsWithTag |
| **Devices** | AddTag |
| **Users** | List, Add, Remove, Permissions (add/remove/list), SetRole |
| **Resources** | List, Subscribe, Unsubscribe |
| **Ingestion Keys** | List, Create, Delete |
| **Payloads** | List, Delete |
| **Extensions** | List, Subscribe, Unsubscribe, ReKey, GetSchema |
| **Hive** | List, ListMtd, Get, GetMTD, Add, Update, UpdateTx, Remove, Rename |
| **Exfil Rules** | List, AddEvent, DeleteEvent, AddWatch, DeleteWatch |
| **Artifact Rules** | List, Add, Delete |
| **Org Values** | Get, Set |
| **Insight Objects** | SearchIOCSummary |
| **Hostnames** | SearchHostname |
| **Billing** | Status, Details, Plans, InvoiceURL, AuthRequirements |
| **Groups** | Create, List, ListConcurrent, GetInfo, Delete, Members (add/remove), Owners (add/remove), Orgs (add/remove), SetPermissions |
| **Service Requests** | Generic service request passthrough (exfil, logging) |
| **JWT** | RefreshJWT |

## What's Not Covered

These are intentionally skipped because they involve infrastructure that doesn't lend itself to simple HTTP mocking:

- **Firehose** - Requires a TLS listener and push-based event delivery
- **Spout / WebSocket streaming** - Requires a WebSocket server
- **SimpleRequest / Request** - Requires an active Spout
- **Artifact upload** - Multipart chunked upload to a separate ingestion service
- **Artifact export** - Polling loop with GCS integration
- **Payload upload/download** - Pre-signed URL redirect flows
- **Query / Replay** - Uses a separate replay service with its own HTTP client

For these, use `CustomHandlers` to stub the specific endpoints your code hits, or use integration tests against a real LimaCharlie environment.

## State Reference

All state fields on `MockServer` are exported for direct access:

| Field | Type | Description |
|-------|------|-------------|
| `OrgInfo` | `OrganizationInformation` | Returned by `GetInfo()` |
| `URLs` | `SiteURLs` | Returned by `GetURLs()` / `GetSiteConnectivityInfo()` |
| `DRRules` | `map[string]Dict` | D&R rules by name |
| `FPRules` | `map[FPRuleName]FPRule` | FP rules by name |
| `InstallationKeyStore` | `map[string]InstallationKey` | Installation keys by IID |
| `OutputStore` | `map[OutputName]OutputConfig` | Outputs by name |
| `SensorStore` | `map[string]*Sensor` | Sensors by SID |
| `SensorTags` | `map[string]map[string]TagInfo` | Tags by SID, then tag name |
| `SensorIsolation` | `map[string]bool` | Isolation state by SID |
| `SensorOnline` | `map[string]bool` | Online state by SID |
| `UserEmails` | `[]string` | User email list |
| `UserPermissions` | `map[string][]string` | Permissions by email |
| `UserRoles` | `map[string]string` | Roles by email |
| `ResourceStore` | `ResourcesByCategory` | Subscribed resources |
| `IngestionKeyStore` | `map[string]string` | Ingestion keys by name |
| `PayloadStore` | `map[PayloadName]Payload` | Payload metadata by name |
| `ExtensionStore` | `map[ExtensionName]bool` | Subscribed extensions |
| `HiveStore` | `map[string]map[string]HiveData` | Hive records by `"hiveName/partition"` then key |
| `ExfilEventRules` | `map[ExfilRuleName]ExfilRuleEvent` | Exfil event rules |
| `ExfilWatchRules` | `map[ExfilRuleName]ExfilRuleWatch` | Exfil watch rules |
| `ArtifactRuleStore` | `map[ArtifactRuleName]ArtifactRule` | Artifact collection rules |
| `OrgValues` | `map[string]string` | Org config values |
| `IOCSummaries` | `map[string]IOCSummaryResponse` | IOC data, keyed as `"type/name"` |
| `HostnameResults` | `map[string][]HostnameSearchResult` | Hostname search results |
| `BillingStatus` | `*BillingOrgStatus` | Billing status |
| `BillingDetails` | `*BillingOrgDetails` | Billing details |
| `BillingPlans` | `[]BillingPlan` | Available plans |
| `GroupStore` | `map[string]*GroupInfo` | Groups by GID |
| `WhoAmIResponse` | `*WhoAmIJsonResponse` | Override for WhoAmI (nil = default) |
| `AllTags` | `[]string` | Tags returned by `GetAllTags()` |
| `CustomHandlers` | `map[string]http.HandlerFunc` | Custom route overrides (prefix match) |
