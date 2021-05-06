package limacharlie

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestSyncPushDRRules(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	rules, err := org.DRRules()
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("unexpected preexisting rules in add/delete: %+v", rules)
	}

	yc := `
rules:
  r1:
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope1
    respond:
      - action: report
        name: t1
  r2:
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope2
    respond:
      - action: report
        name: t2
  r3:
    namespace: managed
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope3
    respond:
      - action: report
        name: t3
`
	c := OrgConfig{}
	err = yaml.Unmarshal([]byte(yc), &c)
	a.NoError(err)

	if len(c.DRRules) != 3 {
		t.Errorf("unexpected conf: %+v", c)
	}

	ops, err := org.SyncPush(c, SyncOptions{
		IsDryRun:    true,
		SyncDRRules: true,
	})
	a.NoError(err)

	if len(ops) != 3 {
		t.Errorf("unexpected ops: %+v", err)
	}
	for _, o := range ops {
		if !o.IsAdded {
			t.Errorf("non-add op: %+v", o)
		}
	}

	rules, err = org.DRRules(WithNamespace("general"))
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("general rules is not empty")
	}
	rules, err = org.DRRules(WithNamespace("managed"))
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("managed rules is not empty")
	}

	ops, err = org.SyncPush(c, SyncOptions{
		SyncDRRules: true,
	})
	a.NoError(err)

	if len(ops) != 3 {
		t.Errorf("unexpected ops: %+v", err)
	}
	for _, o := range ops {
		if !o.IsAdded {
			t.Errorf("non-add op: %+v", o)
		}
	}

	rules, err = org.DRRules(WithNamespace("general"))
	a.NoError(err)
	if len(rules) != 2 {
		t.Errorf("general rules has: %+v", rules)
	}
	rules, err = org.DRRules(WithNamespace("managed"))
	a.NoError(err)
	if len(rules) != 1 {
		t.Errorf("managed rules has: %+v", rules)
	}

	nc := `
rules:
  r1:
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope1
    respond:
      - action: report
        name: t1
  r2:
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope2
    respond:
      - action: report
        name: t2
  r3:
    namespace: general
    detect:
      op: is
      event: NEW_PROCESS
      path: event/FILE_PATH
      value: nope3
    respond:
      - action: report
        name: t3
`

	c = OrgConfig{}
	err = yaml.Unmarshal([]byte(nc), &c)
	a.NoError(err)

	ops, err = org.SyncPush(c, SyncOptions{
		SyncDRRules: true,
	})
	a.NoError(err)

	if len(ops) != 3 {
		t.Errorf("unexpected ops: %+v", err)
	}
	nNew := 0
	nOld := 0
	for _, o := range ops {
		if o.IsAdded {
			nNew++
		}
		if !o.IsAdded && !o.IsRemoved {
			nOld++
		}
	}
	if nNew != 1 || nOld != 2 {
		t.Errorf("unexpected ops: %v", ops)
	}

	rules, err = org.DRRules(WithNamespace("general"))
	a.NoError(err)
	if len(rules) != 3 {
		t.Errorf("general rules has: %+v", rules)
	}
	rules, err = org.DRRules(WithNamespace("managed"))
	a.NoError(err)
	if len(rules) != 0 {
		t.Errorf("managed rules has: %+v", rules)
	}

	ops, err = org.SyncPush(OrgConfig{}, SyncOptions{
		SyncDRRules: true,
		IsForce:     true,
	})
	a.NoError(err)

	if len(ops) != 3 {
		t.Errorf("unexpected ops: %+v", err)
	}
	for _, o := range ops {
		if !o.IsRemoved {
			t.Errorf("non-remove op: %+v", o)
		}
	}
}

func deleteAllFPRules(org *Organization) {
	rules, _ := org.FPRules()
	for ruleName := range rules {
		org.FPRuleDelete(ruleName)
	}
}

func sortSyncOps(ops []OrgSyncOperation) []OrgSyncOperation {
	sort.Slice(ops, func(i int, j int) bool {
		return ops[i].ElementName < ops[j].ElementName
	})
	return ops
}

