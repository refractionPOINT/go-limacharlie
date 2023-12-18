package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrgValue(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	const testConf = "otx"
	const testValue = "val1"
	err := org.OrgValueSet(testConf, testValue)
	a.NoError(err)

	ov, err := org.OrgValueGet(testConf)
	a.NoError(err)
	a.Equal(testConf, ov.Name)
	a.Equal(testValue, ov.Value)

	err = org.OrgValueSet(testConf, "")
	a.NoError(err)

	ov, err = org.OrgValueGet(testConf)
	a.NoError(err)
	a.Equal(testConf, ov.Name)
	a.Equal("", ov.Value)
}
