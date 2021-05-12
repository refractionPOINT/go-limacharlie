package limacharlie

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func resetResource(org *Organization, resources ResourcesByCategory) {
	orgResources, _ := org.Resources()
	for orgResCat, orgResNames := range orgResources {
		resCat, found := resources[orgResCat]
		if !found {
			for orgResName := range orgResNames {
				org.ResourceUnsubscribe(orgResName, orgResCat)
			}
			continue
		}
		for orgResName := range orgResNames {
			_, found := resCat[orgResName]
			if !found {
				org.ResourceUnsubscribe(orgResName, orgResCat)
			}
		}
	}
}

func TestSyncPushResources(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	resourcesBase, err := org.Resources()
	a.NoError(err)
	defer resetResource(org, resourcesBase.duplicate())

	resourcesConfig := `
resources:
  api:
    - ip-geo
    - vt
  replicant:
    - exfil
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(resourcesConfig), &orgConfig))

	// sync resources in dry run
	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncResources: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "api/ip-geo", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "api/vt", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "replicant/exfil", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	resources, err := org.Resources()
	a.NoError(err)
	a.Equal(resourcesBase, resources)

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncResources: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	time.Sleep(5 * time.Second)
	resources, err = org.Resources()
	a.NoError(err)
	expectedResources := resourcesBase.duplicate()
	expectedResources.AddToCategory(ResourceCategories.API, "ip-geo")
	expectedResources.AddToCategory(ResourceCategories.API, "vt")
	expectedResources.AddToCategory(ResourceCategories.Replicant, "exfil")
	a.Equal(expectedResources, resources)

	// force dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{IsForce: true, IsDryRun: true, SyncResources: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "api/ip-geo"},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "api/ip-geo", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "api/vt"},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "api/vt", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "replicant/exfil"},
		{ElementType: OrgSyncOperationElementType.Resource, ElementName: "replicant/exfil", IsRemoved: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	a.Equal(expectedResources, resources)

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{IsForce: true, SyncResources: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	time.Sleep(5 * time.Second)
	resources, err = org.Resources()
	a.NoError(err)
	a.Equal(resourcesBase, resources)

}

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

func deleteIntegrityRules(org *Organization) {
	rules, _ := org.IntegrityRules()
	for ruleName := range rules {
		org.IntegrityRuleDelete(ruleName)
	}
}

func TestSyncPushIntegrity(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer deleteIntegrityRules(org)

	unsubReplicantCB, err := findUnsubscribeReplicantCallback(org, "integrity")
	a.NoError(err)
	if unsubReplicantCB != nil {
		defer unsubReplicantCB()
	}

	rules, err := org.IntegrityRules()
	a.NoError(err)
	a.Empty(rules)

	yamlIntegrityRules := `
integrity:
  testrule0:
    patterns:
    - /root/.ssh/authorized_keys
    platforms:
    - linux
  testrule1:
    patterns:
    - /home/user/.ssh/*
    platforms:
    - linux
  testrule2:
    patterns:
    - c:\\test.txt
    platforms:
    - windows
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlIntegrityRules), &orgConfig))

	// dry run
	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncIntegrity: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule0", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule1", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule2", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.IntegrityRules()
	a.NoError(err)
	a.Empty(rules)

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncIntegrity: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.IntegrityRules()
	a.NoError(err)
	a.Equal(len(orgConfig.Integrity), len(rules))
	for ruleName, rule := range rules {
		configRule, found := orgConfig.Integrity[ruleName]
		a.True(found)
		a.True(configRule.EqualsContent(rule), "integrity rule content not equal %v != %v", configRule, rule)
	}

	// force and dry run
	yamlIntegrityRules = `
integrity:
  testrule1:
    patterns:
    - /home/user/.ssh/*
    platforms:
    - linux
  testrule3:
    patterns:
    - /home/user/.gitconfig
    platforms:
    - linux
    - windows
`
	forceOrgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlIntegrityRules), &forceOrgConfig))

	ops, err = org.SyncPush(forceOrgConfig, SyncOptions{IsForce: true, IsDryRun: true, SyncIntegrity: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule1"},
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule3", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule0", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Integrity, ElementName: "testrule2", IsRemoved: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.IntegrityRules()
	a.NoError(err)
	a.Equal(len(orgConfig.Integrity), len(rules))
	for ruleName, rule := range rules {
		configRule, found := orgConfig.Integrity[ruleName]
		a.True(found, "rule '%s' not found", ruleName)
		a.True(configRule.EqualsContent(rule), "integrity rule content not equal %v != %v", configRule, rule)
	}

	// force and no dry run

	ops, err = org.SyncPush(forceOrgConfig, SyncOptions{IsForce: true, SyncIntegrity: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.IntegrityRules()
	a.NoError(err)
	a.Equal(len(forceOrgConfig.Integrity), len(rules))
	for ruleName, rule := range rules {
		configRule, found := forceOrgConfig.Integrity[ruleName]
		a.True(found, "rule '%s' not found", ruleName)
		a.True(configRule.EqualsContent(rule), "integrity rule content not equal %v != %v", configRule, rule)
	}
}

func deleteExfil(org *Organization) {
	rules, _ := org.ExfilRules()
	for ruleName := range rules.Watches {
		org.ExfilRuleWatchDelete(ruleName)
	}
	for ruleName := range rules.Events {
		org.ExfilRuleEventDelete(ruleName)
	}
}

func TestSyncPushExfil(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer deleteExfil(org)

	unsubReplicantCB, err := findUnsubscribeReplicantCallback(org, "exfil")
	a.NoError(err)
	if unsubReplicantCB != nil {
		defer unsubReplicantCB()
	}

	rules, err := org.ExfilRules()
	a.NoError(err)
	rulesWatchesLenStart := len(rules.Watches)
	rulesEventsLenStart := len(rules.Events)

	yamlExfil := `
exfil:
  watch:
    watch_evil:
      event: NEW_PROCESS
      path:
        - COMMAND_LINE
      operator: contains
      value: evil
    watch_ps1:
      event: NEW_DOCUMENT
      path:
        - FILE_PATH
      operator: ends with
      value: .ps1
  list:
    event_base:
      events:
        - NEW_PROCESS
        - EXEC_OOB
      filters:
        platforms:
          - windows
          - linux
    event_chrome:
      events:
        - DNS_REQUEST
      filters:
        platforms:
          - chrome
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlExfil), &orgConfig))

	// dry run
	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncExfil: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.ExfilWatch, ElementName: "watch_evil", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.ExfilWatch, ElementName: "watch_ps1", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.ExfilEvent, ElementName: "event_base", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.ExfilEvent, ElementName: "event_chrome", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ExfilRules()
	a.NoError(err)
	a.Equal(rulesWatchesLenStart, len(rules.Watches))
	a.Equal(rulesEventsLenStart, len(rules.Events))

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncExfil: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.ExfilWatch, ElementName: "watch_evil", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.ExfilWatch, ElementName: "watch_ps1", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.ExfilEvent, ElementName: "event_base", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.ExfilEvent, ElementName: "event_chrome", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ExfilRules()
	a.NoError(err)

	a.Equal(rulesWatchesLenStart+2, len(rules.Watches))
	for ruleName, watch := range orgConfig.Exfil.Watches {
		configWatch, found := rules.Watches[ruleName]
		a.True(found, "watch '%s' not found", ruleName)
		a.True(watch.EqualsContent(configWatch), "watch content not equals: %v != %v", watch, configWatch)
	}
	rulesWatchesLenStart += 2

	a.Equal(rulesEventsLenStart+2, len(rules.Events))
	for ruleName, event := range orgConfig.Exfil.Events {
		configEvent, found := rules.Events[ruleName]
		a.True(found, "event '%s' not found", ruleName)
		a.True(event.EqualsContent(configEvent), "event content not equals: %v != %v", event, configEvent)
	}
	rulesEventsLenStart += 2

	// force sync and dry run
	yamlExfil = `
exfil:
  watch:
    watch_evil:
      event: NEW_PROCESS
      path:
        - COMMAND_LINE
      operator: contains
      value: evil
  list:
    event_base:
      events:
        - NEW_PROCESS
        - EXEC_OOB
      filters:
        platforms:
          - windows
          - linux
`
	forceOrgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlExfil), &forceOrgConfig))

	ops, err = org.SyncPush(forceOrgConfig, SyncOptions{IsForce: true, IsDryRun: true, SyncExfil: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.ExfilWatch, ElementName: "watch_evil"},
		{ElementType: OrgSyncOperationElementType.ExfilEvent, ElementName: "event_base"},
		{ElementType: OrgSyncOperationElementType.ExfilWatch, ElementName: "watch_ps1", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.ExfilEvent, ElementName: "event_chrome", IsRemoved: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ExfilRules()
	a.NoError(err)

	a.Equal(rulesWatchesLenStart, len(rules.Watches))
	for ruleName, watch := range orgConfig.Exfil.Watches {
		configWatch, found := rules.Watches[ruleName]
		a.True(found, "watch '%s' not found", ruleName)
		a.True(watch.EqualsContent(configWatch), "watch content not equals: %v != %v", watch, configWatch)
	}

	a.Equal(rulesEventsLenStart, len(rules.Events))
	for ruleName, event := range orgConfig.Exfil.Events {
		configEvent, found := rules.Events[ruleName]
		a.True(found, "event '%s' not found", ruleName)
		a.True(event.EqualsContent(configEvent), "event content not equals: %v != %v", event, configEvent)
	}

	// no dry run
	ops, err = org.SyncPush(forceOrgConfig, SyncOptions{IsForce: true, SyncExfil: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ExfilRules()
	a.NoError(err)

	a.Equal(rulesWatchesLenStart-1, len(rules.Watches))
	for ruleName, watch := range forceOrgConfig.Exfil.Watches {
		configWatch, found := rules.Watches[ruleName]
		a.True(found, "watch '%s' not found", ruleName)
		a.True(watch.EqualsContent(configWatch), "watch content not equals: %v != %v", watch, configWatch)
	}

	a.Equal(rulesEventsLenStart-1, len(rules.Events))
	for ruleName, event := range forceOrgConfig.Exfil.Events {
		configEvent, found := rules.Events[ruleName]
		a.True(found, "event '%s' not found", ruleName)
		a.True(event.EqualsContent(configEvent), "event content not equals: %v != %v", event, configEvent)
	}
}

func deleteArtifacts(org *Organization) {
	rules, _ := org.ArtifactsRules()
	for ruleName := range rules {
		org.ArtifactRuleDelete(ruleName)
	}
}

func TestSyncPushArtifact(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer deleteArtifacts(org)

	unsubCB, err := findUnsubscribeReplicantCallback(org, "logging")
	a.NoError(err)
	if unsubCB != nil {
		defer unsubCB()
	}

	rules, err := org.ArtifactsRules()
	a.NoError(err)
	rulesCountStart := len(rules)

	yamlArtifact := `
artifact:
  linux-logs:
    is_delete_after: false
    is_ignore_cert: false
    patterns:
    - /var/log/syslog.1
    - /var/log/auth.log.1
    platforms:
    - linux
    days_retention: 30
    tags: []
  windows-logs:
    is_delete_after: false
    is_ignore_cert: false
    patterns:
    - c:\\windows\\system32\\winevt\\logs\\Security.evtx
    - c:\\windows\\system32\\winevt\\logs\\System.evtx
    platforms:
    - windows
    days_retention: 30
    tags: []
  browser-chrome-logs:
    is_delete_after: false
    is_ignore_cert: false
    patterns:
    - "%homepath%\\AppData\\Local\\Google\\Chrome\\User Data\\Crashpad\\reports"
    - "~/Library/Application Support/Google/Chrome/Crashpad/completed/"
    platforms:
    - windows
    - macos
    tags: []
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlArtifact), &orgConfig))

	// dry run - no force
	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncArtifacts: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Artifact, ElementName: "linux-logs", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Artifact, ElementName: "windows-logs", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Artifact, ElementName: "browser-chrome-logs", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ArtifactsRules()
	a.NoError(err)
	a.Equal(rulesCountStart, len(rules))

	// no force
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncArtifacts: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))

	rules, err = org.ArtifactsRules()
	a.NoError(err)
	a.Equal(rulesCountStart+3, len(rules))
	for ruleName, rule := range orgConfig.Artifacts {
		orgRule, found := rules[ruleName]
		a.True(found, "artifact rule not found for %s", ruleName)
		a.True(rule.EqualsContent(orgRule), "artifact rule content not equal: %v != %v", rule, OrgSyncArtifactRule{}.FromArtifactRule(orgRule))
	}

	// dry run - force
	yamlArtifact = `
artifact:
  windows-logs:
    is_delete_after: false
    is_ignore_cert: false
    patterns:
    - c:\\windows\\system32\\winevt\\logs\\Security.evtx
    - c:\\windows\\system32\\winevt\\logs\\System.evtx
    platforms:
    - windows
    days_retention: 30
    tags: []
`
	forceOrgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlArtifact), &forceOrgConfig))

	ops, err = org.SyncPush(forceOrgConfig, SyncOptions{IsForce: true, IsDryRun: true, SyncArtifacts: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Artifact, ElementName: "linux-logs", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Artifact, ElementName: "windows-logs"},
		{ElementType: OrgSyncOperationElementType.Artifact, ElementName: "browser-chrome-logs", IsRemoved: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ArtifactsRules()
	a.NoError(err)
	a.Equal(rulesCountStart+3, len(rules))
	for ruleName, rule := range orgConfig.Artifacts {
		orgRule, found := rules[ruleName]
		a.True(found, "artifact rule not found for %s", ruleName)
		a.True(rule.EqualsContent(orgRule), "artifact rule content not equal: %v != %v", rule, OrgSyncArtifactRule{}.FromArtifactRule(orgRule))
	}

	// force
	ops, err = org.SyncPush(forceOrgConfig, SyncOptions{IsForce: true, SyncArtifacts: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	rules, err = org.ArtifactsRules()
	a.NoError(err)
	a.Equal(rulesCountStart+1, len(rules))
	for ruleName, rule := range forceOrgConfig.Artifacts {
		orgRule, found := rules[ruleName]
		a.True(found, "artifact rule not found for %s", ruleName)
		a.True(rule.EqualsContent(orgRule), "artifact rule content not equal: %v != %v", rule, OrgSyncArtifactRule{}.FromArtifactRule(orgRule))
	}
}
