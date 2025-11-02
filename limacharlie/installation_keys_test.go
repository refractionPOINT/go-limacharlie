package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallationKeys(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	_, err := org.InstallationKeys()
	a.NoError(err)

	iid, err := org.AddInstallationKey(InstallationKey{
		Description: "testcicd",
		Tags:        []string{"t1", "t2"},
	})
	a.NoError(err)
	a.NotEmpty(iid, "installation key ID should not be empty")

	keys, err := org.InstallationKeys()
	a.NoError(err)

	isFound := false
	for _, k := range keys {
		if k.ID == iid {
			a.Equal(2, len(k.Tags), "Tags should be set properly")
			isFound = true
			break
		}
	}
	a.True(isFound, "key should be found in the list")

	k, err := org.InstallationKey(iid)
	a.NoError(err)
	a.NotZero(k.CreatedAt, "InstallationKey should have CreatedAt data")

	err = org.DelInstallationKey(iid)
	a.NoError(err)

	k, err = org.InstallationKey(iid)
	a.Error(err, "InstallationKey should be deleted and return error")
}
