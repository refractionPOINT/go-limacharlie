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
	time.Sleep(5 * time.Second)

	resources, err := org.Resources()
	a.NoError(err)
	expectedResources := resourcesBase.duplicate()
	apiResources := expectedResources.GetForCategory(ResourceCategories.API)
	apiResources["ip-geo"] = struct{}{}
	expectedResources[ResourceCategories.API] = apiResources
	a.Equal(expectedResources, resources)

	err = org.ResourceUnsubscribe(resourceName, resourceCategory)
	a.NoError(err)
	delete(apiResources, "ip-geo")
	time.Sleep(15 * time.Second)

	resources, err = org.Resources()
	a.NoError(err)
	a.Equal(resourcesBase, resources)
}
