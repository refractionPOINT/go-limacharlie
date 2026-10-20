package limacharlie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeHiveGateway emulates the hive listing endpoint of the API gateway.
// legacy=true behaves like a gateway that predates paging: it ignores the
// limit, cursor and mtd_only query parameters. tooLargeAt makes the unpaged
// full listing answer 413 when the hive has more than that many records.
type fakeHiveGateway struct {
	mu         sync.Mutex
	legacy     bool
	tooLargeAt int
	records    map[string]HiveData
	queries    []map[string]string
}

func newFakeHiveGateway(n int) *fakeHiveGateway {
	g := &fakeHiveGateway{records: map[string]HiveData{}}
	for i := 0; i < n; i++ {
		g.records[fmt.Sprintf("rec-%03d", i)] = HiveData{
			Data:   map[string]interface{}{"i": float64(i)},
			SysMtd: SysMtd{Etag: fmt.Sprintf("etag-%d", i)},
			UsrMtd: UsrMtd{Enabled: true, Tags: []string{"t"}},
		}
	}
	return g
}

func (g *fakeHiveGateway) names() []string {
	names := make([]string, 0, len(g.records))
	for k := range g.records {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func stripped(m map[string]HiveData) map[string]HiveData {
	out := map[string]HiveData{}
	for k, v := range m {
		out[k] = HiveData{SysMtd: v.SysMtd, UsrMtd: v.UsrMtd}
	}
	return out
}

func (g *fakeHiveGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	q := r.URL.Query()
	flat := map[string]string{}
	for k := range q {
		flat[k] = q.Get(k)
	}
	g.queries = append(g.queries, flat)

	_, hasLimit := q["limit"]
	_, hasCursor := q["cursor"]
	paged := !g.legacy && (hasLimit || hasCursor)
	mtdOnly := !g.legacy && q.Get("mtd_only") == "true"

	if !paged {
		if !g.legacy && g.tooLargeAt > 0 && !mtdOnly && len(g.records) > g.tooLargeAt {
			http.Error(w, `{"error":"HIVE_LISTING_TOO_LARGE: page with limit"}`, http.StatusRequestEntityTooLarge)
			return
		}
		recs := g.records
		if mtdOnly {
			recs = stripped(recs)
		}
		_ = json.NewEncoder(w).Encode(recs)
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	start := 0
	if c := q.Get("cursor"); c != "" {
		if c == "bad" {
			http.Error(w, `{"error":"INVALID_CURSOR"}`, http.StatusBadRequest)
			return
		}
		start, _ = strconv.Atoi(c)
	}
	names := g.names()
	end := start + limit
	next := ""
	if end < len(names) {
		next = strconv.Itoa(end)
	} else {
		end = len(names)
	}
	page := map[string]HiveData{}
	for _, n := range names[start:end] {
		page[n] = g.records[n]
	}
	if mtdOnly {
		page = stripped(page)
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"records": page, "next_cursor": next})
}

func (g *fakeHiveGateway) client(t *testing.T) *HiveClient {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	return NewHiveClient(&Organization{client: &Client{
		options:    ClientOptions{OID: "00000000-0000-4000-8000-000000000001", JWT: "test-jwt"},
		logger:     &LCLoggerEmpty{},
		httpClient: srv.Client(),
		baseURL:    srv.URL,
	}})
}

var testHiveArgs = HiveArgs{HiveName: "lookup", PartitionKey: "00000000-0000-4000-8000-000000000001"}

func TestHiveListPageWalksCursor(t *testing.T) {
	g := newFakeHiveGateway(25)
	h := g.client(t)

	var pages []int
	got := HiveConfigData{}
	cursor := ""
	for {
		page, next, err := h.ListPage(testHiveArgs, ListPageOptions{Limit: 10, Cursor: cursor})
		require.NoError(t, err)
		pages = append(pages, len(page))
		for k, v := range page {
			got[k] = v
		}
		if next == "" {
			break
		}
		cursor = next
	}
	require.Equal(t, []int{10, 10, 5}, pages)
	require.Equal(t, HiveConfigData(g.records), got)
	require.Equal(t, "10", g.queries[0]["limit"])
	require.NotContains(t, g.queries[0], "cursor")
	require.Equal(t, "10", g.queries[1]["cursor"])
}

func TestHiveListPageAlwaysSendsLimit(t *testing.T) {
	g := newFakeHiveGateway(3)
	h := g.client(t)
	page, next, err := h.ListPage(testHiveArgs, ListPageOptions{})
	require.NoError(t, err)
	require.Len(t, page, 3)
	require.Empty(t, next)
	require.Equal(t, strconv.Itoa(DefaultHivePageSize), g.queries[0]["limit"], "limit selects the paged response")
}

func TestHiveListPageOptionsForwarded(t *testing.T) {
	g := newFakeHiveGateway(3)
	h := g.client(t)
	page, _, err := h.ListPage(testHiveArgs, ListPageOptions{Limit: 2, MtdOnly: true, TagPrefix: "pre", MaxBytes: 1234})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"limit": "2", "mtd_only": "true", "tag_prefix": "pre", "max_bytes": "1234"}, g.queries[0])
	for _, v := range page {
		require.Nil(t, v.Data)
	}
}

