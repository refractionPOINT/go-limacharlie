# Changelog

All notable changes to this SDK are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This repository publishes no tags and no GitHub Releases, so consumers pin a commit and Go resolves a pseudo-version from it. "Unreleased" therefore means "on `master`", and the first release heading will appear here if and when the repository starts tagging.

This file starts at the entry below. Changes made before it are not reconstructed here; use `git log` for those.

## Unreleased

### Fixed

- **A fetch always skips a hive the API does not know.** `SyncFetch` and `SyncFetchWithReport` used to fail the whole org when a requested hive was one the API does not serve: some deployments have hives disabled (the investigation hives, for instance), so "sync everything" with an SDK or CLI that lists them failed there with `lc_error_code:UNKNOWN_HIVE`. A hive the API does not know holds nothing to back up, so it is now left out of `OrgConfig.Hives` and listed in `SyncFetchReport.SkippedHives` whether or not `SyncOptions.SkipUnauthorizedHives` is set. It is recognised from the structured `lc_error_code:UNKNOWN_HIVE` code in the response body, never from loose text. Every other error is unchanged, including `UNAUTHORIZED`, which stays fatal unless `SkipUnauthorizedHives` is set. `SyncPush` is unchanged too: a config with records for a hive the API does not know still fails, so a restore never silently drops data.

- **`SyncSkippedHive` has a `Reason`.** `SyncSkippedHive.Reason` (`json:"reason"`) is `"unauthorized"` (the credentials may not read the hive; `Permission` names the missing permission when the API gave one) or `"unknown_hive"` (the API does not know the hive; no `Permission`). The constants are `SyncSkipReasonUnauthorized` and `SyncSkipReasonUnknownHive`. This adds a field to the report, so callers that decode it keep working. No new permission is needed.

### Added

- **Sync carries the `investigation_pack` hive.** `KnownHives` and `SyncAll()` now include `investigation_pack`, the packs of shared checks the Investigation Engine's templates use, with the same treatment as `investigation_policy` and `investigation_plan`: records are passed through as written, and an existing record is written against the version listed at the start of the apply (compare-and-swap). A pack is applied after the settings record (`investigation_policy`) and before the templates (`investigation_plan`) that use it, so the engine never loads a template whose pack is not stored yet, and a pack write by an API key finds the settings record already in place. Removals of investigation records (forced pushes) now wait until every hive has been applied and run in the reverse order: templates, then packs, then the settings record. A refused write therefore removes nothing from these three hives. Every other hive removes as before.

  **New permissions: `investigation_pack` hive.** The pack hive uses the `investigation_plan` permission family: reading needs `investigation_plan.get.mtd` and `investigation_plan.get`; writing needs `investigation_plan.set` (+ `investigation_plan.set.mtd`), and removing a record needs `investigation_plan.del`. A key that already syncs `investigation_plan` needs no new permission, but Hive adds two rules for packs. An API key (as opposed to a person) can write a pack only when the enabled `investigation_policy/default` record carries the tag `allow-api-key-promotion` (setting that tag needs `investigation_policy.set.mtd`); without it every pack write fails and the push stops there. And a changed pack source must carry a higher `version` than the stored one, or the write is refused. Because the settings record is applied first, a push that removes the tag and also changes a pack fails on the pack: remove the tag in a separate push after the pack changes. A full `SyncFetch` with a key that lacks the read permission fails unless `SyncOptions.SkipUnauthorizedHives` is set. Like the other `investigation_*` hives it exists only in orgs where the Investigation Engine is enabled; elsewhere a fetch skips it (reason `unknown_hive`).

- **`SyncFetchWithReport` skips hives the credentials may not read.** With `SyncOptions.SkipUnauthorizedHives` set, a hive in `SyncHives` whose read fails with the API's `lc_error_code:UNAUTHORIZED` error is left out of `OrgConfig.Hives` (no key at all, not an empty map) instead of failing the whole fetch, and is listed in `SyncFetchReport.SkippedHives` (sorted by hive name) with the hive, the missing permission when the message names one, and the error. Any other error stays fatal, and with the option unset behaviour is unchanged. This is for callers that expand "all hives" and so cannot know which hives a key covers: each new hive added to `KnownHives` otherwise breaks every backup run with a key that predates its permission. `SyncFetch` keeps its signature and calls `SyncFetchWithReport`. The unauthorized error is recognised from the structured error code in the response body, never from loose text. `syncFetchHive` now waits for every fetch and reports the first failure by hive name, so which error is returned no longer depends on timing.

