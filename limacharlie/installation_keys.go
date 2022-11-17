package limacharlie

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type InstallationKey struct {
	CreatedAt   int64    `json:"created,omitempty": yaml:"created,omitempty"`
	Description string   `json:"desc,omitempty" yaml:"desc,omitempty"`
	ID          string   `json:"iid,omitempty" yaml:"iid,omitempty"`
	Key         string   `json:"key,omitempty" yaml:"key,omitempty"`
	JsonKey     string   `json:"json_key,omitempty" yaml:"json_key,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type InstallationKeyName = string

func (ik *InstallationKey) UnmarshalJSON(data []byte) error {
	d := map[string]interface{}{}
	d, err := UnmarshalCleanJSON(string(data))
	if err != nil {
		return err
	}

	s, ok := d["desc"].(string)
	if !ok {
		return errors.New("invalid field desc")
	}
	ik.Description = s

	s, ok = d["iid"].(string)
	if !ok {
		return errors.New("invalid field iid")
	}
	ik.ID = s

	s, ok = d["key"].(string)
	if !ok {
		return errors.New("invalid field key")
	}
	ik.Key = s

	s, ok = d["json_key"].(string)
	if !ok {
		return errors.New("invalid field json_key")
	}
	ik.JsonKey = s

	s, ok = d["tags"].(string)
	if !ok {
		return errors.New("invalid field tags")
	}
	ik.Tags = strings.Split(s, ",")

	s, ok = d["created"].(string)
	if !ok {
		i, ok := d["created"].(int64)
		if !ok {
			return fmt.Errorf("invalid field created: %#v (%T)", d["created"], d["created"])
		}
		ik.CreatedAt = i
	} else {
		t, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			return err
		}
		ik.CreatedAt = t.Unix()
	}
	return nil
}

func (org Organization) InstallationKeys() ([]InstallationKey, error) {
	resp := map[string]map[string]InstallationKey{}

	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("installationkeys/%s", org.client.options.OID), request); err != nil {
		return nil, err
	}
	keys := []InstallationKey{}
	orgKeys, ok := resp[org.client.options.OID]
	if !ok {
		return nil, errors.New("response missing org keys")
	}
	for _, k := range orgKeys {
		keys = append(keys, k)
	}
	return keys, nil
}

func (org Organization) InstallationKey(iid string) (*InstallationKey, error) {
	resp := InstallationKey{}

	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("installationkeys/%s/%s", org.client.options.OID, iid), request); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (org Organization) AddInstallationKey(k InstallationKey) (string, error) {
	resp := Dict{}

	request := makeDefaultRequest(&resp).withFormData(Dict{
		"tags": k.Tags,
		"desc": k.Description,
	})
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("installationkeys/%s", org.client.options.OID), request); err != nil {
		return "", err
	}
	iid, _ := resp["iid"].(string)
	return iid, nil
}

func (org Organization) DelInstallationKey(iid string) error {
	resp := Dict{}

	request := makeDefaultRequest(&resp).withFormData(Dict{
		"iid": iid,
	})
	if err := org.client.reliableRequest(http.MethodDelete, fmt.Sprintf("installationkeys/%s", org.client.options.OID), request); err != nil {
		return err
	}
	return nil
}