func TestSyncPushFPRules(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer deleteAllFPRules(org)

	rules, err := org.FPRules()
	a.NoError(err)
	a.Empty(rules)

	// sync rules in dry run
	orgRules := `
fps:
  fp0:
    detect:
      op: ends with
      path: detect/event/FILE_PATH
      value: fp.exe
  fp1:
    detect:
      op: is
      path: routing/hostname
      value: google.com
  fp2:
    detect:
      op: is
      path: DOMAIN_NAME
      value: 8.8.8.8
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(orgRules), &orgConfig))

	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncFPRules: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp0", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp1", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp2", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	fpRules, err := org.FPRules()
	a.NoError(err)
	a.Empty(fpRules)

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncFPRules: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	fpRules, err = org.FPRules()
	a.NoError(err)
	a.Equal(len(orgConfig.FPRules), len(fpRules))
	for fpRuleName, fpRule := range fpRules {
		configFPRule, found := orgConfig.FPRules[fpRuleName]
		a.True(found)
		a.True(configFPRule.DetectionEquals(fpRule))
	}

	// force sync in dry run
	orgRulesForce := `
fps:
  fp0:
    detect:
      op: ends with
      path: detect/event/FILE_PATH
      value: fp.exe
  fp11:
    detect:
      op: is
      path: routing/hostname
      value: google.somethingelse
  fp12:
    detect:
      op: is
      path: DOMAIN_NAME
      value: 8.8.4.4
`
	orgConfigForce := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(orgRulesForce), &orgConfigForce))

	ops, err = org.SyncPush(orgConfigForce, SyncOptions{IsDryRun: true, SyncFPRules: true, IsForce: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp0"},
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp1", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp2", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp11", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.FPRule, ElementName: "fp12", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	fpRulesForce, err := org.FPRules()
	a.NoError(err)
	for fpRuleName, fpRule := range fpRulesForce {
		configFPRule, found := orgConfig.FPRules[fpRuleName]
		a.True(found)
		a.True(configFPRule.DetectionEquals(fpRule))
	}

	// no dry run
	ops, err = org.SyncPush(orgConfigForce, SyncOptions{SyncFPRules: true, IsForce: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	fpRulesForce, err = org.FPRules()
	a.NoError(err)
	a.Equal(len(orgConfigForce.FPRules), len(fpRulesForce))
	for fpRuleName, fpRule := range fpRulesForce {
		configFPRule, found := orgConfigForce.FPRules[fpRuleName]
		a.True(found)
		a.True(configFPRule.DetectionEquals(fpRule))
	}
}

func deleteAllOutputs(org *Organization) {
	outputs, _ := org.Outputs()
	for outputName := range outputs {
		org.OutputDel(outputName)
	}
}

func TestSyncPushOutputs(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer deleteAllOutputs(org)

	outputs, err := org.Outputs()
	a.NoError(err)
	a.Empty(outputs)

	yamlOutputs := `
outputs:
  output0:
    module: s3
    type: detect
    bucket: aws-bucket-name
    key_id: 105c750e-8d6f-4ee5-9815-5975fda15e5b
    secret_key: 403aabff-d7a8-4602-ab9c-815a638a8a30
    is_indexing: "true"
    is_compression: "true"
  output1:
    module: scp
    type: artifact
    dest_host: storage.corp.com
    dir: /uploads/
    username: root
    password: 9a7448cb-df59-423d-b879-d3a83d6ced50
  output2:
    module: slack
    type: detect
    slack_api_token: e8ef2263-baeb-4459-87d3-c7d0cff8aba1
    slack_channe: #detections
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlOutputs), &orgConfig))

	// sync in dry run
	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncOutputs: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output0", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output1", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output2", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	outputs, err = org.Outputs()
	a.NoError(err)
	a.Empty(outputs)

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncOutputs: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	outputs, err = org.Outputs()
	a.NoError(err)
	a.Equal(len(orgConfig.Outputs), len(outputs))
	for outputName, output := range outputs {
		configOutput, found := orgConfig.Outputs[outputName]
		a.True(found)
		configOutput.Name = outputName
		a.True(output.Equals(configOutput), "outputs are not equal %v != %v", output, configOutput)
	}

	// force sync in dry run
	yamlOutputs = `
outputs:
  output0:
    module: s3
    type: detect
    bucket: aws-bucket-name
    key_id: 105c750e-8d6f-4ee5-9815-5975fda15e5b
    secret_key: 403aabff-d7a8-4602-ab9c-815a638a8a30
    is_indexing: "true"
    is_compression: "true"
  output11:
    module: scp
    type: artifact
    dest_host: storage.corp.com
    dir: /uploads/
    username: root
    password: 9a7448cb-df59-423d-b879-d3a83d6ced50
  output12:
    module: slack
    type: detect
    slack_api_token: e8ef2263-baeb-4459-87d3-c7d0cff8aba1
    slack_channe: #detections
`
	orgConfigForce := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlOutputs), &orgConfigForce))

	ops, err = org.SyncPush(orgConfigForce, SyncOptions{IsDryRun: true, SyncOutputs: true, IsForce: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output0"},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output1", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output2", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output11", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output12", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	outputsForce, err := org.Outputs()
	a.NoError(err)
	for outputName, output := range outputsForce {
		configOutput, found := orgConfig.Outputs[outputName]
		a.True(found)
		configOutput.Name = outputName
		a.True(output.Equals(configOutput), "outputs are not equal %v != %v", output, configOutput)
	}

	// no dry run
	ops, err = org.SyncPush(orgConfigForce, SyncOptions{SyncOutputs: true, IsForce: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output0"},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output1", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output2", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output11", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Output, ElementName: "output12", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	outputsForce, err = org.Outputs()
	a.NoError(err)
	for outputName, output := range outputsForce {
		configOutput, found := orgConfigForce.Outputs[outputName]
		a.True(found)
		configOutput.Name = outputName
		a.True(output.Equals(configOutput), "outputs are not equal %v != %v", output, configOutput)
	}

}