- **Sync carries the `sensor_auto_upgrade` org value.** `SyncFetch` and `SyncPush` with `SyncOrgValues` now read and write `sensor_auto_upgrade`, which keeps an org's sensors on a sensor release channel. The value names the channel, `"stable"` or `"latest"`; `"true"` means stable. Unset, empty and `"false"` all mean off. Sync passes the value through as written, and the API refuses a value it does not know: an API from before channels accepts only `"true"` and `"false"`. Setting it takes the `module.update` permission as well as `org.conf.set`. An API that does not know the value yet answers the read with an error, which sync already treats as "not set", so this is safe against any deployment.

- **A forced push never clears `sensor_auto_upgrade`.** `SyncPush` with `IsForce` removes the org values a config does not mention. `sensor_auto_upgrade` is exempt: a config that does not name it leaves the org's value alone, so a template written before the value existed cannot turn off what an org's owner turned on. To turn it off, name it: `sensor_auto_upgrade: "false"`.

- **Sync carries Cloud Security, Email Security and Host Posture configuration.** `KnownHives` and `SyncAll()` now include `cloudsec_provider`, `cloudsec_policy`, `cloudsec_query`, `cloudsec_code_rule`, `mailsec_provider`, `mailsec_policy`, `dr-mail` and `host_posture_rule`. None of these records holds a credential: provider records hold only `hive://secret/<name>` references, which Hive enforces. The `secret` hive is synced before them, as before.

- **New permissions for the hives added to `KnownHives` and `SyncAll()`.** A "sync everything" run (`SyncAll()`, `SyncFetch`, `SyncPush`) now reads and writes the hives below. A key that predates this version lacks their permissions, and a full `SyncFetch` with such a key fails on the first hive it cannot read, unless `SyncOptions.SkipUnauthorizedHives` is set. With that option, unreadable hives are left out of the result and listed in `SyncFetchReport.SkippedHives`, with the missing permission when the API names one. Listing a hive's records needs its metadata permission (`.get.mtd`) and reading each record's data needs its `.get` permission; without the latter the record's data comes back `null`. Writing needs the set permission (plus `.set.mtd` where listed), and removing a record, which only a forced push does, needs the delete permission where one is listed. Where the read column lists a single permission (for example `cloudsec.get`), it covers both listing and reading data; where the write column lists no delete permission (for example `cloudsec.set`), the set permission also covers deletes.

  | Hive | Read (`SyncFetch`) | Write (`SyncPush`) |
  |---|---|---|
  | `cloudsec_provider` | `cloudsec_provider.get.mtd`, `cloudsec_provider.get` | `cloudsec_provider.set` (+ `cloudsec_provider.set.mtd`), `cloudsec_provider.del` |
  | `cloudsec_policy` | `cloudsec.get` | `cloudsec.set` |
  | `cloudsec_query` | `cloudsec.get` | `cloudsec.set` |
  | `cloudsec_code_rule` | `cloudsec.get` | `cloudsec.set` |
  | `mailsec_provider` | `mailsec_provider.get.mtd`, `mailsec_provider.get` | `mailsec_provider.set` (+ `mailsec_provider.set.mtd`), `mailsec_provider.del` |
  | `mailsec_policy` | `mailsec.get` | `mailsec.set` |
  | `dr-mail` | `mailsec.get` | `mailsec.set` |
  | `host_posture_rule` | `host_posture.get` | `host_posture.set` and `sensor.task` (removing a record needs only `host_posture.set`) |

  Some hives that were already in `KnownHives` before this changelog began follow the same rule, and a key made before they were added needs the same grants:

  | Hive | Read (`SyncFetch`) | Write (`SyncPush`) |
  |---|---|---|
  | `app_control_policy` | `app_control.get` | `app_control.set` |
  | `app_control_rule` | `app_control.get` | `app_control.set` |
  | `investigation_policy` | `investigation_policy.get.mtd`, `investigation_policy.get` | `investigation_policy.set` (+ `investigation_policy.set.mtd`), `investigation_policy.del` |
  | `investigation_plan` | `investigation_plan.get.mtd`, `investigation_plan.get` | `investigation_plan.set` (+ `investigation_plan.set.mtd`), `investigation_plan.del` |
  | `org_notes` | `org_notes.get.mtd`, `org_notes.get` | `org_notes.set` (+ `org_notes.set.mtd`), `org_notes.del` |
  | `sop` | `sop.get.mtd`, `sop.get` | `sop.set` (+ `sop.set.mtd`), `sop.del` |

  The `investigation_*` hives exist only in orgs where the Investigation Engine is enabled.

