package limacharlie

import (
	"fmt"
	"net/http"
	"time"
)

type ResourceCategory = string
type ResourceName = string
type Resources = map[ResourceCategory][]ResourceName

var ResourceCategories = struct {
	API       string
	Replicant string
}{
	API:       "api",
	Replicant: "replicant",
}

func (org Organization) resources(verb string, request restRequest) error {
	return org.client.reliableRequest(verb, fmt.Sprintf("orgs/%s/resources", org.client.options.OID), request)
}

type resourceGetResponse = map[string]Resources

// Resources list available resources
func (org Organization) Resources() (Resources, error) {
	resp := resourceGetResponse{}
	req := makeDefaultRequest(&resp).withTimeout(10 * time.Second)
	if err := org.resources(http.MethodGet, req); err != nil {
		return Resources{}, err
	}

	if resources, found := resp["resources"]; found {
		return resources, nil
	}
	return Resources{}, fmt.Errorf("resources: expected key 'resources' is missing from response")
}

// ResourceSubscribe subscribe to a resource.
// The backend call is async meaning that you will get a response right away but it might take a
// few seconds before a call to list resources shows up with the updated list.
func (org Organization) ResourceSubscribe(name ResourceName, category ResourceCategory) error {
	resp := Dict{}
	req := makeDefaultRequest(&resp).withTimeout(10 * time.Second).withFormData(Dict{
		"res_cat":  category,
		"res_name": name,
	})
	return org.resources(http.MethodPost, req)
}

// ResourceUnsubscribe unsubscribe from a resource.
// The backend call is async meaning that you will get a response right away but it might take a
// few seconds before a call to list resources shows up with the updated list.
func (org Organization) ResourceUnsubscribe(name ResourceName, category ResourceCategory) error {
	resp := Dict{}
	req := makeDefaultRequest(&resp).withTimeout(10 * time.Second).withFormData(Dict{
		"res_cat":  category,
		"res_name": name,
	})
	return org.resources(http.MethodDelete, req)
}
