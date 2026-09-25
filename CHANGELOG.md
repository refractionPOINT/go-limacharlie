# Changelog

All notable changes to this SDK are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This repository publishes no tags and no GitHub Releases, so consumers pin a commit and Go resolves a pseudo-version from it. "Unreleased" therefore means "on `master`", and the first release heading will appear here if and when the repository starts tagging.

This file starts at the entry below. Changes made before it are not reconstructed here; use `git log` for those.

## Unreleased

### Added

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

- **Search types.** `SearchRequest`, `SearchMode` (`SearchModeInteractive`, `SearchModeBatch`), `SearchPoll` with a `NextToken()` helper, `SearchResultItem`, `SearchStats`, `SearchPartialInfo`, `SearchValidation`, `SearchExecuteOptions` and `SearchPageHandler`.

- **`SearchStats` reports what a page actually ran as**, through `SearchMode`, `PageSize` and `PaginatedByteCap`, alongside the scan counters. All three are absent, and so read as zero, for a search that does not paginate. `SearchStats.SearchMode` is the field to trust: a mode the server does not recognise is ignored and the search runs interactively, and nothing else says so.

- **`SearchPartialInfo` surfaces a page that is known to be missing data.** A partial page is a successful response, so a client that ignores the block under-reports silently. Its `Reasons` is an open set; treat an unrecognised value as an omission, never as no omission.

- **`CancelSearch` closes a documented gap.** `search_queries.go` already told readers to cancel a search with the search API's delete endpoint, and there was no method to do it with.

- **Opt-in end-to-end tests** in `limacharlie/search_e2e_test.go`, which exercise the mode against a live organization. They skip themselves unless `LC_SEARCH_E2E` is set alongside `_OID` and `_KEY`, so they do not run in CI and are not part of this package's regression suite. They require the server-side search mode support to be deployed and fail until it ships; the skip message says so and names what to set. `LC_SEARCH_E2E_LOOKBACK_HOURS` and `LC_SEARCH_E2E_MAX_PAGES` size a run.

### Changed

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
