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