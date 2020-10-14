package limacharlie

import (
	"fmt"
)

type Organization struct {
	Permissions []string
}

func Authorize(clientOpts ClientOptions, permissions []string) (Organization, error) {
	c, err := NewClient(clientOpts)
	if err != nil {
		return Organization{}, fmt.Errorf("Could not initialize client: %s", err)
	}
	result, err := c.whoAmI()
	if err != nil {
		return Organization{}, fmt.Errorf("Error with WhoAmI request: %s", err)
	}

	effective := []string{}
	if result.UserPermissions != nil && len(*result.UserPermissions) > 1 {
		// permissions for multiple orgs
		effective, _ = (*result.UserPermissions)[clientOpts.OID]
	} else if result.Organizations != nil {
		// machine permissions
		if _, found := (*result.Organizations)[clientOpts.OID]; found {
			if result.Permissions != nil {
				effective = *result.Permissions
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
		return Organization{}, fmt.Errorf("Unauthorized, missing permissions: %q", missing)
	}
	return Organization{effective}, nil
}

func mapFromArray(arr []string) map[string]int {
	m := map[string]int{}
	for i, v := range arr {
		m[v] = i
	}
	return m
}
