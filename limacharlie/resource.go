package limacharlie

import (
	"fmt"
	"net/http"
	"time"
)

type ResourceCategory = string
type ResourceName = string
type ResourcesByCategory map[ResourceCategory]map[ResourceName]struct{}

var ResourceCategories = struct {
	API       string
	Replicant string
	Service   string
}{
	API:       "api",
	Replicant: "replicant",
	Service:   "service",
}

func (org Organization) resources(verb string, request restRequest) error {
	return org.client.reliableRequest(verb, fmt.Sprintf("orgs/%s/resources", org.client.options.OID), request)
}

type resourceGetResponse = map[string]map[string][]string

func (r ResourcesByCategory) duplicate() ResourcesByCategory {
	dup := ResourcesByCategory{}
	for resCat, resNames := range r {
		names, found := dup[resCat]
		if !found {
			names = map[string]struct{}{}
		}
		for name := range resNames {
			names[name] = struct{}{}
		}
		dup[resCat] = names
	}
	return dup
}

func (r *ResourcesByCategory) AddToCategory(category ResourceCategory, name ResourceName) {
	cat, found := (*r)[category]
	if !found {
		cat = map[string]struct{}{}
	}
	cat[name] = struct{}{}
	(*r)[category] = cat
}

func (r *ResourcesByCategory) GetForCategory(category ResourceCategory) map[ResourceName]struct{} {
	resourcesForCat, found := (*r)[category]
	if !found {
		resourcesForCat = map[ResourceName]struct{}{}
		(*r)[category] = resourcesForCat
	}
	return resourcesForCat
}

func (r *ResourcesByCategory) RemoveFromCategory(category ResourceCategory, name ResourceName) {
	cat, found := (*r)[category]
	if !found {
		return
	}
	delete(cat, name)
	(*r)[category] = cat
}

// Resources list available resources
func (org Organization) Resources() (ResourcesByCategory, error) {
	resp := resourceGetResponse{}
	req := makeDefaultRequest(&resp).withTimeout(10 * time.Second)
	if err := org.resources(http.MethodGet, req); err != nil {
		return ResourcesByCategory{}, err
	}

	resources := ResourcesByCategory{}
	resourcesContent, found := resp["resources"]
	if !found {
		return resources, fmt.Errorf("resources: expected key 'resources' is missing from response")
	}
	for resCat, resNames := range resourcesContent {
		resourcesForCat, ok := resources[resCat]
		if !ok {
			resourcesForCat = map[string]struct{}{}
		}
		for _, resName := range resNames {
			resourcesForCat[resName] = struct{}{}
		}
		resources[resCat] = resourcesForCat
	}
	return resources, nil
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
