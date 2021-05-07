package limacharlie

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResourceAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	resourcesBase, err := org.Resources()
	a.NoError(err)

	resourceName := "ip-geo"
	resourceCategory := ResourceCategories.API
	err = org.ResourceSubscribe(resourceName, resourceCategory)
	a.NoError(err)
	time.Sleep(2 * time.Second)

	resources, err := org.Resources()
	a.NoError(err)
	expectedResources := resourcesBase
	apiResources := expectedResources[ResourceCategories.API]
	apiResources = append(apiResources, "ip-geo")
	expectedResources[ResourceCategories.API] = apiResources
	a.Equal(expectedResources, resources)

	err = org.ResourceUnsubscribe(resourceName, resourceCategory)
	a.NoError(err)
	time.Sleep(2 * time.Second)

	resources, err = org.Resources()
	a.NoError(err)
	a.Equal(resourcesBase, resources)
}
