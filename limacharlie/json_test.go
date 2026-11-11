package limacharlie

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUnmarshalCleanJSONNumbers pins how Dict types each JSON number: exactly
// as an integer whenever a 64-bit integer type holds it, and as a float64
// otherwise. A number spelled with an exponent or out of the integer ranges
// must decode, since a single such value in an event would otherwise fail the
// whole response it arrived in.
func TestUnmarshalCleanJSONNumbers(t *testing.T) {
	for name, tc := range map[string]struct {
		number string
		want   interface{}
	}{
		"zero":                         {number: `0`, want: uint64(0)},
		"negative zero":                {number: `-0`, want: int64(0)},
		"uint64 maximum is exact":      {number: `18446744073709551615`, want: uint64(math.MaxUint64)},
		"past float64 precision":       {number: `9007199254740993`, want: uint64(9007199254740993)},
		"int64 minimum is exact":       {number: `-9223372036854775808`, want: int64(math.MinInt64)},
		"fraction":                     {number: `1.5`, want: 1.5},
		"lower-case exponent":          {number: `1e-07`, want: 1e-07},
		"upper-case exponent":          {number: `2E+21`, want: 2e21},
		"exponent on an integer":       {number: `5e3`, want: 5000.0},
		"negative exponent form":       {number: `-3.5e-10`, want: -3.5e-10},
		"one past the uint64 maximum":  {number: `18446744073709551616`, want: 18446744073709551616.0},
		"one past the int64 minimum":   {number: `-9223372036854775809`, want: -9223372036854775809.0},
		"thirty digit integer":         {number: `123456789012345678901234567890`, want: 1.2345678901234568e29},
		"largest float64 exponent use": {number: `1.7976931348623157e308`, want: math.MaxFloat64},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := UnmarshalCleanJSON(`{"v":` + tc.number + `,"nested":{"list":[` + tc.number + `]}}`)
			require.NoError(t, err)
			require.Equal(t, tc.want, got["v"])
			require.Equal(t, tc.want, got["nested"].(map[string]interface{})["list"].([]interface{})[0],
				"nested values must be typed the same way")
		})
	}

	t.Run("a value past float64's range is still an error", func(t *testing.T) {
		_, err := UnmarshalCleanJSON(`{"v":1e400}`)
		require.Error(t, err)
	})
}

// FuzzUnmarshalCleanJSON checks Dict decoding against encoding/json as an
// oracle for what is decodable: any JSON object that encoding/json can decode
// into a map must decode through UnmarshalCleanJSON too, to the same keys, and
// a number must come back as a float64 equal to encoding/json's or as an
// integer type that converts to it.
func FuzzUnmarshalCleanJSON(f *testing.F) {
	for _, seed := range []string{
		`{}`,
		`{"a":1}`,
		`{"a":-1,"b":1.5,"c":1e-07,"d":[1,2E+3,{"e":18446744073709551616}]}`,
		`{"a":null,"b":true,"c":"s","d":[]}`,
		`{"a":123456789012345678901234567890}`,
		`{"a":1e400}`,
		`[1]`,
		`not json`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		var oracle map[string]interface{}
		oracleErr := json.Unmarshal([]byte(data), &oracle)

		got, err := UnmarshalCleanJSON(data)
		if oracleErr != nil {
			// UnmarshalCleanJSON reads one value from a stream, so it may
			// accept trailing input encoding/json refuses. It must never
			// panic, which reaching here shows.
			return
		}
		require.NoError(t, err, "encoding/json decodes %q, so Dict decoding must too", data)
		requireCleanMatchesOracle(t, oracle, got)
	})
}

// requireCleanMatchesOracle walks two decodings of one document in step.
func requireCleanMatchesOracle(t *testing.T, oracle, got interface{}) {
	t.Helper()
	switch o := oracle.(type) {
	case map[string]interface{}:
		g, ok := got.(map[string]interface{})
		require.True(t, ok, "want an object, got %T", got)
		require.Len(t, g, len(o))
		for k, v := range o {
			gv, present := g[k]
			require.True(t, present, "key %q missing", k)
			requireCleanMatchesOracle(t, v, gv)
		}
	case []interface{}:
		g, ok := got.([]interface{})
		require.True(t, ok, "want an array, got %T", got)
		require.Len(t, g, len(o))
		for i := range o {
			requireCleanMatchesOracle(t, o[i], g[i])
		}
	case float64:
		switch g := got.(type) {
		case float64:
			require.Equal(t, o, g)
		case uint64:
			require.Equal(t, o, float64(g))
		case int64:
			require.Equal(t, o, float64(g))
		default:
			require.Failf(t, "number decoded as a non-number", "%v became %T", o, got)
		}
	default:
		require.Equal(t, oracle, got)
	}
}