func TestHiveListPageEmptyHive(t *testing.T) {
	g := newFakeHiveGateway(0)
	h := g.client(t)
	page, next, err := h.ListPage(testHiveArgs, ListPageOptions{})
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Empty(t, page)
	require.Empty(t, next)
}

// A gateway that predates paging ignores limit and answers with the whole hive
// as a plain map: that must come back as one final page.
func TestHiveListPageLegacyGateway(t *testing.T) {
	g := newFakeHiveGateway(25)
	g.legacy = true
	h := g.client(t)

	page, next, err := h.ListPage(testHiveArgs, ListPageOptions{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, next)
	require.Equal(t, HiveConfigData(g.records), page)

	all, err := h.ListAllPages(testHiveArgs, ListPageOptions{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, HiveConfigData(g.records), all)
	require.Len(t, g.queries, 2, "a legacy answer is one request, not a loop")
}

func TestHiveListPageLegacyGatewayMtdOnlyStripsData(t *testing.T) {
	g := newFakeHiveGateway(5)
	g.legacy = true
	h := g.client(t)
	page, _, err := h.ListPage(testHiveArgs, ListPageOptions{MtdOnly: true})
	require.NoError(t, err)
	require.Len(t, page, 5)
	for _, v := range page {
		require.Nil(t, v.Data)
		require.NotEmpty(t, v.SysMtd.Etag)
	}
}

// Records named like the envelope keys must not be mistaken for an envelope.
func TestDecodeHiveListingLegacyRecordNamesLikeEnvelope(t *testing.T) {
	raw := `{"records":{"data":{"a":1},"sys_mtd":{},"usr_mtd":{}},"next_cursor":{"data":{"b":2},"sys_mtd":{},"usr_mtd":{}}}`
	recs, next, err := decodeHiveListing([]byte(raw))
	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, recs, 2)
	require.Contains(t, recs, "records")
	require.Contains(t, recs, "next_cursor")
}

func TestDecodeHiveListingShapes(t *testing.T) {
	for name, raw := range map[string]string{
		"null records": `{"records":null,"next_cursor":""}`,
		"null cursor":  `{"records":{},"next_cursor":null}`,
		"no cursor":    `{"records":{}}`,
		"empty map":    `{}`,
		"empty body":   ``,
		"null":         `null`,
	} {
		t.Run(name, func(t *testing.T) {
			recs, next, err := decodeHiveListing([]byte(raw))
			require.NoError(t, err)
			require.NotNil(t, recs)
			require.Empty(t, recs)
			require.Empty(t, next)
		})
	}
	_, _, err := decodeHiveListing([]byte(`[1,2]`))
	require.Error(t, err)
	_, _, err = decodeHiveListing([]byte(`{"records":{"a":5},"next_cursor":"x"}`))
	require.Error(t, err)
}

// A paged answer that omits (or nulls) next_cursor on its last page must still
// be read as an envelope, never as a legacy map holding one record "records".
func TestDecodeHiveListingEnvelopeWithoutCursor(t *testing.T) {
	for name, raw := range map[string]string{
		"absent": `{"records":{"a":{"data":{"x":1},"sys_mtd":{"etag":"e"},"usr_mtd":{}}}}`,
		"null":   `{"records":{"a":{"data":{"x":1},"sys_mtd":{"etag":"e"},"usr_mtd":{}}},"next_cursor":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			recs, next, err := decodeHiveListing([]byte(raw))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Len(t, recs, 1)
			require.Equal(t, "e", recs["a"].SysMtd.Etag)
		})
	}
}

// A legacy hive whose only record is named "records" is still a legacy map.
func TestDecodeHiveListingLegacySingleRecordNamedRecords(t *testing.T) {
	recs, next, err := decodeHiveListing([]byte(`{"records":{"data":{"x":1},"sys_mtd":{"etag":"e"},"usr_mtd":{}}}`))
	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, recs, 1)
	require.Equal(t, "e", recs["records"].SysMtd.Etag)
}

func TestHiveListMtdUsesMtdOnly(t *testing.T) {
	g := newFakeHiveGateway(5)
	h := g.client(t)
	got, err := h.ListMtd(testHiveArgs)
	require.NoError(t, err)
	require.Equal(t, HiveConfigData(stripped(g.records)), got)
	require.Equal(t, map[string]string{"mtd_only": "true"}, g.queries[0])
}

func TestHiveListMtdLegacyGatewayStripsData(t *testing.T) {
	g := newFakeHiveGateway(5)
	g.legacy = true
	h := g.client(t)
	got, err := h.ListMtd(testHiveArgs)
	require.NoError(t, err)
	require.Equal(t, HiveConfigData(stripped(g.records)), got)
}

func TestHiveListFallsBackToPagesOn413(t *testing.T) {
	g := newFakeHiveGateway(1200)
	g.tooLargeAt = 100
	h := g.client(t)
	got, err := h.List(testHiveArgs)
	require.NoError(t, err)
	require.Equal(t, HiveConfigData(g.records), got)

	// One refused full listing (not retried), then 3 pages of up to 500.
	require.Len(t, g.queries, 4)
	require.Empty(t, g.queries[0])
	require.Equal(t, "500", g.queries[1]["limit"])
}

func TestHiveListNoFallbackWhenSmall(t *testing.T) {
	g := newFakeHiveGateway(10)
	g.tooLargeAt = 100
	h := g.client(t)
	got, err := h.List(testHiveArgs)
	require.NoError(t, err)
	require.Equal(t, HiveConfigData(g.records), got)
	require.Len(t, g.queries, 1)
}

func TestHiveListMtdFallsBackToPagesOn413(t *testing.T) {
	var mu sync.Mutex
	var queries []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		q := r.URL.Query()
		queries = append(queries, map[string]string{"limit": q.Get("limit"), "mtd_only": q.Get("mtd_only")})
		if q.Get("limit") == "" {
			http.Error(w, "HIVE_LISTING_TOO_LARGE", http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = w.Write([]byte(`{"records":{"a":{"sys_mtd":{"etag":"e"},"usr_mtd":{}}},"next_cursor":""}`))
	}))
	t.Cleanup(srv.Close)
	h := NewHiveClient(&Organization{client: &Client{options: ClientOptions{JWT: "j"}, logger: &LCLoggerEmpty{}, httpClient: srv.Client(), baseURL: srv.URL}})

	got, err := h.ListMtd(testHiveArgs)
	require.NoError(t, err)
	require.Contains(t, got, "a")
	require.Equal(t, "true", queries[1]["mtd_only"])
}

func TestHiveListPagesStopsOnCallbackError(t *testing.T) {
	g := newFakeHiveGateway(25)
	h := g.client(t)
	calls := 0
	err := h.ListPages(testHiveArgs, ListPageOptions{Limit: 10}, func(HiveConfigData) error {
		calls++
		return fmt.Errorf("stop")
	})
	require.EqualError(t, err, "stop")
	require.Equal(t, 1, calls)
	require.Len(t, g.queries, 1)
	require.Error(t, h.ListPages(testHiveArgs, ListPageOptions{}, nil))
}

func TestHiveListPagesRejectsStuckCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"records":{},"next_cursor":"same"}`))
	}))
	t.Cleanup(srv.Close)
	h := NewHiveClient(&Organization{client: &Client{options: ClientOptions{JWT: "j"}, logger: &LCLoggerEmpty{}, httpClient: srv.Client(), baseURL: srv.URL}})
	err := h.ListPages(testHiveArgs, ListPageOptions{}, func(HiveConfigData) error { return nil })
	require.ErrorContains(t, err, "did not advance")
}

