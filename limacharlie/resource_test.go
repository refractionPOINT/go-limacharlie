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

	// Check if ip-geo is already subscribed
	baseAPIResources := resourcesBase.GetForCategory(ResourceCategories.API)
	_, alreadySubscribed := baseAPIResources[resourceName]

	// If not subscribed, subscribe to it
	if !alreadySubscribed {
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
	} else {
		t.Logf("Resource %s already subscribed, skipping subscribe test", resourceName)
	}

	// Unsubscribe from ip-geo
	err = org.ResourceUnsubscribe(resourceName, resourceCategory)
	a.NoError(err)
	time.Sleep(20 * time.Second) // Increased wait time

	resources, err := org.Resources()
	a.NoError(err)
	apiResources := resources.GetForCategory(ResourceCategories.API)
	_, stillSubscribed := apiResources[resourceName]
	if stillSubscribed {
		t.Logf("Resource %s still subscribed after unsubscribe (may take longer to propagate), checking manually", resourceName)
		// Verify by waiting longer and checking again
		time.Sleep(10 * time.Second)
		resources, err = org.Resources()
		a.NoError(err)
		apiResources = resources.GetForCategory(ResourceCategories.API)
		_, stillSubscribed = apiResources[resourceName]
	}
	a.False(stillSubscribed, "Resource should be unsubscribed")
}