- **Hives are applied in a fixed order.** `syncHive` applies hives in `KnownHives` order, with any other hive after them by name. Map order used to decide which hive failed first and what was already written when it did. A product's provider records now come before the records that use them.

- **Tags Hive stamps are left out of sync.** Hive tags every `mailsec_provider` record `mailsec:provider:<provider>`. Sync no longer reads, exports or compares that tag, and never sends it. Before this, a connection written without the tag differed from the stored record on every sync and was rewritten each time.

- **Search API submission.** `limacharlie/search.go` adds the calls that start a search and read its pages, which the SDK previously had no way to do. `Organization.ListOpenQueries` and `Organization.GetSearchLimits` could already report on searches, but nothing could submit one.

  | Call | Endpoint |
  |---|---|
  | `InitiateSearch(req) (string, error)` | `POST /v1/search`, returns the query id |
  | `PollSearch(queryID, token) (*SearchPoll, error)` | `GET /v1/search/{queryId}`, one request |
  | `FetchSearchPage(ctx, queryID, token, opts) (*SearchPoll, error)` | polls one page to completion at the cadence the server advises |
  | `ExecuteSearch(ctx, req, opts, handler) error` | submits, then pages through to the end |
  | `CancelSearch(queryID) error` | `DELETE /v1/search/{queryId}` |
  | `ValidateSearch(req) (*SearchValidation, error)` | `POST /v1/search/validate` |

  Every call except `ExecuteSearch` has a `...WithContext` variant. The search host is resolved from the organization's URL map through `getServiceRoot("search")`, the same path the existing search calls use, and every request goes through the SDK's shared request layer, so JWT refresh, retry and error handling behave as they do everywhere else.

- **Searches are paginated by default.** A `SearchRequest` that leaves `Paginated` unset submits `"paginated": true`, and `ExecuteSearch` follows the continuation tokens to the last page, so a caller using it receives the whole result set. `WithPaginated(false)` asks for a single unpaginated response instead. A caller driving `PollSearch` or `FetchSearchPage` by hand follows `SearchPoll.NextToken()` itself.

- **Search types.** `SearchRequest`, `SearchMode` (`SearchModeInteractive`, `SearchModeBatch`), `SearchPoll` with a `NextToken()` helper, `SearchResultItem`, `SearchStats`, `SearchPartialInfo`, `SearchValidation`, `SearchExecuteOptions` and `SearchPageHandler`.

- **`SearchStats` reports what a page actually ran as**, through `SearchMode`, `PageSize` and `PaginatedByteCap`, alongside the scan counters. All three are absent, and so read as zero, for a search that does not paginate. `SearchStats.SearchMode` is the field to trust: a mode the server does not recognise, or declines for an organization, is treated as no mode so the organization's default decides, and nothing else says so.

- **`SearchPartialInfo` surfaces a page that is known to be missing data.** A partial page is a successful response, so a client that ignores the block under-reports silently. Its `Reasons` is an open set; treat an unrecognised value as an omission, never as no omission.

- **`CancelSearch` closes a documented gap.** `search_queries.go` already told readers to cancel a search with the search API's delete endpoint, and there was no method to do it with. A search that has already finished or expired answers 404, and the returned error carries that status (`errors.As` to a `RESTError`, then `StatusCode()`), so a caller that only wants the search stopped can treat it as done.

- **`ExecuteSearch` releases a search it stops reading.** Whenever it returns before the last page (the handler stopped, failed or panicked, a callback panicked, a page failed, the poll bound was reached, the context ended) it cancels the search server-side on a best-effort basis, so the search stops holding a concurrency slot. The cancel runs on its own five-second context that keeps the caller's context values. A search that ran to its last page is left alone.

- **Criteria that cannot describe a search are refused before anything is sent.** `InitiateSearch`, `ExecuteSearch` and `ValidateSearch` return an error, without a request, for an empty or blank query, a negative time, an end before the start, and a time past the year 5138, which is what a millisecond timestamp in a seconds field looks like. Without that check the server reads a millisecond value as seconds, searches a window thousands of years ahead and answers with no rows. Stream and mode values are not checked client-side, so a value the server adds later still reaches it.

