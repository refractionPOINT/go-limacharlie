# go-limacharlie
### Mission:
Go SDK and client library for the LimaCharlie security platform API. Provides programmatic access to LimaCharlie's endpoint detection and response (EDR) capabilities, including sensor management, detection rules, artifacts, organizational configurations, and data streaming through the firehose API.

### Resource Profile:
Memory and network bandwidth are most critical. The SDK makes API calls and handles data streaming from the firehose endpoint, which can consume significant network bandwidth when monitoring large deployments. CPU usage is generally minimal except during JSON parsing of large responses.

### Internal / External Dependencies: 
- External: LimaCharlie API (api.limacharlie.io, jwt.limacharlie.io)
- External: Google Cloud Storage (for artifact uploads)
- Required: Valid LimaCharlie Organization ID (OID) and API key for authentication
- Optional: JWT token for advanced authentication scenarios

### Testing Procedure
The tests run in a docker container using the `run_test.sh` script in the limacharlie directory.

Testing requires:
- Set environment variables: `LC_TEST_OID` (Organization ID) and `LC_TEST_KEY` (API Key)
- Run: `cd limacharlie && ./run_test.sh`
- For quick build verification: `go build ./...` in both limacharlie/ and firehose/ directories

### Notes
- The repository contains two Go modules: `limacharlie` (main SDK) and `firehose` (streaming data client)
- Tests require valid credentials with basic permissions (org.get)
- The SDK supports multiple authentication methods: API keys, UIDs, and JWTs
- Includes support for infrastructure-as-code workflows through sync functionality
- The firehose module provides real-time event streaming capabilities for monitoring sensor activity

### Syncing ACL scope membership

`KnownHives` and `SyncAll()` include the `acl` hive. To sync only scope membership
and the secrets it classifies, use explicit hive options:

```go
opts := limacharlie.SyncOptions{
    SyncHives: map[string]bool{"acl": true, "secret": true},
}
config, err := org.SyncFetch(opts)
// Handle err before saving config or passing it to org.SyncPush.
```

Scope membership is stored under `hives.acl`; `acl:` tags on other records remain
in `usr_mtd.tags`. Sync does not add classification tags automatically. The syncing
identity needs `acl.get` to fetch membership and `acl.set` to write membership or
add/remove `acl:` tags, in addition to the usual resource permissions. Reading
restricted content also requires the relevant scope membership; `acl.set` is not
an implicit read bypass. Do not push `acl_restricted: true` redaction markers as
replacement record data. Always handle errors returned by `SyncFetch` and `SyncPush`.

Existing Owner/Administrator assignments may need to be reapplied, or ACL
permissions granted explicitly, before an existing automation identity can sync ACLs.
