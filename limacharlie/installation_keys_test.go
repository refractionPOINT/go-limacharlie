package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallationKeys(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	_, err := org.InstallationKeys()
	if err != nil {
		t.Errorf("InstallationKeys(): %v", err)
	}

	iid, err := org.AddInstallationKey(InstallationKey{
		Description: "testcicd",
		Tags:        []string{"t1", "t2"},
	})
	if err != nil {
		t.Errorf("AddInstallationKey(): %v", err)
	}
	if iid == "" {
		t.Error("invalid iid")
	}
	keys, err := org.InstallationKeys()
	if err != nil {
		t.Errorf("InstallationKeys(): %v", err)
	}
	isFound := false
	for _, k := range keys {
		if k.ID == iid {
			if len(k.Tags) != 2 {
				t.Errorf("Tags not set properly: %#v", k.Tags)
			}
			isFound = true
			break
		}
	}
	if !isFound {
		t.Errorf("key not found: %#v", keys)
	}
	k, err := org.InstallationKey(iid)
	if err != nil {
		t.Errorf("InstallationKey(): %v", err)
	} else {
		if k.CreatedAt == 0 {
			t.Errorf("InstallationKey missing data(): %#v", k)
		}
		if err := org.DelInstallationKey(iid); err != nil {
			t.Errorf("DelInstallationKey(): %v", err)
		}
		k, err = org.InstallationKey(iid)
		if err == nil {
			t.Errorf("InstallationKey() should be deleted: %#v", k)
		}
	}
}
