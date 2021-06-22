package limacharlie

import (
	"fmt"
	"net/http"
)

type OrgValueInfo struct {
	Name  OrgValueName `json:"config"`
	Value OrgValue     `json:"value"`
}

type OrgValueName = string
type OrgValue = string

// Get an Org Value from a specific org.
func (org Organization) OrgValueGet(name string) (*OrgValueInfo, error) {
	resp := OrgValueInfo{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("configs/%s/%s", org.client.options.OID, name), request); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Set an Org Value for a specific org.
func (org Organization) OrgValueSet(name string, value string) error {
	resp := Dict{}
	request := makeDefaultRequest(&resp).withFormData(Dict{
		"value": value,
	})
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("configs/%s/%s", org.client.options.OID, name), request); err != nil {
		return err
	}
	return nil
}
