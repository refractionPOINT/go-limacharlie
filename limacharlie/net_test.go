package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetPolicyAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	policies, err := org.NetPolicies()
	a.NoError(err)
	policiesCountStart := len(policies)

	policy := NetPolicy{
		OID:  org.client.options.OID,
		Name: "testpolicy",
	}.WithFirewallPolicy("src host 80", true, "", nil, nil)
	a.NoError(org.NetPolicyAdd(policy))

	policies, err = org.NetPolicies()
	a.NoError(err)
	a.Equal(policiesCountStart+1, len(policies))
	policyFound, found := policies[policy.Name]
	a.True(found, "policy %s not found", policy.Name)
	delete(policy.Policy, "sources")
	delete(policy.Policy, "tag")
	delete(policy.Policy, "times")
	a.True(policy.EqualsContent(policyFound), "policies content are not equal %v != %v", policy, policyFound)

	a.NoError(org.NetPolicyDelete(policy.Name))

	policies, err = org.NetPolicies()
	a.NoError(err)
	a.Equal(policiesCountStart, len(policies))
}
