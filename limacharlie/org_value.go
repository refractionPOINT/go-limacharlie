package limacharlie

import (
	"fmt"
	"net/http"
)

type OrgValue struct {
	Name  string `json:"config"`
	Value string `json:"value"`
}

// Get an Org Value from a specific org.
func (org Organization) OrgValueGet(name string) (*OrgValue, error) {
	resp := OrgValue{}
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
