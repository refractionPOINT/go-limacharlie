package limacharlie

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// FuzzSearchSubmitBody checks the submission body for any criteria a caller can
// build: it is always a JSON object, every value round-trips exactly, the
// optional keys are present exactly when the request asks for them, and
// nothing else appears. A body that lost or mangled a field would run a
// different search from the one the caller described.
func FuzzSearchSubmitBody(f *testing.F) {
	f.Add("* | * | *", int64(1700000000), int64(1700003600), "", "", false, false, false, true)
	f.Add("q\"\\\n </script>", int64(0), int64(0), "detection", "interactive", true, true, true, false)
	f.Add("", int64(-1), int64(1<<62), "a\x00b", "batché", true, false, true, true)

	f.Fuzz(func(t *testing.T, query string, start, end int64, stream, mode string,
		setMode, setPaginated, paginated, withoutMode bool) {
		if !utf8.ValidString(query) || !utf8.ValidString(stream) || !utf8.ValidString(mode) {
			// encoding/json replaces invalid UTF-8, so such strings cannot
			// round-trip by design; that is the encoder's contract, not ours.
			t.Skip()
		}
		req := SearchRequest{Query: query, StartTime: start, EndTime: end, Stream: stream}
		if setPaginated {
			req = req.WithPaginated(paginated)
		}
		wantMode := "batch"
		switch {
		case withoutMode:
			req = req.WithoutMode()
			wantMode = ""
		case setMode:
			req = req.WithMode(SearchMode(mode))
			wantMode = mode
		}

		body, err := buildSearchSubmitBody(searchTestOID, req)
		require.NoError(t, err)

		parsed := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(body, &parsed), "body is not a JSON object: %s", body)

		wantKeys := map[string]bool{"oid": true, "query": true, "startTime": true, "endTime": true, "paginated": true}
		require.Equal(t, searchTestOID, parsed["oid"])
		require.Equal(t, query, parsed["query"])
		require.Equal(t, strconv.FormatInt(start, 10), parsed["startTime"])
		require.Equal(t, strconv.FormatInt(end, 10), parsed["endTime"])
		require.Equal(t, !setPaginated || paginated, parsed["paginated"])
		if stream != "" {
			wantKeys["stream"] = true
			require.Equal(t, stream, parsed["stream"])
		}
		if wantMode != "" {
			wantKeys["mode"] = true
			require.Equal(t, wantMode, parsed["mode"])
		}
		require.Len(t, parsed, len(wantKeys), "unexpected key set in %s", body)
		for k := range parsed {
			require.True(t, wantKeys[k], "unexpected key %q in %s", k, body)
		}
	})
}

// FuzzSearchQueryPath checks that any query id a caller can hand in either is
// refused or addresses exactly one search: the escaped path has the fixed
// prefix and one more segment, that segment is not a dot-segment, it decodes
// back to the id, and parsing the URL yields no query string or fragment.
func FuzzSearchQueryPath(f *testing.F) {
	for _, seed := range []string{
		"query-1", "", ".", "..", "...", "a/b", "a?b=c", "a#frag", "%2E%2E", "../validate",
		"q/../validate?token=injected", " ", "\x00", "\xff\xfe", "a;b", "a\\b", "//", "?",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, id string) {
		path, err := searchQueryPath(id)
		if id == "" || id == "." || id == ".." {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)

		u, err := url.Parse("https://search.example" + path)
		require.NoError(t, err)
		require.Empty(t, u.RawQuery, "id %q leaked a query string", id)
		require.Empty(t, u.Fragment, "id %q leaked a fragment", id)

		segments := strings.Split(u.EscapedPath(), "/")
		require.Len(t, segments, 4, "id %q did not stay one segment: %q", id, u.EscapedPath())
		require.Equal(t, []string{"", "v1", "search"}, segments[:3])
		require.NotContains(t, []string{".", ".."}, segments[3])

		decoded, err := url.PathUnescape(segments[3])
		require.NoError(t, err)
		require.Equal(t, id, decoded)
	})
}
