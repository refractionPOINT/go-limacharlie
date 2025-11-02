package limacharlie

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type InstallationKey struct {
	CreatedAt   uint64   `json:"created,omitempty" yaml:"created,omitempty"`
	Description string   `json:"desc,omitempty" yaml:"desc,omitempty"`
	ID          string   `json:"iid,omitempty" yaml:"iid,omitempty"`
	Key         string   `json:"key,omitempty" yaml:"key,omitempty"`
	JsonKey     string   `json:"json_key,omitempty" yaml:"json_key,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	UsePublicCA bool     `json:"use_public_root_ca,omitempty" yaml:"use_public_root_ca,omitempty"`
}

type InstallationKeyName = string

func (ik *InstallationKey) UnmarshalJSON(data []byte) error {
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
	for _, tag := range strings.Split(s, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		ik.Tags = append(ik.Tags, tag)
	}

	s, ok = d["created"].(string)
	if !ok {
		i, ok := d["created"].(uint64)
		if !ok {
			return fmt.Errorf("invalid field created: %#v (%T)", d["created"], d["created"])
		}
		ik.CreatedAt = i
	} else {
		t, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			return err
		}
		ik.CreatedAt = uint64(t.Unix())
	}

	if b, ok := d["use_public_root_ca"].(bool); ok {
		ik.UsePublicCA = b
	}
	return nil
}

func (k InstallationKey) EqualsContent(k2 InstallationKey) bool {
	if k.Description != k2.Description {
		return false
	}
	if len(k.Tags) != len(k2.Tags) {
		return false
	}
	for _, t1 := range k.Tags {
		found := false
		for _, t2 := range k2.Tags {
			if t1 == t2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if k.UsePublicCA != k2.UsePublicCA {
		return false
	}
	return true
}

func (org *Organization) InstallationKeys() ([]InstallationKey, error) {
	resp := map[string]map[string]InstallationKey{}

	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("installationkeys/%s", org.client.options.OID), request); err != nil {
		return nil, err
	}
	keys := []InstallationKey{}
	orgKeys := resp[org.client.options.OID]
	for _, k := range orgKeys {
		keys = append(keys, k)
	}
	return keys, nil
}

func (org *Organization) InstallationKey(iid string) (*InstallationKey, error) {
	resp := InstallationKey{}

	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("installationkeys/%s/%s", org.client.options.OID, url.PathEscape(iid)), request); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (org *Organization) AddInstallationKey(k InstallationKey) (string, error) {
	resp := Dict{}
	req := Dict{
		"tags":               k.Tags,
		"desc":               k.Description,
		"use_public_root_ca": k.UsePublicCA,
	}
	if k.ID != "" {
		req["iid"] = k.ID
	}
	request := makeDefaultRequest(&resp).withFormData(req)
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("installationkeys/%s", org.client.options.OID), request); err != nil {
		return "", err
	}
	iid, _ := resp["iid"].(string)
	return iid, nil
}

func (org *Organization) DelInstallationKey(iid string) error {
	resp := Dict{}

	request := makeDefaultRequest(&resp).withFormData(Dict{
		"iid": iid,
	})
	if err := org.client.reliableRequest(http.MethodDelete, fmt.Sprintf("installationkeys/%s", org.client.options.OID), request); err != nil {
		return err
	}
	return nil
}
