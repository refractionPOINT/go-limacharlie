package limacharlie

import (
	"fmt"
)

func (d *Dict) UnmarshalYAML(unmarshal func(interface{}) error) error {
	out := map[string]interface{}{}
	if err := unmarshal(&out); err != nil {
		return err
	}
	if err := unmarshalCleanYAMLMap(out); err != nil {
		return err
	}
	*d = out

	return nil
}

func (l *List) UnmarshalYAML(unmarshal func(interface{}) error) error {
	out := []interface{}{}
	if err := unmarshal(&out); err != nil {
		return err
	}
	if err := unmarshalCleanYAMLList(out); err != nil {
		return err
	}
	*l = out

	return nil
}

func unmarshalCleanYAMLMap(out map[string]interface{}) error {
	for k, v := range out {
		switch val := v.(type) {
		case map[interface{}]interface{}:
			n, err := yamlMapToJsonMap(val)
			if err != nil {
				return err
			}
			out[k] = n
			if err := unmarshalCleanYAMLMap(n); err != nil {
				return err
			}
		case []interface{}:
			if err := unmarshalCleanYAMLList(val); err != nil {
				return err
			}
		default:
			var err error
			out[k], err = unmarshalCleanYAMLElement(val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func yamlMapToJsonMap(m map[interface{}]interface{}) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	for k, v := range m {
		s, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("unsupported key type: %T", k)
		}
		out[s] = v
	}
	return out, nil
}

func unmarshalCleanYAMLList(out []interface{}) error {
	for i, v := range out {
		switch val := v.(type) {
		case map[interface{}]interface{}:
			n, err := yamlMapToJsonMap(val)
			if err != nil {
				return err
			}
			out[i] = n
			if err := unmarshalCleanYAMLMap(n); err != nil {
				return err
			}
		case []interface{}:
			if err := unmarshalCleanYAMLList(val); err != nil {
				return err
			}
		default:
			var err error
			out[i], err = unmarshalCleanYAMLElement(val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func unmarshalCleanYAMLElement(out interface{}) (interface{}, error) {
	// YAML handles integers vs floats properly already.
	return out, nil
}
