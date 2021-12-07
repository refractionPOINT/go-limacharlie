package limacharlie

import (
	"fmt"
	"net/http"
)

// Organization holds a connection to the LC cloud organization
type Organization struct {
	client *Client
	logger LCLogger
	invID  string
}

// OrganizationInformation has the information about the organization
type OrganizationInformation struct {
	OID            string            `json:"oid,omitempty"`
	SensorVersion  string            `json:"sensor_version,omitempty"`
	LatestVersions map[string]string `json:"latest_versions,omitempty"`
	NumberOutputs  int64             `json:"n_outputs,omitempty"`
	NumberInstKeys int64             `json:"n_installation_keys,omitempty"`
	NumberRules    int64             `json:"n_rules,omitempty"`
	Name           string            `json:"name,omitempty"`
	SensorQuota    int64             `json:"sensor_quota,omitempty"`
}

type NewOrganizationResponse struct {
	Data    NewOrganizationDataResponse `json:"data,omitempty"`
	Success bool                        `json:"success,omitempty"`
}

type NewOrganizationDataResponse struct {
	Oid string `json:"oid,omitempty"`
}

// NewOrganization initialize a link to an organization
func NewOrganization(c *Client) (*Organization, error) {
	return &Organization{
		client: c,
		logger: c.logger,
	}, nil
}

// NewOrganizationFromClientOptions initialize an organization from client options
func NewOrganizationFromClientOptions(opt ClientOptions, logger LCLogger) (*Organization, error) {
	c, err := NewClient(opt, logger)
	if err != nil {
		return nil, err
	}
	return NewOrganization(c)
}

// Permission represents the permission granted in LC
type Permission struct {
	Name string
}

// NoPermission is an empty permission slice
func NoPermission() []Permission {
	return make([]Permission, 0)
}

// MakePermissions create a permission slice based on permissions name
func MakePermissions(arr []string) []Permission {
	permissions := make([]Permission, len(arr))
	for i, p := range arr {
		permissions[i] = Permission{p}
	}
	return permissions
}

func arrayExistsInString(key string, arr []string) bool {
	for _, v := range arr {
		if key == v {
			return true
		}
	}
	return false
}

// Authorize validate requested permissions for the organization
func (org *Organization) Authorize(permissionsNeeded []string) (string, []Permission, error) {
	effective := NoPermission()
	result, err := org.client.whoAmI()
	if err != nil {
		return "", effective, fmt.Errorf("Error with WhoAmI request: %s", err)
	}

	if result.UserPermissions != nil && len(*result.UserPermissions) > 1 {
		// permissions for multiple orgs
		effectiveNames := (*result.UserPermissions)[org.client.options.OID]
		effective = MakePermissions(effectiveNames)
	} else if result.Organizations != nil {
		// machine token
		orgs := *result.Organizations
		found := arrayExistsInString(org.client.options.OID, orgs)
		if found {
			if result.Permissions != nil {
				effective = MakePermissions(*result.Permissions)
			}
		}
	}

	missing := []string{}
	mapEffective := makeSet(effective)
	for _, p := range permissionsNeeded {
		if _, found := mapEffective[p]; !found {
			missing = append(missing, p)
		}
	}

	if len(missing) > 0 {
		return "", NoPermission(), fmt.Errorf("unauthorized, missing permissions: '%q'", missing)
	}

	ident := ""
	if result.Identity != nil {
		ident = *result.Identity
	}
	return ident, effective, nil
}

func makeSet(arr []Permission) map[string]struct{} {
	m := map[string]struct{}{}
	for _, v := range arr {
		m[v.Name] = struct{}{}
	}
	return m
}

// GetCurrentJWT returns the JWT of the client
func (org *Organization) GetCurrentJWT() string {
	return org.client.GetCurrentJWT()
}

func (org *Organization) WithInvestigationID(invID string) *Organization {
	org.invID = invID
	return org
}

func (o *Organization) Comms() *Comms {
	return &Comms{o: o}
}

func (o *Organization) GetURLs() (map[string]string, error) {
	resp := struct {
		URLs map[string]string `json:"url"`
	}{}

	if err := o.client.reliableRequest(http.MethodGet, fmt.Sprintf("orgs/%s/url", o.client.options.OID), makeDefaultRequest(&resp)); err != nil {
		return nil, err
	}
	return resp.URLs, nil
}

func (o *Organization) GetInfo() (OrganizationInformation, error) {
	resp := OrganizationInformation{}
	if err := o.client.reliableRequest(http.MethodGet, fmt.Sprintf("orgs/%s", o.client.options.OID), makeDefaultRequest(&resp)); err != nil {
		return OrganizationInformation{}, err
	}
	return resp, nil
}

func (o *Organization) CreateOrganization(location, name string) (NewOrganizationResponse, error) {
	resp := NewOrganizationResponse{}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"loc":  location,
		"name": name,
	})
	if err := o.client.reliableRequest(http.MethodPost, "orgs/new", request); err != nil {
		return NewOrganizationResponse{}, err
	}
	return resp, nil
}

func (o *Organization) SetQuota(quota int64) (bool, error) {
	resp := map[string]bool{}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"quota": quota,
	})
	if err := o.client.reliableRequest(http.MethodPost, fmt.Sprintf("orgs/%s/quota", o.client.options.OID), request); err != nil {
		return false, err
	}
	if val, ok := resp["success"]; ok {
		return val, nil
	}
	return false, nil
}

func (o *Organization) ServiceRequest(responseData interface{}, serviceName string, serviceData Dict, isAsync bool) error {
	return o.client.serviceRequest(responseData, serviceName, serviceData, isAsync)
}