- **Polling is bounded on both sides.** The server's suggested poll delay is honoured between the caller's `PollInterval` floor (500ms by default) and a one-minute ceiling, so a corrupt or very large suggestion cannot park a caller. `FetchSearchPage` reports its poll bound as soon as the last allowed poll comes back incomplete, without waiting out one more delay first.

- **`ExecuteSearch` cannot loop on one page.** A completed page whose continuation token is the token it was fetched with is handed to the handler, then reported as an error, because following it would fetch the same page forever.

- **Query ids address exactly one search.** `PollSearch` and `CancelSearch` escape the id as a single path segment, and refuse `.` and `..`, which URL normalization would resolve against the path to address a different endpoint.

- **End-to-end tests** in `limacharlie/search_e2e_test.go`, which exercise the Search calls against a live organization. `cloudbuild.yaml` sets `LC_SEARCH_E2E=1` next to the build's `_OID` and `_KEY`, so they run on every build, and passes `go test` an explicit `-timeout`. They run in parallel on one shared, once-authenticated organization, with concurrent searches capped at half the organization's concurrency limit (at most four), over two-hour windows (one settled hour for the cross-mode comparison). Each search is bounded at 90 seconds and every context ends before the `go test` deadline, so a hang fails fast and the server-side cancel still runs. Everywhere else they skip unless `LC_SEARCH_E2E` is set, because they start real, billed searches. A window with no events skips rather than fails. `LC_SEARCH_E2E_LOOKBACK_HOURS` and `LC_SEARCH_E2E_MAX_PAGES` size a run.

### Changed

- **`Sensor.Delete` documents the restore window.** A deleted sensor can be restored for 30 days after its deletion. After that the deletion is permanent, the sensor's tags are removed (except tags in the reserved `acl:` namespace) and the endpoint has to be enrolled again. `Delete` had no doc comment; it now says this. No behavior change.

- **A search submitted through this SDK asks for batch mode unless the caller says otherwise.** This is listed apart from the entry above because it is the one part of the new surface that decides something on the caller's behalf instead of passing their input through, and because it means the request this SDK sends is not the request a hand-written call with the same criteria would send.

  `POST /v1/search` treats `mode` as optional and resolves an absent one from the organization's own default. A `SearchRequest` with no mode set fills the field in with `"batch"` rather than leaving it out. The reasoning is the audience: what calls a Go SDK is a program that reads every page and pays a fixed cost per round trip, not a person watching one page at a time.

  **Nobody reads different data.** The result set and its ordering are identical under either mode. What moves is where one page ends and the next begins. Code that cares about page boundaries will see different numbers for the same query: a caller sizing a buffer per page, metering progress per page, or running a per-page timeout tuned to the other shape.

  To send no `mode` field at all, which is what an SDK without the field would have sent and what leaves the organization's own default in charge, call `SearchRequest.WithoutMode()`:

  ```go
  req := limacharlie.SearchRequest{
      Query:     "* | NEW_PROCESS | *",
      StartTime: start.Unix(),
      EndTime:   end.Unix(),
  }.WithoutMode()
  ```

  `WithoutMode()` and `WithMode(SearchModeInteractive)` are not two spellings of one request. The server resolves the mode in the order: a server-side override, then the mode in the submission, then the organization's default. An explicit mode therefore overrides that organization default, and an absent one lets it apply. A caller that wants the organization's setting honoured must use `WithoutMode()`, not an explicit interactive.

  `SearchRequest.Mode` is a `*SearchMode` to carry those three states: `nil` submits batch, a pointer to a mode submits that mode, and a pointer to the empty `SearchMode` submits no field. Use `WithMode` and `WithoutMode` rather than taking the address of a local.

### Fixed

- **`SelectorSIDs.MatchedCount` is documented as what the server returns.** The docs said it was the size of the whole match set, greater than `len(SIDs)` when `Truncated` is true. The server returns `len(SIDs)`: it stops resolving once it has proven that more than `Limit` sensors match, so the total is never computed, and `Truncated` is what reports that more matched. `MockServer` now answers the same way, so a test against the mock no longer sees a count the real server never sends.

- **`Dict` decodes every number `encoding/json` decodes.** A number written with an exponent (`1e-07`, `2E+21`), or an integer outside both 64-bit ranges, failed the decode of the whole response it arrived in. It now decodes as a `float64`. Integers that fit are still typed exactly, as `uint64` when non-negative and `int64` when negative. This matters most for Search result rows, which are `Dict`s holding arbitrary event bodies, but applies to every `Dict` and to `UnmarshalCleanJSON`.
