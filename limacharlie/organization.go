package limacharlie

import (
	"fmt"
)

type Organization struct {
	client Client
}

func MakeOrganization(clientOpts ClientOptions) (Organization, error) {
	c, err := NewClient(clientOpts)
	if err != nil {
		return Organization{}, fmt.Errorf("Could not initialize client: %s", err)
	}
	return Organization{*c}, nil
}

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
	searchMap := map[string]interface{}{}
	for _, v := range arr {
		searchMap[v] = v
	}
	_, found := searchMap[key]
	return found
}

// Authorize validate requested permissions for the organization
func (org Organization) Authorize(permissionsNeeded []string) ([]Permission, error) {
	effective := NoPermission()
	result, err := org.client.whoAmI()
	if err != nil {
		return effective, fmt.Errorf("Error with WhoAmI request: %s", err)
	}

	if result.UserPermissions != nil && len(*result.UserPermissions) > 1 {
		// permissions for multiple orgs
		effectiveNames, _ := (*result.UserPermissions)[org.client.options.OID]
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
		return NoPermission(), fmt.Errorf("Unauthorized, missing permissions: '%q'", missing)
	}
	return effective, nil
}

func makeSet(arr []Permission) map[string]struct{} {
	m := map[string]struct{}{}
	for _, v := range arr {
		m[v.Name] = struct{}{}
	}
	return m
}
