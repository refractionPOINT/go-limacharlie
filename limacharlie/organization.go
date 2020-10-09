package limacharlie

import (
	"fmt"
)

type Organization struct {
	Permissions []string
}

func Authorize(clientOpts ClientOptions, permissions []string) (*Organization, error) {
	c, err := NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("Could not initialize client: %s", err)
	}
	result, err := c.whoAmI()
	if err != nil {
		return nil, fmt.Errorf("Error with WhoAmI request: %s", err)
	}

	effective := []string{}
	if len(*result.UserPermissions) > 1 {
		// permissions for multiple orgs
		effective, _ = (*result.UserPermissions)[clientOpts.OID]
	} else {
		// machine permissions
		if _, found := (*result.Organizations)[clientOpts.OID]; found {
			effective = *result.Permissions
		}
	}

	missing := []string{}
	mapPermissions := mapFromArray(permissions)
	for k := range mapPermissions {
		if _, found := mapPermissions[k]; !found {
			missing = append(missing, k)
		}
	}

	if len(missing) > 1 {
		return nil, fmt.Errorf("Unauthorized, missing permissions: %q", missing)
	}
	return &Organization{effective}, nil
}

func mapFromArray(arr []string) map[string]int {
	m := map[string]int{}
	for i, v := range arr {
		m[v] = i
	}
	return m
}
