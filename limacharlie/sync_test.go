package limacharlie

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func resetResource(org *Organization) {
	orgResources, _ := org.Resources()
	for orgResCat, orgResNames := range orgResources {
		for orgResName := range orgResNames {
			if orgResCat != "insight" && orgResName != "api" {
				org.ResourceUnsubscribe(orgResName, orgResCat)
			}
		}
	}
}

func sortSyncOps(ops []OrgSyncOperation) []OrgSyncOperation {
	sort.Slice(ops, func(i int, j int) bool {
		return ops[i].ElementName < ops[j].ElementName
	})
	return ops
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

func TestMerge(t *testing.T) {
	o1 := OrgConfig{
		Version: 3,
		Resources: orgSyncResources{
			"replicant": []string{
				"a1",
				"a2",
			},
		},
		DRRules: orgSyncDRRules{
			"r1": CoreDRRule{
				Name:      "r1",
				Namespace: "managed",
				Detect: Dict{
					"t": "v",
				},
				Response: List{
					"l1",
					"l2",
				},
			},
			"r2": CoreDRRule{
				Name:      "r2",
				Namespace: "managed",
				Detect: Dict{
					"t": "v",
				},
				Response: List{
					"l1",
					"l2",
				},
			},
		},
	}
	o2 := OrgConfig{
		Resources: orgSyncResources{
			"replicant": []string{
				"a3",
				"a1",
			},
		},
		DRRules: orgSyncDRRules{
			"r1": CoreDRRule{
				Name:      "r1",
				Namespace: "general",
				Detect: Dict{
					"t": "v1",
				},
				Response: List{
					"l11",
					"l21",
				},
			},
		},
	}
	expected := `version: 3
resources:
    replicant:
        - a1
        - a2
        - a3
rules:
    r1:
        name: r1
        namespace: general
        detect:
            t: v1
        respond:
            - l11
            - l21
    r2:
        name: r2
        namespace: managed
        detect:
            t: v
        respond:
            - l1
            - l2
`

	out := o1.Merge(o2)

	yOut, err := yaml.Marshal(out)
	if err != nil {
		t.Errorf("yaml: %v", err)
	}

	if string(yOut) != expected {
		t.Errorf("unexpected config: %s\n!=\n\n%s", string(yOut), expected)
	}

	// Add new test case for hive merging
	t.Run("hive merge", func(t *testing.T) {
		o1 := OrgConfig{
			Version: 3,
			Hives: orgSyncHives{
				"lookup": {
					"record1": SyncHiveData{
						"key1": "val1",
						"key2": "val2",
					},
					"record2": SyncHiveData{
						"key3": "val3",
					},
				},
				"secret": {
					"secret1": SyncHiveData{
						"user": "admin",
					},
				},
			},
		}

		o2 := OrgConfig{
			Hives: orgSyncHives{
				"lookup": {
					"record1": SyncHiveData{
						"key2": "newval2",
						"key4": "val4",
					},
					"record3": SyncHiveData{
						"key5": "val5",
					},
				},
				"secret": {
					"secret2": SyncHiveData{
						"pass": "1234",
					},
				},
			},
		}

		expected := `version: 3
hives:
    lookup:
        record1:
            key1: val1
            key2: newval2
            key4: val4
        record2:
            key3: val3
        record3:
            key5: val5
    secret:
        secret1:
            user: admin
        secret2:
            pass: "1234"
`

		out := o1.Merge(o2)

		yOut, err := yaml.Marshal(out)
		if err != nil {
			t.Errorf("yaml: %v", err)
		}

		if string(yOut) != expected {
			t.Errorf("unexpected hive merge config:\n%s\n!=\n\n%s", string(yOut), expected)
		}
	})
}

func TestPushMultiFiles(t *testing.T) {
	files := map[string][]byte{
		"f1": []byte(`version: 3
resources:
  replicant:
  - a1
  - a2
  - a3
`),
		"r": []byte(`version: 3
include:
- s/f2
- f1
`),
		"s/f2": []byte(`version: 3
include:
- f3
rules:
  r1:
    name: r1
    namespace: managed
    detect:
      t: v1
    respond:
    - l11
    - l21
  r2:
    name: r2
    namespace: managed
    detect:
      t: v
    respond:
    - l1
    - l2
`),
		"s/f3": []byte(`version: 3
rules:
  r1:
    name: r1
    namespace: general
    detect:
      t: v1
    respond:
    - l11
    - l21
`),
	}

	expected := `version: 3
resources:
    replicant:
        - a1
        - a2
        - a3
rules:
    r1:
        name: r1
        namespace: general
        detect:
            t: v1
        respond:
            - l11
            - l21
    r2:
        name: r2
        namespace: managed
        detect:
            t: v
        respond:
            - l1
            - l2
`

	ldr := func(parent string, configFile string) ([]byte, error) {
		full := filepath.Join(filepath.Dir(parent), configFile)
		d, ok := files[full]
		if !ok {
			return nil, fmt.Errorf("file not found: %s", full)
		}
		return d, nil
	}

	out, err := loadEffectiveConfig("", "r", SyncOptions{
		IncludeLoader: ldr,
	})
	if err != nil {
		t.Errorf("failed to load: %v", err)
	}

	yOut, err := yaml.Marshal(out)
	if err != nil {
		t.Errorf("yaml: %v", err)
	} else if string(yOut) != expected {
		t.Errorf("unexpected config: %s\n!=\n\n%s", string(yOut), expected)
	}
}

func TestSyncOrgValues(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Start by zeroing out all values.
	for _, v := range supportedOrgValues {
		err := org.OrgValueSet(v, "")
		a.NoError(err)
	}

	ov1 := uuid.NewString()
	ov2 := uuid.NewString()
	yamlValues := fmt.Sprintf(`org-value:
  otx: %s
  twilio: %s
`, ov1, ov2)
	orgConf := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlValues), &orgConf))

	ops, err := org.SyncPush(orgConf, SyncOptions{IsForce: true, SyncOrgValues: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "otx", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "twilio", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	ov, err := org.OrgValueGet("otx")
	a.NoError(err)
	a.Equal(ov1, ov.Value)
	ov, err = org.OrgValueGet("twilio")
	a.NoError(err)
	a.Equal(ov2, ov.Value)

	yamlValues = fmt.Sprintf(`org-value:
  otx: %s
`, ov1)
	orgConf = OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlValues), &orgConf))

	ops, err = org.SyncPush(orgConf, SyncOptions{IsForce: true, SyncOrgValues: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "otx"},
		{ElementType: OrgSyncOperationElementType.OrgValue, ElementName: "twilio", IsRemoved: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	ov, err = org.OrgValueGet("otx")
	a.NoError(err)
	a.Equal(ov1, ov.Value)
	ov, err = org.OrgValueGet("twilio")
	a.NoError(err)
	a.Equal("", ov.Value)
}

func TestSyncFullBidirectional(t *testing.T) {
	rawConf := `version: 3
resources:
    api:
        - vt
        - insight
    replicant:
        - infrastructure-service
        - integrity
        - reliable-tasking
        - responder
        - sigma
        - logging
        - yara
rules:
    vt-domains:
        name: vt-domains
        namespace: general
        detect:
            event: DNS_REQUEST
            metadata_rules:
                length of: true
                op: is greater than
                path: /
                value: 4
            op: lookup
            path: event/DOMAIN_NAME
            resource: lcr://api/vt
        respond:
            - action: report
              name: vt-bad-domain
    vt-hashes:
        name: vt-hashes
        namespace: general
        detect:
            event: CODE_IDENTITY
            metadata_rules:
                length of: true
                op: is greater than
                path: /
                value: 3
            op: lookup
            path: event/HASH
            resource: lcr://api/vt
        respond:
            - action: report
              name: vt-bad-hash
integrity:
    linux-key:
        patterns:
            - /home/*/.ssh/*
        tags: []
        platforms:
            - linux
artifact:
    linux-logs:
        is_ignore_cert: false
        is_delete_after: false
        days_retention: 30
        patterns:
            - /var/log/syslog.1
            - /var/log/auth.log.1
        tags: []
        platforms:
            - linux
    windows-logs:
        is_ignore_cert: false
        is_delete_after: false
        days_retention: 30
        patterns:
            - wel://system:*
            - wel://security:*
            - wel://application:*
        tags: []
        platforms:
            - windows
`
	c := OrgConfig{}
	if err := yaml.Unmarshal([]byte(rawConf), &c); err != nil {
		t.Errorf("failed parsing yaml: %v", err)
	}
	newConf, err := yaml.Marshal(c)
	if err != nil {
		t.Errorf("failed producing yaml: %v", err)
	}
	if string(newConf) != rawConf {
		t.Errorf("round trip through yaml failed to produce same output: %s\n\n!=\n\n%s", newConf, rawConf)
	}
}

func deleteYaraRules(org *Organization) {
	rules, _ := org.IntegrityRules()
	for ruleName := range rules {
		org.IntegrityRuleDelete(ruleName)
	}
}

func TestSyncInstallationKeys(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	deleteAllInstallationKeys(org)
	defer deleteAllInstallationKeys(org)

	keys, err := org.InstallationKeys()
	a.NoError(err)
	a.Empty(keys)

	// sync rules in dry run
	orgKeys := `
installation_keys:
  testk1:
    desc: testk1
    tags:
      - t1
      - t2
    use_public_root_ca: true
  testk2:
    desc: testk2
    tags:
      - t1
      - t2
    use_public_root_ca: true
  testk3:
    desc: testk3
    tags:
      - t1
      - t2
    use_public_root_ca: false
`
	orgConfig := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(orgKeys), &orgConfig))

	ops, err := org.SyncPush(orgConfig, SyncOptions{IsDryRun: true, SyncInstallationKeys: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk1", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk2", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk3", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	keys, err = org.InstallationKeys()
	a.NoError(err)
	a.Empty(keys)

	// no dry run
	ops, err = org.SyncPush(orgConfig, SyncOptions{SyncInstallationKeys: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	keys, err = org.InstallationKeys()
	a.NoError(err)
	a.Equal(len(orgConfig.InstallationKeys), len(keys))
	for _, k := range keys {
		configKey, found := orgConfig.InstallationKeys[k.Description]
		a.True(found)
		a.True(configKey.EqualsContent(k))
	}

	// force sync in dry run
	orgKeysForce := `
installation_keys:
  testk1:
    desc: testk1
    tags:
      - t1
      - t2
    use_public_root_ca: true
  testk4:
    desc: testk4
    tags:
      - t1
    use_public_root_ca: true
  testk3:
    desc: testk3
    tags:
      - t1
      - t2
`
	orgConfigForce := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(orgKeysForce), &orgConfigForce))

	ops, err = org.SyncPush(orgConfigForce, SyncOptions{IsDryRun: true, SyncInstallationKeys: true, IsForce: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk1"},
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk3"},
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk2", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.InstallationKey, ElementName: "testk4", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
	keysForce, err := org.InstallationKeys()
	a.NoError(err)
	for _, k := range keysForce {
		configKey, found := orgConfig.InstallationKeys[k.Description]
		a.True(found)
		a.True(configKey.EqualsContent(k))
	}

	// no dry run
	ops, err = org.SyncPush(orgConfigForce, SyncOptions{SyncInstallationKeys: true, IsForce: true})
	a.NoError(err)
	a.Equal(expectedOps, sortSyncOps(ops))
	keysForce, err = org.InstallationKeys()
	a.NoError(err)
	a.Equal(len(orgConfigForce.InstallationKeys), len(keysForce))
	for _, k := range keysForce {
		configKey, found := orgConfigForce.InstallationKeys[k.Description]
		a.True(found)
		a.True(configKey.EqualsContent(k))
	}
}

func deleteAllInstallationKeys(org *Organization) {
	keys, _ := org.InstallationKeys()
	for _, k := range keys {
		org.DelInstallationKey(k.ID)
	}
	time.Sleep(1 * time.Second)
}

func TestSyncOrgExtensions(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	orgExtensions, err := org.Extensions()
	a.NoError(err)
	for _, ext := range orgExtensions {
		a.NoError(org.UnsubscribeFromExtension(ext))
	}

	yamlValues := `extensions:
  - ext-reliable-tasking
  - ext-sensor-cull
`
	orgConf := OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlValues), &orgConf))

	ops, err := org.SyncPush(orgConf, SyncOptions{IsForce: true, SyncExtensions: true})
	a.NoError(err)
	expectedOps := sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Extension, ElementName: "ext-reliable-tasking", IsAdded: true},
		{ElementType: OrgSyncOperationElementType.Extension, ElementName: "ext-sensor-cull", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))

	yamlValues = `extensions:
  - ext-reliable-tasking
  - binlib
`
	orgConf = OrgConfig{}
	a.NoError(yaml.Unmarshal([]byte(yamlValues), &orgConf))

	ops, err = org.SyncPush(orgConf, SyncOptions{IsForce: true, SyncExtensions: true})
	a.NoError(err)
	expectedOps = sortSyncOps([]OrgSyncOperation{
		{ElementType: OrgSyncOperationElementType.Extension, ElementName: "ext-reliable-tasking"},
		{ElementType: OrgSyncOperationElementType.Extension, ElementName: "ext-sensor-cull", IsRemoved: true},
		{ElementType: OrgSyncOperationElementType.Extension, ElementName: "binlib", IsAdded: true},
	})
	a.Equal(expectedOps, sortSyncOps(ops))
}
