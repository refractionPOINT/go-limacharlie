package limacharlie

import (
	"encoding/json"
	"strings"
)

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
			i, err := val.Int64()
			if err != nil {
				return nil, err
			}
			out = i
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
