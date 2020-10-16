package limacharlie

import (
	"fmt"
)

type Organization struct {
	client     Client
	clientOpts ClientOptions
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
	for _, p := range arr {
		permissions = append(permissions, Permission{p})
	}
	return permissions
}

func MakeOrganization(clientOpts ClientOptions) (Organization, error) {
	c, err := NewClient(clientOpts)
	if err != nil {
		return Organization{}, fmt.Errorf("Could not initialize client: %s", err)
	}
	return Organization{*c, clientOpts}, nil
}

// Authorize validate requested permissions for the organization
func (org Organization) Authorize(permissions []string) ([]Permission, error) {
	effective := NoPermission()
	result, err := org.client.whoAmI()
	if err != nil {
		return effective, fmt.Errorf("Error with WhoAmI request: %s", err)
	}

	if result.UserPermissions != nil && len(*result.UserPermissions) > 1 {
		// permissions for multiple orgs
		effectiveNames, _ := (*result.UserPermissions)[org.clientOpts.OID]
		effective = MakePermissions(effectiveNames)
	} else if result.Organizations != nil {
		// machine permissions
		if _, found := (*result.Organizations)[org.clientOpts.OID]; found {
			if result.Permissions != nil {
				effective = MakePermissions(*result.Permissions)
			}
		}
	}

	missing := []string{}
	mapPermissions := mapFromArray(permissions)
	for _, p := range permissions {
		if _, found := mapPermissions[p]; !found {
			missing = append(missing, p)
		}
	}

	if len(missing) > 1 {
		return NoPermission(), fmt.Errorf("Unauthorized, missing permissions: %q", missing)
	}
	return effective, nil
}

func mapFromArray(arr []string) map[string]int {
	m := map[string]int{}
	for i, v := range arr {
		m[v] = i
	}
	return m
}
