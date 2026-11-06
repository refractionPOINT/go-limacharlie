package limacharlie

import (
	"encoding/json"
	"strconv"
	"strings"
)

type Dict map[string]interface{}
type List []interface{}

func (d *Dict) UnmarshalJSON(data []byte) error {
	c, err := UnmarshalCleanJSON(string(data))
	if err != nil {
		return err
	}
	*d = c

	return nil
}

func (l *List) UnmarshalJSON(data []byte) error {
	c, err := UnmarshalCleanJSONList(string(data))
	if err != nil {
		return err
	}
	*l = c

	return nil
}

func UnmarshalCleanJSON(data string) (map[string]interface{}, error) {
	d := json.NewDecoder(strings.NewReader(data))
	d.UseNumber()

	out := map[string]interface{}{}
	if err := d.Decode(&out); err != nil {
		return nil, err
	}

	if err := unmarshalCleanJSONMap(out); err != nil {
		return nil, err
	}

	return out, nil
}

func UnmarshalCleanJSONList(data string) ([]interface{}, error) {
	d := json.NewDecoder(strings.NewReader(data))
	d.UseNumber()

	out := []interface{}{}
	if err := d.Decode(&out); err != nil {
		return nil, err
	}

	if err := unmarshalCleanJSONList(out); err != nil {
		return nil, err
	}

	return out, nil
}

func unmarshalCleanJSONMap(out map[string]interface{}) error {
	for k, v := range out {
		switch val := v.(type) {
		case map[string]interface{}:
			if err := unmarshalCleanJSONMap(val); err != nil {
				return err
			}
		case []interface{}:
			if err := unmarshalCleanJSONList(val); err != nil {
				return err
			}
		default:
			var err error
			out[k], err = unmarshalCleanJSONElement(val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func unmarshalCleanJSONList(out []interface{}) error {
	for i, v := range out {
		switch val := v.(type) {
		case map[string]interface{}:
			if err := unmarshalCleanJSONMap(val); err != nil {
				return err
			}
		case []interface{}:
			if err := unmarshalCleanJSONList(val); err != nil {
				return err
			}
		default:
			var err error
			out[i], err = unmarshalCleanJSONElement(val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// unmarshalCleanJSONElement types a json.Number as the narrowest Go number that
// holds it exactly: uint64 for a non-negative integer, int64 for a negative
// one, and float64 for anything else. "Anything else" is a fraction, an
// exponent (which is how encoders commonly write very small and very large
// values, such as 1e-07), and an integer outside both 64-bit ranges, which a
// float64 holds approximately rather than not at all. Only a value outside
// float64's own range is an error, as it is for encoding/json.
func unmarshalCleanJSONElement(out interface{}) (interface{}, error) {
	val, ok := out.(json.Number)
	if !ok {
		return out, nil
	}
	original := val.String()
	if !strings.ContainsAny(original, ".eE") {
		if !strings.HasPrefix(original, "-") {
			// val.Int64() cannot be used here because it does not support
			// unsigned 64 bit ints.
			if i, err := strconv.ParseUint(original, 10, 64); err == nil {
				return i, nil
			}
		} else if i, err := val.Int64(); err == nil {
			return i, nil
		}
	}
	f, err := val.Float64()
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (d Dict) UnMarshalToStruct(out interface{}) error {
	tmp, err := json.Marshal(d)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(tmp, out); err != nil {
		return err
	}
	return nil
}

func (d *Dict) ImportFromStruct(in interface{}) (Dict, error) {
	tmp, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(tmp, d); err != nil {
		return nil, err
	}
	return *d, nil
}