// 400 and 413 are definitive answers: they must be requested exactly once.
func TestHiveListErrorsAreNotRetried(t *testing.T) {
	g := newFakeHiveGateway(5)
	h := g.client(t)

	_, _, err := h.ListPage(testHiveArgs, ListPageOptions{Cursor: "bad"})
	require.ErrorContains(t, err, "INVALID_CURSOR")
	require.False(t, IsHiveListingTooLarge(err))
	require.Len(t, g.queries, 1)

	var restErr RESTError
	require.ErrorAs(t, err, &restErr)
	require.Equal(t, http.StatusBadRequest, restErr.StatusCode())

	g.queries = nil
	g.tooLargeAt = 1
	_, _, err = h.ListPage(testHiveArgs, ListPageOptions{})
	require.NoError(t, err, "paged requests are never refused as too large")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.queries = append(g.queries, nil)
		g.mu.Unlock()
		http.Error(w, "HIVE_LISTING_TOO_LARGE", http.StatusRequestEntityTooLarge)
	}))
	t.Cleanup(srv.Close)
	g.queries = nil
	h2 := NewHiveClient(&Organization{client: &Client{options: ClientOptions{JWT: "j"}, logger: &LCLoggerEmpty{}, httpClient: srv.Client(), baseURL: srv.URL}})
	_, _, err = h2.ListPage(testHiveArgs, ListPageOptions{})
	require.True(t, IsHiveListingTooLarge(err))
	require.Len(t, g.queries, 1)
}

func TestHiveListServerErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	h := NewHiveClient(&Organization{client: &Client{options: ClientOptions{JWT: "j"}, logger: &LCLoggerEmpty{}, httpClient: srv.Client(), baseURL: srv.URL}})
	_, err := h.List(testHiveArgs)
	require.Error(t, err)
	require.False(t, IsHiveListingTooLarge(err))
	_, err = h.ListMtd(testHiveArgs)
	require.Error(t, err)
}

func TestSyncFetchHiveConfigDataPagedAndLegacy(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			g := newFakeHiveGateway(30)
			g.legacy = legacy
			g.tooLargeAt = 10
			srv := httptest.NewServer(g)
			t.Cleanup(srv.Close)
			org := &Organization{client: &Client{options: ClientOptions{JWT: "j"}, logger: &LCLoggerEmpty{}, httpClient: srv.Client(), baseURL: srv.URL}}

			got, err := org.fetchHiveConfigData(testHiveArgs)
			require.NoError(t, err)
			require.Len(t, got, 30)
			require.Equal(t, g.records["rec-007"].Data, got["rec-007"].Data)
			require.Equal(t, g.records["rec-007"].UsrMtd, got["rec-007"].UsrMtd)
		})
	}
}
