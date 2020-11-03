package limacharlie

import (
	"testing"
)

const (
	testConfig = `
oid: 11111111-2222-3333-4444-555555555555
api_key: 31111111-2222-3333-4444-555555555555
env:
  ttt:
    oid: 41111111-2222-3333-4444-555555555555
    uid: 51111111-2222-3333-4444-555555555555
    api_key: 61111111-2222-3333-4444-555555555555
  vvv:
    oid: 71111111-2222-3333-4444-555555555555
    uid: 81111111-2222-3333-4444-555555555555
    api_key: 91111111-2222-3333-4444-555555555555`
)

func TestLoadingDefaultConfig(t *testing.T) {
	o := ClientOptions{}

	if err := o.FromConfigString([]byte(testConfig), ""); err != nil {
		t.Errorf("failed parsing yaml config: %v", err)
	}
	if o.OID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected oid: %+v", o)
	}
	if o.UID != "" {
		t.Errorf("unexpected uid: %+v", o)
	}
	if o.APIKey != "31111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected apiKey: %+v", o)
	}

	if err := o.FromConfigString([]byte(testConfig), "default"); err != nil {
		t.Errorf("failed parsing yaml config: %v", err)
	}
	if o.OID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected oid: %+v", o)
	}
	if o.UID != "" {
		t.Errorf("unexpected uid: %+v", o)
	}
	if o.APIKey != "31111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected apiKey: %+v", o)
	}
}

func TestLoadingEnvConfig(t *testing.T) {
	o := ClientOptions{}

	if err := o.FromConfigString([]byte(testConfig), "vvv"); err != nil {
		t.Errorf("failed parsing yaml config: %v", err)
	}
	if o.OID != "71111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected oid: %+v", o)
	}
	if o.UID != "81111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected uid: %+v", o)
	}
	if o.APIKey != "91111111-2222-3333-4444-555555555555" {
		t.Errorf("unexpected apiKey: %+v", o)
	}
}
