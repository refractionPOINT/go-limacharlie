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

func unmarshalCleanJSONElement(out interface{}) (interface{}, error) {
	if val, ok := out.(json.Number); ok {
		// This is a Number, we need to check if it has
		// a float component originally to see if we should
		// type it as an int64 or a float64.
		original := val.String()
		if !strings.Contains(original, ".") {
			// No dot component, return as uint64 so
			// that it gets serialized back to a notation
			// without a dot in it.
			if !strings.HasPrefix(original, "-") {
				// We cannot use val.Int64() because it does not
				// support Unsigned 64 bit ints.
				i, err := strconv.ParseUint(original, 10, 64)
				if err != nil {
					return nil, err
				}
				out = i
			} else {
				// Looks like a signed value.
				i, err := val.Int64()
				if err != nil {
					return nil, err
				}
				out = i
			}
		} else {
			// There is a dot, assume a float.
			i, err := val.Float64()
			if err != nil {
				return nil, err
			}
			out = i
		}
	}
	return out, nil
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
