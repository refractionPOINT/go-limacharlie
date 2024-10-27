package limacharlie

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	OrgConfigLatestVersion = 3
)

// Describes which configuration
// types to Sync.
type SyncOptions struct {
	// Force makes the remove Org an exact mirror of the
	// configuration provided, adding and removing.
	// Otherwise elements will only be added, not removed.
	IsForce bool `json:"is_force"`

	// IgnoreInaccessible ignores elements that are
	// locked and cannot be modified by the credentials
	// currently in use.
	IsIgnoreInaccessible bool `json:"ignore_inaccessible"`

	// Only simulate changes to the Org.
	IsDryRun bool `json:"is_dry_run"`

	// Tags used with isForce if tags set force delete will only delete rules with matched tags
	Tags []string `json:"tags"`

	SyncDRRules          bool            `json:"sync_dr"`
	SyncOutputs          bool            `json:"sync_outputs"`
	SyncResources        bool            `json:"sync_resources"`
	SyncExtensions       bool            `json:"sync_extensions"`
	SyncIntegrity        bool            `json:"sync_integrity"`
	SyncFPRules          bool            `json:"sync_fp"`
	SyncExfil            bool            `json:"sync_exfil"`
	SyncArtifacts        bool            `json:"sync_artifacts"`
	SyncOrgValues        bool            `json:"sync_org_values"`
	SyncHives            map[string]bool `json:"sync_hives"`
	SyncInstallationKeys bool            `json:"sync_installation_keys"`
	SyncYara             bool            `json:"sync_yara"`

	IncludeLoader IncludeLoaderCB `json:"-"`
}

var KnownHives = []string{
	"dr-general",
	"dr-managed",
	"dr-service",
	"fp",
	"cloud_sensor",
	"extension_config",
	"yara",
	"secret",
	"lookup",
	"query",
}

func SyncAll() SyncOptions {
	return SyncOptions{
		SyncDRRules:          true,
		SyncOutputs:          true,
		SyncResources:        true,
		SyncExtensions:       true,
		SyncIntegrity:        true,
		SyncFPRules:          true,
		SyncExfil:            true,
		SyncArtifacts:        true,
		SyncOrgValues:        true,
		SyncInstallationKeys: true,
		SyncYara:             true,
		SyncHives: map[string]bool{
			"dr-general":       true,
			"dr-managed":       true,
			"dr-service":       true,
			"fp":               true,
			"cloud_sensor":     true,
			"extension_config": true,
			"yara":             true,
			"secret":           true,
			"lookup":           true,
			"query":            true,
		},
	}
}

type IncludeLoaderCB = func(parentFilePath string, filePathToInclude string) ([]byte, error)

var supportedOrgValues []string = []string{
	"vt",
	"otx",
	"domain",
	"shodan",
	"pagerduty",
	"twilio",
	"socprime",
	"alphamountain",
	"pangea",
	"hybrid-analysis",
	"echotrail",
	"greynoise",
}

type DRRuleName = string

type OrgSyncFPRule struct {
	Detection Dict `json:"data" yaml:"data"`
}

func (r OrgSyncFPRule) DetectionEquals(fpRule FPRule) bool {
	orgRuleDetectionBytes, err := json.Marshal(r.Detection)
	if err != nil {
		return false
	}
	fpRuleDetectionBytes, err := json.Marshal(fpRule.Detection)
	if err != nil {
		return false
	}
	return string(orgRuleDetectionBytes) == string(fpRuleDetectionBytes)
}

type OrgSyncIntegrityRule struct {
	Patterns  []string `json:"patterns" yaml:"patterns"`
	Tags      []string `json:"tags" yaml:"tags"`
	Platforms []string `json:"platforms" yaml:"platforms"`
}

func (oir OrgSyncIntegrityRule) EqualsContent(ir IntegrityRule) bool {
	orgRulePatterns := oir.Patterns
	if orgRulePatterns == nil {
		orgRulePatterns = []string{}
	}
	orgRuleTags := oir.Tags
	if orgRuleTags == nil {
		orgRuleTags = []string{}
	}
	orgRulePlatforms := oir.Platforms
	if orgRulePlatforms == nil {
		orgRulePlatforms = []string{}
	}
	bytes, err := json.Marshal(Dict{
		"patterns":  orgRulePatterns,
		"tags":      orgRuleTags,
		"platforms": orgRulePlatforms,
	})
	if err != nil {
		return false
	}

	rulePattern := ir.Patterns
	if rulePattern == nil {
		rulePattern = []string{}
	}
	ruleTags := ir.Filters.Tags
	if ruleTags == nil {
		ruleTags = []string{}
	}
	rulePlatforms := ir.Filters.Platforms
	if rulePlatforms == nil {
		rulePlatforms = []string{}
	}
	otherBytes, err := json.Marshal(Dict{
		"patterns":  rulePattern,
		"tags":      ruleTags,
		"platforms": rulePlatforms,
	})
	if err != nil {
		return false
	}
	return string(bytes) == string(otherBytes)
}

type OrgSyncArtifactRule struct {
	IsIgnoreCert   bool     `json:"is_ignore_cert" yaml:"is_ignore_cert"`
	IsDeleteAfter  bool     `json:"is_delete_after" yaml:"is_delete_after"`
	DaysRetentions uint     `json:"days_retention" yaml:"days_retention"`
	Patterns       []string `json:"patterns" yaml:"patterns"`
	Tags           []string `json:"tags" yaml:"tags"`
	Platforms      []string `json:"platforms" yaml:"platforms"`
}

func (oar OrgSyncArtifactRule) ToArtifactRule() ArtifactRule {
	return ArtifactRule{
		IsIgnoreCert:   oar.IsIgnoreCert,
		IsDeleteAfter:  oar.IsDeleteAfter,
		DaysRetentions: oar.DaysRetentions,
		Patterns:       oar.Patterns,
		Filters: ArtifactRuleFilter{
			Tags:      oar.Tags,
			Platforms: oar.Platforms,
		},
	}
}

func (oar OrgSyncArtifactRule) FromArtifactRule(artifact ArtifactRule) OrgSyncArtifactRule {
	oar.IsIgnoreCert = artifact.IsIgnoreCert
	oar.IsDeleteAfter = artifact.IsDeleteAfter
	oar.DaysRetentions = artifact.DaysRetentions
	oar.Patterns = artifact.Patterns
	oar.Tags = artifact.Filters.Tags
	oar.Platforms = artifact.Filters.Platforms
	return oar
}

func (oar OrgSyncArtifactRule) ToJson() ([]byte, error) {
	if oar.Patterns == nil {
		oar.Patterns = []string{}
	}
	if oar.Tags == nil {
		oar.Tags = []string{}
	}
	if oar.Platforms == nil {
		oar.Platforms = []string{}
	}
	return json.Marshal(oar)
}

func (oar OrgSyncArtifactRule) EqualsContent(artifact ArtifactRule) bool {
	bytes, err := oar.ToJson()
	if err != nil {
		return false
	}
	otherBytes, err := OrgSyncArtifactRule{}.FromArtifactRule(artifact).ToJson()
	if err != nil {
		return false
	}
	return string(bytes) == string(otherBytes)
}

type orgSyncResources = map[ResourceName][]string
type orgSyncExtensions = []ExtensionName
type orgSyncDRRules = map[DRRuleName]CoreDRRule
type orgSyncFPRules = map[FPRuleName]OrgSyncFPRule
type orgSyncOutputs = map[OutputName]OutputConfig
type orgSyncIntegrityRules = map[IntegrityRuleName]OrgSyncIntegrityRule
type orgSyncExfilRules = ExfilRulesType
type orgSyncArtifacts = map[ArtifactRuleName]OrgSyncArtifactRule
type orgSyncOrgValues = map[OrgValueName]OrgValue
type orgSyncHives = map[HiveName]map[HiveKey]SyncHiveData
type orgSyncInstallationKeys = map[InstallationKeyName]InstallationKey
type orgSyncYara = struct {
	Rules   map[YaraRuleName]YaraRule     `json:"rules,omitempty" yaml:"rules,omitempty"`
	Sources map[YaraSourceName]YaraSource `json:"sources,omitempty" yaml:"sources,omitempty"`
}

type OrgConfig struct {
	Version          int                     `json:"version" yaml:"version"`
	Includes         []string                `json:"includes" yaml:"includes"`
	Resources        orgSyncResources        `json:"resources,omitempty" yaml:"resources,omitempty"`
	Extensions       orgSyncExtensions       `json:"extensions,omitempty" yaml:"extensions,omitempty"`
	DRRules          orgSyncDRRules          `json:"rules,omitempty" yaml:"rules,omitempty"`
	FPRules          orgSyncFPRules          `json:"fps,omitempty" yaml:"fps,omitempty"`
	Outputs          orgSyncOutputs          `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Integrity        orgSyncIntegrityRules   `json:"integrity,omitempty" yaml:"integrity,omitempty"`
	Exfil            *orgSyncExfilRules      `json:"exfil,omitempty" yaml:"exfil,omitempty"`
	Artifacts        orgSyncArtifacts        `json:"artifact,omitempty" yaml:"artifact,omitempty"`
	OrgValues        orgSyncOrgValues        `json:"org-value,omitempty" yaml:"org-value,omitempty"`
	Hives            orgSyncHives            `json:"hives,omitempty" yaml:"hives,omitempty"`
	InstallationKeys orgSyncInstallationKeys `json:"installation_keys,omitempty" yaml:"installation_keys,omitempty"`
	Yara             *orgSyncYara            `json:"yara,omitempty" yaml:"yara,omitempty"`
}

type orgConfigRaw OrgConfig

func (o *OrgConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Custom yaml unmarshal since our official format
	// for config files supports the "include" key containing
	// either a single import or a list of imports.

	// Start with the base unmarshal.
	org := orgConfigRaw{}
	if err := unmarshal(&org); err != nil {
		return err
	}

	// Manually unmarshal the includes.
	singleInclude := struct {
		D string `yaml:"include"`
	}{}
	multiInclude := struct {
		D []string `yaml:"include"`
	}{}

	if err := unmarshal(&singleInclude); err == nil && len(singleInclude.D) != 0 {
		org.Includes = []string{singleInclude.D}
	} else if err := unmarshal(&multiInclude); err == nil {
		org.Includes = multiInclude.D
	} else {
		// Is there an unknown include statement.
		d := map[string]interface{}{}
		unmarshal(&d)
		if inc, ok := d["include"]; ok {
			return fmt.Errorf("unknown include format: %T", inc)
		}
	}

	*o = OrgConfig(org)
	return nil
}

func (o OrgConfig) Merge(conf OrgConfig) OrgConfig {
	o.Resources = o.mergeResources(conf.Resources)
	o.Extensions = o.mergeExtensions(conf.Extensions)
	o.DRRules = o.mergeDRRules(conf.DRRules)
	o.FPRules = o.mergeFPRules(conf.FPRules)
	o.Outputs = o.mergeOutputs(conf.Outputs)
	o.Integrity = o.mergeIntegrity(conf.Integrity)
	o.Exfil = o.mergeExfil(conf.Exfil)
	o.Artifacts = o.mergeArtifacts(conf.Artifacts)
	o.OrgValues = o.mergeOrgValues(conf.OrgValues)
	o.Hives = o.mergeHives(conf.Hives)
	o.InstallationKeys = o.mergeInstallationKeys(conf.InstallationKeys)
	o.Yara = o.mergeYara(conf.Yara)
	return o
}

func (a OrgConfig) mergeResources(b orgSyncResources) orgSyncResources {
	if a.Resources == nil && b == nil {
		return nil
	}
	n := map[string][]string{}
	for k, v := range a.Resources {
		n[k] = v
	}
	for k, v := range b {
		s := map[string]struct{}{}
		if e, ok := n[k]; ok {
			for _, sub := range e {
				s[sub] = struct{}{}
			}
		}
		for _, sub := range v {
			if _, ok := s[sub]; !ok {
				n[k] = append(n[k], sub)
			}
		}
	}
	return n
}

func (a OrgConfig) mergeExtensions(b orgSyncExtensions) orgSyncExtensions {
	if a.Extensions == nil && b == nil {
		return nil
	}
	n := map[string]struct{}{}
	for _, v := range a.Extensions {
		n[v] = struct{}{}
	}
	for _, v := range b {
		n[v] = struct{}{}
	}
	l := []ExtensionName{}
	for k := range n {
		l = append(l, k)
	}
	return l
}

func (a OrgConfig) mergeDRRules(b orgSyncDRRules) orgSyncDRRules {
	if a.DRRules == nil && b == nil {
		return nil
	}
	n := orgSyncDRRules{}
	for k, v := range a.DRRules {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a OrgConfig) mergeFPRules(b orgSyncFPRules) orgSyncFPRules {
	if a.FPRules == nil && b == nil {
		return nil
	}
	n := orgSyncFPRules{}
	for k, v := range a.FPRules {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a OrgConfig) mergeOutputs(b orgSyncOutputs) orgSyncOutputs {
	if a.Outputs == nil && b == nil {
		return nil
	}
	n := orgSyncOutputs{}
	for k, v := range a.Outputs {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a OrgConfig) mergeIntegrity(b orgSyncIntegrityRules) orgSyncIntegrityRules {
	if a.Integrity == nil && b == nil {
		return nil
	}
	n := orgSyncIntegrityRules{}
	for k, v := range a.Integrity {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a OrgConfig) mergeHives(hiveConfig orgSyncHives) orgSyncHives {
	if a.Hives == nil && hiveConfig == nil {
		return orgSyncHives{}
	}

	n := orgSyncHives{}
	for k, v := range a.Hives {
		n[k] = v
	}
	for k, v := range hiveConfig {
		n[k] = v
	}
	return n
}

func (a OrgConfig) mergeInstallationKeys(ikeys orgSyncInstallationKeys) orgSyncInstallationKeys {
	nk := orgSyncInstallationKeys{}
	for k, v := range a.InstallationKeys {
		nk[k] = v
	}
	for k, v := range ikeys {
		nk[k] = v
	}
	return ikeys
}

func (a OrgConfig) mergeYara(yara *orgSyncYara) *orgSyncYara {
	ny := &orgSyncYara{}
	if a.Yara != nil && a.Yara.Sources != nil && yara != nil && yara.Sources != nil {
		ny.Sources = map[YaraSourceName]YaraSource{}
		for k, v := range a.Yara.Sources {
			ny.Sources[k] = v
		}
		if yara != nil {
			for k, v := range yara.Sources {
				ny.Sources[k] = v
			}
		}
	}
	if a.Yara != nil && a.Yara.Rules != nil && yara != nil && yara.Rules != nil {
		ny.Rules = map[YaraRuleName]YaraRule{}
		for k, v := range a.Yara.Rules {
			ny.Rules[k] = v
		}
		if yara != nil {
			for k, v := range yara.Rules {
				ny.Rules[k] = v
			}
		}
	}

	return yara
}

func IsInterfaceNil(v interface{}) bool {
	return v == nil || reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()
}

func (a OrgConfig) mergeExfil(b *orgSyncExfilRules) *orgSyncExfilRules {
	if IsInterfaceNil(a.Exfil) && IsInterfaceNil(b) {
		return nil
	}
	if IsInterfaceNil(a.Exfil) {
		a.Exfil = &orgSyncExfilRules{}
	}
	if IsInterfaceNil(b) {
		b = &orgSyncExfilRules{}
	}
	n := &orgSyncExfilRules{}
	if a.Exfil.Performance != nil || b.Performance != nil {
		n.Performance = Dict{}
		for k, v := range a.Exfil.Performance {
			n.Performance[k] = v
		}
		for k, v := range b.Performance {
			n.Performance[k] = v
		}
	}
	if a.Exfil.Events != nil || b.Events != nil {
		n.Events = map[string]ExfilRuleEvent{}
		for k, v := range a.Exfil.Events {
			n.Events[k] = v
		}
		for k, v := range b.Events {
			n.Events[k] = v
		}
	}
	if a.Exfil.Watches != nil || b.Watches != nil {
		n.Watches = map[string]ExfilRuleWatch{}
		for k, v := range a.Exfil.Watches {
			n.Watches[k] = v
		}
		for k, v := range b.Watches {
			n.Watches[k] = v
		}
	}
	return n
}

func (a OrgConfig) mergeArtifacts(b orgSyncArtifacts) orgSyncArtifacts {
	if a.Artifacts == nil && b == nil {
		return nil
	}
	n := orgSyncArtifacts{}
	for k, v := range a.Artifacts {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a OrgConfig) mergeOrgValues(b orgSyncOrgValues) orgSyncOrgValues {
	if a.OrgValues == nil && b == nil {
		return nil
	}
	n := orgSyncOrgValues{}
	for k, v := range a.OrgValues {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

var OrgSyncOperationElementType = struct {
	DRRule          string
	FPRule          string
	Output          string
	Resource        string
	Extension       string
	Integrity       string
	ExfilEvent      string
	ExfilWatch      string
	Artifact        string
	NetPolicy       string
	OrgValue        string
	Hives           string
	InstallationKey string
	YaraRule        string
	YaraSource      string
}{
	DRRule:          "dr-rule",
	FPRule:          "fp-rule",
	Output:          "output",
	Resource:        "resource",
	Extension:       "extension",
	Integrity:       "integrity",
	ExfilEvent:      "exfil-list",
	ExfilWatch:      "exfil-watch",
	Artifact:        "artifact",
	OrgValue:        "org-value",
	Hives:           "hives",
	InstallationKey: "installation-key",
	YaraRule:        "yara-rule",
	YaraSource:      "yara-source",
}

type OrgSyncOperation struct {
	ElementType string `json:"type"`
	ElementName string `json:"name"`
	IsAdded     bool   `json:"is_added"`
	IsRemoved   bool   `json:"is_removed"`
}

func (o OrgSyncOperation) String() string {
	op := "="
	if o.IsAdded {
		op = "+"
	} else if o.IsRemoved {
		op = "-"
	}
	return fmt.Sprintf("%s %s %s", op, o.ElementType, o.ElementName)
}

func (org *Organization) SyncFetch(options SyncOptions) (orgConfig OrgConfig, err error) {
	if options.SyncResources {
		orgConfig.Resources, err = org.syncFetchResources()
		if err != nil {
			return orgConfig, fmt.Errorf("resources: %v", err)
		}
	}
	if options.SyncExtensions {
		orgConfig.Extensions, err = org.syncFetchExtensions()
		if err != nil {
			return orgConfig, fmt.Errorf("extensions: %v", err)
		}
	}
	if options.SyncDRRules {
		who, err := org.client.WhoAmI()
		if err != nil {
			return orgConfig, fmt.Errorf("dr-rule: %v", err)
		}
		orgConfig.DRRules, err = org.syncFetchDRRules(who)
		if err != nil {
			return orgConfig, fmt.Errorf("dr-rule: %v", err)
		}
	}
	if options.SyncFPRules {
		orgConfig.FPRules, err = org.syncFetchFPRules()
		if err != nil {
			return orgConfig, fmt.Errorf("fp-rule: %v", err)
		}
	}
	if options.SyncOutputs {
		orgConfig.Outputs, err = org.syncFetchOutputs()
		if err != nil {
			return orgConfig, fmt.Errorf("outputs: %v", err)
		}
	}
	if options.SyncIntegrity {
		orgConfig.Integrity, err = org.syncFetchIntegrity()
		if err != nil {
			return orgConfig, fmt.Errorf("integrity: %v", err)
		}
	}
	if options.SyncArtifacts {
		orgConfig.Artifacts, err = org.syncFetchArtifacts()
		if err != nil {
			return orgConfig, fmt.Errorf("artifact: %v", err)
		}
	}
	if options.SyncExfil {
		orgConfig.Exfil, err = org.syncFetchExfil()
		if err != nil {
			return orgConfig, fmt.Errorf("exfil: %v", err)
		}
	}
	if options.SyncOrgValues {
		orgConfig.OrgValues, err = org.syncFetchOrgValues()
		if err != nil {
			return orgConfig, fmt.Errorf("org-value: %v", err)
		}
	}
	if options.SyncHives != nil || len(options.SyncHives) != 0 {
		orgConfig.Hives, err = org.syncFetchHive(options.SyncHives)
		if err != nil {
			return orgConfig, fmt.Errorf("sync_hives: %v", err)
		}
	}
	if options.SyncInstallationKeys {
		orgConfig.InstallationKeys, err = org.syncFetchInstallationKeys()
		if err != nil {
			return orgConfig, fmt.Errorf("installation_keys: %v", err)
		}
	}
	if options.SyncYara {
		orgConfig.Yara, err = org.syncFetchYara()
		if err != nil {
			return orgConfig, fmt.Errorf("integrity: %v", err)
		}
	}

	orgConfig.Version = OrgConfigLatestVersion
	return orgConfig, nil
}

func (org *Organization) syncFetchOrgValues() (orgSyncOrgValues, error) {
	return org.getSupportedOrgValues()
}

func (org *Organization) getSupportedOrgValues() (map[OrgValueName]OrgValue, error) {
	ov := map[OrgValueName]OrgValue{}
	for _, ovn := range supportedOrgValues {
		ovi, err := org.OrgValueGet(ovn)
		if err != nil {
			// Likely the value was never set.
			continue
		}
		ov[ovn] = ovi.Value
	}
	return ov, nil
}

func (org *Organization) syncFetchExfil() (*orgSyncExfilRules, error) {
	exfils := &orgSyncExfilRules{}
	orgExfil, err := org.ExfilRules()
	if err != nil {
		return exfils, err
	}

	if len(orgExfil.Events) != 0 {
		exfils.Events = make(map[string]ExfilRuleEvent)
	}
	for name, rule := range orgExfil.Events {
		rule.CreatedBy = ""
		rule.LastUpdated = 0

		exfils.Events[name] = rule
	}

	if len(orgExfil.Performance) != 0 {
		exfils.Performance = make(Dict)
	}
	for name, rule := range orgExfil.Performance {
		exfils.Performance[name] = rule
	}

	if len(orgExfil.Watches) != 0 {
		exfils.Watches = make(map[string]ExfilRuleWatch)
	}
	for name, rule := range orgExfil.Watches {
		rule.CreatedBy = ""
		rule.LastUpdated = 0
		exfils.Watches[name] = rule
	}
	return exfils, nil
}

func (org *Organization) syncFetchArtifacts() (orgSyncArtifacts, error) {
	orgArtifacts, err := org.ArtifactsRules()
	if err != nil {
		return nil, err
	}
	rules := orgSyncArtifacts{}
	for name, artifactRule := range orgArtifacts {
		rules[name] = OrgSyncArtifactRule{
			IsIgnoreCert:   artifactRule.IsIgnoreCert,
			IsDeleteAfter:  artifactRule.IsDeleteAfter,
			DaysRetentions: artifactRule.DaysRetentions,
			Patterns:       artifactRule.Patterns,
			Tags:           artifactRule.Filters.Tags,
			Platforms:      artifactRule.Filters.Platforms,
		}
	}
	return rules, nil
}

func (org *Organization) syncFetchIntegrity() (orgSyncIntegrityRules, error) {
	orgRules, err := org.IntegrityRules()
	if err != nil {
		return nil, err
	}
	rules := orgSyncIntegrityRules{}
	for ruleName, rule := range orgRules {
		rules[ruleName] = OrgSyncIntegrityRule{
			Patterns:  rule.Patterns,
			Tags:      rule.Filters.Tags,
			Platforms: rule.Filters.Platforms,
		}
	}
	return rules, nil
}

func (org *Organization) syncFetchOutputs() (orgSyncOutputs, error) {
	orgOutputs, err := org.Outputs()
	if err != nil {
		return nil, err
	}
	// Delete-on-Failure Outputs are associated with temporary
	// outputs so we skip them here.
	for outputName, output := range orgOutputs {
		if !output.DeleteOnFailure {
			continue
		}
		delete(orgOutputs, outputName)
	}
	return orgOutputs, nil
}

func (org *Organization) syncFetchFPRules() (orgSyncFPRules, error) {
	orgRules, err := org.FPRules()
	if err != nil {
		return nil, err
	}
	rules := orgSyncFPRules{}
	for ruleName, rule := range orgRules {
		rule.Name = ""
		rules[ruleName] = OrgSyncFPRule{
			Detection: rule.Detection,
		}
	}
	return rules, nil
}

func (org *Organization) syncFetchResources() (orgSyncResources, error) {
	orgResources, err := org.Resources()
	if err != nil {
		return nil, err
	}

	resources := orgSyncResources{}
	for category, names := range orgResources {
		resourceNames := []string{}
		for name := range names {
			resourceNames = append(resourceNames, name)
		}
		resources[category] = resourceNames
	}
	return resources, nil
}

func (org *Organization) syncFetchExtensions() (orgSyncExtensions, error) {
	orgExtensions, err := org.Extensions()
	if err != nil {
		return nil, err
	}
	return orgExtensions, nil
}

func (org *Organization) syncFetchDRRules(who WhoAmIJsonResponse) (orgSyncDRRules, error) {
	rules := orgSyncDRRules{}
	availableNamespaces := org.resolveAvailableNamespaces(who)
	orgRules, err := org.drRulesFromNamespaces(availableNamespaces)
	if err != nil {
		return rules, err
	}
	for ruleName, rule := range orgRules {
		// ignore replicant rule
		if strings.HasPrefix(ruleName, "__") {
			continue
		}
		rule.Name = ""
		rules[ruleName] = rule
	}
	return rules, nil
}

func (org *Organization) syncFetchInstallationKeys() (orgSyncInstallationKeys, error) {
	ikeys, err := org.InstallationKeys()
	if err != nil {
		return nil, err
	}
	keys := orgSyncInstallationKeys{}
	for _, key := range ikeys {
		key.CreatedAt = 0
		key.ID = ""
		key.Key = ""
		key.JsonKey = ""
		keys[key.Description] = key
	}
	return keys, nil
}

func (org *Organization) syncFetchYara() (*orgSyncYara, error) {
	rules, err := org.YaraListRules()
	if err != nil {
		return nil, err
	}
	for k, rule := range rules {
		rule.Author = ""
		rule.LastUpdated = 0
		rules[k] = rule
	}
	sources, err := org.YaraListSources()
	if err != nil {
		return nil, err
	}
	for k, source := range sources {
		source.Author = ""
		source.LastUpdated = 0
		sources[k] = source
	}
	return &orgSyncYara{
		Rules:   rules,
		Sources: sources,
	}, nil
}

func (org *Organization) SyncPushFromFiles(rootConfigFile string, options SyncOptions) ([]OrgSyncOperation, error) {
	// If no custom loader was included, default to the built-in
	// local file system loader.
	if options.IncludeLoader == nil {
		return nil, fmt.Errorf("no include loader provided")
	}
	conf, err := loadEffectiveConfig("", rootConfigFile, options)
	if err != nil {
		return nil, err
	}
	return org.SyncPush(conf, options)
}

func loadEffectiveConfig(parent string, configFile string, options SyncOptions) (OrgConfig, error) {
	thisConfig, err := loadConfWithOptions(parent, configFile, options)
	if err != nil {
		return OrgConfig{}, err
	}

	includePath := filepath.Join(filepath.Dir(parent), configFile)

	for _, toInclude := range thisConfig.Includes {
		incConf, err := loadEffectiveConfig(includePath, toInclude, options)
		if err != nil {
			return OrgConfig{}, err
		}
		thisConfig = thisConfig.Merge(incConf)
	}
	return thisConfig, nil
}

func loadConfWithOptions(parent string, configFile string, options SyncOptions) (OrgConfig, error) {
	conf, err := options.IncludeLoader(parent, configFile)
	if err != nil {
		return OrgConfig{}, err
	}

	thisConfig := OrgConfig{}
	if err := yaml.Unmarshal(conf, &thisConfig); err != nil {
		return OrgConfig{}, err
	}

	if thisConfig.Version <= 0 {
		return OrgConfig{}, fmt.Errorf("invalid version found (%s): %v", configFile, thisConfig.Version)
	}
	if thisConfig.Version > OrgConfigLatestVersion {
		return OrgConfig{}, fmt.Errorf("version not supported (%s): %v", configFile, thisConfig.Version)
	}
	return thisConfig, nil
}

func LocalFileIncludeLoader(parent string, toInclude string) ([]byte, error) {
	// If this is the first include (empty parent), target file is not absolute, assume CWD.
	root := ""
	var err error
	if parent == "" {
		if !filepath.IsAbs(toInclude) {
			if root, err = os.Getwd(); err != nil {
				return nil, err
			}
		}
	} else {
		root = filepath.Dir(parent)
	}

	toInclude = filepath.Join(root, toInclude)

	f, err := os.Open(toInclude)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (org *Organization) SyncPush(conf OrgConfig, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}

	who, err := org.client.WhoAmI()
	if err != nil {
		return ops, err
	}

	// Order matters to minimize issues
	// of dependance between components.
	if options.SyncResources {
		newOps, err := org.syncResources(conf.Resources, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("resources: %v", err)
		}
	}
	if options.SyncExtensions {
		newOps, err := org.syncExtensions(conf.Extensions, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("extensions: %v", err)
		}
	}
	if options.SyncOrgValues {
		newOps, err := org.syncOrgValues(conf.OrgValues, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("org-value: %v", err)
		}
	}
	if options.SyncDRRules {
		newOps, err := org.syncDRRules(who, conf.DRRules, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("dr-rules: %v", err)
		}
	}
	if options.SyncFPRules {
		newOps, err := org.syncFPRules(conf.FPRules, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("fp-rules: %v", err)
		}
	}
	if options.SyncOutputs {
		newOps, err := org.syncOutputs(conf.Outputs, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("outputs: %v", err)
		}
	}
	if options.SyncIntegrity {
		newOps, err := org.syncIntegrity(conf.Integrity, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("integrity: %v", err)
		}
	}
	if options.SyncArtifacts {
		newOps, err := org.syncArtifacts(conf.Artifacts, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("artifact: %v", err)
		}
	}
	if options.SyncExfil {
		newOps, err := org.syncExfil(conf.Exfil, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("exfil: %v", err)
		}
	}
	if len(options.SyncHives) != 0 {
		newOps, err := org.syncHive(conf.Hives, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("sync_hives: %+v ", err)
		}
	}
	if options.SyncInstallationKeys {
		newOps, err := org.syncInstallationKeys(conf.InstallationKeys, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("installation_keys: %v", err)
		}
	}
	if options.SyncYara {
		newOps, err := org.syncYara(conf.Yara, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("yara: %v", err)
		}
	}

	return ops, nil
}

func (org *Organization) syncOrgValues(values orgSyncOrgValues, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(values) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	existingVals, err := org.getSupportedOrgValues()
	if err != nil {
		return ops, err
	}
	if values == nil {
		values = orgSyncOrgValues{}
	}

	for name, val := range values {
		if v, ok := existingVals[name]; ok && v == val {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.OrgValue,
				ElementName: name,
			})
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.OrgValue,
				ElementName: name,
				IsAdded:     true,
			})
			continue
		}

		if err := org.OrgValueSet(name, val); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.OrgValue,
			ElementName: name,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// remove non existing in config
	existingVals, err = org.getSupportedOrgValues()
	if err != nil {
		return ops, err
	}

	for name, v := range existingVals {
		_, found := values[name]
		if found || v == "" {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.OrgValue,
				ElementName: name,
				IsRemoved:   true,
			})
			continue
		}

		if err := org.OrgValueSet(name, ""); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.OrgValue,
			ElementName: name,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncArtifacts(artifacts orgSyncArtifacts, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(artifacts) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgArtifacts, err := org.ArtifactsRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}

	for ruleName, artifact := range artifacts {
		orgArtifact, found := orgArtifacts[ruleName]
		if found {
			if artifact.EqualsContent(orgArtifact) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Artifact,
					ElementName: ruleName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Artifact,
				ElementName: ruleName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.ArtifactRuleAdd(ruleName, artifact.ToArtifactRule()); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Artifact,
			ElementName: ruleName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// remove non existing in config
	orgArtifacts, err = org.ArtifactsRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}

	for ruleName := range orgArtifacts {
		_, found := artifacts[ruleName]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Artifact,
				ElementName: ruleName,
				IsRemoved:   true,
			})
			continue
		}

		if err := org.ArtifactRuleDelete(ruleName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Artifact,
			ElementName: ruleName,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncExfil(exfil *orgSyncExfilRules, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && (exfil == nil || (len(exfil.Events) == 0 && len(exfil.Performance) == 0 && len(exfil.Watches) == 0)) {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgRules, err := org.ExfilRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	if exfil == nil {
		exfil = &orgSyncExfilRules{}
	}

	// watch
	for ruleName, watch := range exfil.Watches {
		orgWatch, found := orgRules.Watches[ruleName]
		if found {
			if watch.EqualsContent(orgWatch) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.ExfilWatch,
					ElementName: ruleName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.ExfilWatch,
				ElementName: ruleName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.ExfilRuleWatchAdd(ruleName, watch); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.ExfilWatch,
			ElementName: ruleName,
			IsAdded:     true,
		})
	}

	for ruleName, event := range exfil.Events {
		orgEvent, found := orgRules.Events[ruleName]
		if found {
			if event.EqualsContent(orgEvent) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.ExfilEvent,
					ElementName: ruleName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.ExfilEvent,
				ElementName: ruleName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.ExfilRuleEventAdd(ruleName, event); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.ExfilEvent,
			ElementName: ruleName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// remove rules not in config
	orgRules, err = org.ExfilRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}

	for ruleName := range orgRules.Watches {
		_, found := exfil.Watches[ruleName]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.ExfilWatch,
				ElementName: ruleName,
				IsRemoved:   true,
			})
			continue
		}

		if err := org.ExfilRuleWatchDelete(ruleName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.ExfilWatch,
			ElementName: ruleName,
			IsRemoved:   true,
		})
	}

	for ruleName := range orgRules.Events {
		_, found := exfil.Events[ruleName]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.ExfilEvent,
				ElementName: ruleName,
				IsRemoved:   true,
			})
			continue
		}

		if err := org.ExfilRuleEventDelete(ruleName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.ExfilEvent,
			ElementName: ruleName,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncIntegrity(integrity orgSyncIntegrityRules, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(integrity) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgIntRules, err := org.IntegrityRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}

	for ruleName, rule := range integrity {
		orgIntRules, found := orgIntRules[ruleName]
		if found {
			if rule.EqualsContent(orgIntRules) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Integrity,
					ElementName: ruleName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Integrity,
				ElementName: ruleName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.IntegrityRuleAdd(ruleName, IntegrityRule{
			Patterns: rule.Patterns,
			Filters: IntegrityRuleFilter{
				Tags:      rule.Tags,
				Platforms: rule.Platforms,
			},
		}); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Integrity,
			ElementName: ruleName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// refetch
	orgIntRules, err = org.IntegrityRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	// list the existing rules and remove the ones not in our list
	for ruleName := range orgIntRules {
		_, found := integrity[ruleName]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Integrity,
				ElementName: ruleName,
				IsRemoved:   true,
			})
			continue
		}
		if err := org.IntegrityRuleDelete(ruleName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Integrity,
			ElementName: ruleName,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncOutputs(outputs orgSyncOutputs, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(outputs) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgOutputs, err := org.Outputs()
	if err != nil {
		return ops, err
	}

	for outputName, output := range outputs {
		// take the key for the name as the conf name might be empty
		output.Name = outputName
		orgOutput, found := orgOutputs[outputName]
		if found {
			if output.Equals(orgOutput) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Output,
					ElementName: outputName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Output,
				ElementName: outputName,
				IsAdded:     true,
			})
			continue
		}
		output.Name = outputName
		if _, err := org.OutputAdd(output); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Output,
			ElementName: outputName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// refetch
	orgOutputs, err = org.Outputs()
	if err != nil {
		return ops, err
	}

	// Go through existing outputs and removes the ones not in our list
	for outputName := range orgOutputs {
		_, found := outputs[outputName]
		if found {
			continue
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Output,
				ElementName: outputName,
				IsRemoved:   true,
			})
			continue
		}
		if _, err := org.OutputDel(outputName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Output,
			ElementName: outputName,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncFPRules(rules orgSyncFPRules, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(rules) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgRules, err := org.FPRules()
	if err != nil {
		return ops, err
	}

	// Add rules that should be replaced first
	for ruleName, rule := range rules {
		orgRule, found := orgRules[ruleName]
		if found {
			if rule.DetectionEquals(orgRule) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.FPRule,
					ElementName: ruleName,
				})
				continue
			}
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.FPRule,
				ElementName: ruleName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.FPRuleAdd(ruleName, rule.Detection, FPRuleOptions{IsReplace: true}); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.FPRule,
			ElementName: ruleName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// refetch
	orgRules, err = org.FPRules()
	if err != nil {
		return ops, err
	}

	// Go through existing rules and removes the ones not in our list
	for ruleName := range orgRules {
		_, found := rules[ruleName]
		if found {
			continue
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.FPRule,
				ElementName: ruleName,
				IsRemoved:   true,
			})
			continue
		}
		if err := org.FPRuleDelete(ruleName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.FPRule,
			ElementName: ruleName,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncInstallationKeys(ikeys orgSyncInstallationKeys, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(ikeys) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgKeys, err := org.InstallationKeys()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	orgKeyMap := map[string]InstallationKey{}
	for _, k := range orgKeys {
		orgKeyMap[k.Description] = k
	}

	for keyName, key := range ikeys {
		orgKey, found := orgKeyMap[keyName]
		if found {
			if key.EqualsContent(orgKey) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.InstallationKey,
					ElementName: keyName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.InstallationKey,
				ElementName: keyName,
				IsAdded:     true,
			})
			continue
		}

		if key.ID == "" {
			// The real primary key is the IID, so make sure
			// we overwrite it if we found a match.
			key.ID = orgKey.ID
		}

		if _, err := org.AddInstallationKey(key); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.InstallationKey,
			ElementName: keyName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// refetch
	orgKeys, err = org.InstallationKeys()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	// list the existing rules and remove the ones not in our list
	for _, k := range orgKeys {
		_, found := ikeys[k.Description]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.InstallationKey,
				ElementName: k.Description,
				IsRemoved:   true,
			})
			continue
		}
		if err := org.DelInstallationKey(k.ID); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.InstallationKey,
			ElementName: k.Description,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) syncYara(yara *orgSyncYara, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && (yara == nil || (len(yara.Rules) == 0 && len(yara.Sources) == 0)) {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgRules, err := org.YaraListRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	orgSources, err := org.YaraListSources()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}

	if yara == nil {
		yara = &orgSyncYara{}
	}
	if yara.Sources == nil {
		yara.Sources = map[string]YaraSource{}
	}
	if yara.Rules == nil {
		yara.Rules = map[string]YaraRule{}
	}

	for sourceName, source := range yara.Sources {
		orgSource, found := orgSources[sourceName]
		if found {
			if source.EqualsContent(orgSource) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.YaraSource,
					ElementName: sourceName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.YaraSource,
				ElementName: sourceName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.YaraSourceAdd(sourceName, source); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.YaraSource,
			ElementName: sourceName,
			IsAdded:     true,
		})
	}

	for ruleName, rule := range yara.Rules {
		orgRule, found := orgRules[ruleName]
		if found {
			if rule.EqualsContent(orgRule) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.YaraRule,
					ElementName: ruleName,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.YaraRule,
				ElementName: ruleName,
				IsAdded:     true,
			})
			continue
		}

		if err := org.YaraRuleAdd(ruleName, rule); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.YaraRule,
			ElementName: ruleName,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// refetch
	orgRules, err = org.YaraListRules()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	orgSources, err = org.YaraListSources()
	if err != nil && (!IsServiceNotRegisteredError(err) || !options.IsDryRun) {
		return ops, err
	}
	// list the existing rules and remove the ones not in our list
	for ruleName := range orgRules {
		_, found := yara.Rules[ruleName]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.YaraRule,
				ElementName: ruleName,
				IsRemoved:   true,
			})
			continue
		}
		if err := org.YaraRuleDelete(ruleName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.YaraRule,
			ElementName: ruleName,
			IsRemoved:   true,
		})
	}

	for sourceName := range orgSources {
		_, found := yara.Sources[sourceName]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.YaraSource,
				ElementName: sourceName,
				IsRemoved:   true,
			})
			continue
		}
		if err := org.YaraSourceDelete(sourceName); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.YaraSource,
			ElementName: sourceName,
			IsRemoved:   true,
		})
	}
	return ops, nil
}

func (org *Organization) resolveAvailableNamespaces(who WhoAmIJsonResponse) map[string]struct{} {
	// Check which namespaces we have available.
	availableNamespaces := map[string]struct{}{}
	if who.HasPermissionForOrg(org.client.options.OID, "dr.list") {
		availableNamespaces["general"] = struct{}{}
	}
	if who.HasPermissionForOrg(org.client.options.OID, "dr.list.managed") {
		availableNamespaces["managed"] = struct{}{}
	}
	if who.HasPermissionForOrg(org.client.options.OID, "dr.list.replicant") {
		availableNamespaces["replicant"] = struct{}{}
	}
	return availableNamespaces
}

func (org *Organization) drRulesFromNamespaces(namespaces map[string]struct{}) (existingRules orgSyncDRRules, err error) {
	existingRules = orgSyncDRRules{}
	// Get rules from all the namespaces we have access to.
	for ns := range namespaces {
		tmpRules, err := org.DRRules(WithNamespace(ns))
		if err != nil {
			return existingRules, fmt.Errorf("DRRules %s: %v", ns, err)
		}
		for ruleName, rule := range tmpRules {
			parsedRule := CoreDRRule{}
			if err := rule.UnMarshalToStruct(&parsedRule); err != nil {
				return existingRules, fmt.Errorf("UnMarshalToStruct %s: %v", ruleName, err)
			}
			existingRules[ruleName] = parsedRule
		}
	}
	return existingRules, nil
}

func (org *Organization) syncDRRules(who WhoAmIJsonResponse, rules orgSyncDRRules, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(rules) == 0 {
		return nil, nil
	}

	availableNamespaces := org.resolveAvailableNamespaces(who)
	ops := []OrgSyncOperation{}
	existingRules, err := org.drRulesFromNamespaces(availableNamespaces)
	if err != nil {
		return ops, err
	}

	// Start by adding missing rules.
	for ruleName, rule := range rules {
		// If is_enabled is not set, it defaults to true.
		if rule.IsEnabled == nil {
			isTrue := true
			rule.IsEnabled = &isTrue
		}
		if existingRule, ok := existingRules[ruleName]; ok {
			// A rule with that name is already there.
			// Is it the exact same rule?
			if existingRule.Equal(rule) {
				ops = append(ops, OrgSyncOperation{ElementType: OrgSyncOperationElementType.DRRule, ElementName: ruleName})
				// Nothing to do, move on.
				continue
			}
			// If this is a DryRun, just report the op and move on.
			if options.IsDryRun {
				ops = append(ops, OrgSyncOperation{ElementType: OrgSyncOperationElementType.DRRule, ElementName: ruleName, IsAdded: true})
				continue
			}
			// It must be replaced.
			// If they are in different namespaces, we must
			// delete the old one before setting the new one.
			if !existingRule.IsInSameNamespace(rule) {
				existingNs := existingRule.Namespace
				if existingNs == "" {
					existingNs = "general"
				}
				if err := org.DRRuleDelete(ruleName, WithNamespace(existingNs)); err != nil {
					return ops, fmt.Errorf("DRDelRule %s: %v", ruleName, err)
				}
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{ElementType: OrgSyncOperationElementType.DRRule, ElementName: ruleName, IsAdded: true})
			continue
		}
		if err := org.DRRuleAdd(ruleName, rule.Detect, rule.Response, NewDRRuleOptions{
			IsReplace: true,
			Namespace: rule.Namespace,
			IsEnabled: *rule.IsEnabled,
		}); err != nil {
			return ops, fmt.Errorf("DRRuleAdd %s: %v", ruleName, err)
		}
		ops = append(ops, OrgSyncOperation{ElementType: OrgSyncOperationElementType.DRRule, ElementName: ruleName, IsAdded: true})
	}

	// If we're not Forcing, then we're done.
	if !options.IsForce {
		return ops, nil
	}

	// Remove rules that no longer exist.
	for ruleName, rule := range existingRules {
		if strings.HasPrefix(ruleName, "__") {
			// Ignore legacy special service rules.
			continue
		}
		if _, ok := rules[ruleName]; ok {
			// Still there.
			continue
		}
		// If this is a DryRun, report the op and move on.
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{ElementType: OrgSyncOperationElementType.DRRule, ElementName: ruleName, IsRemoved: true})
			continue
		}
		if err := org.DRRuleDelete(ruleName, WithNamespace(rule.Namespace)); err != nil {
			return ops, fmt.Errorf("DRDelRule %s: %v", ruleName, err)
		}
		ops = append(ops, OrgSyncOperation{ElementType: OrgSyncOperationElementType.DRRule, ElementName: ruleName, IsRemoved: true})
	}

	return ops, nil
}

func (org *Organization) syncResources(resources orgSyncResources, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(resources) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	orgResources, err := org.Resources()
	if err != nil {
		return ops, err
	}

	for resCat, resNames := range resources {
		// The service category is an alias of the
		// legacy replicant category.
		if resCat == "service" {
			resCat = "replicant"
		}
		orgResCat, found := orgResources[resCat]
		if !found {
			// cat does not exist in org, subscribe to all
			for _, resName := range resNames {
				fullResName := fmt.Sprintf("%s/%s", resCat, resName)
				if options.IsDryRun {
					ops = append(ops, OrgSyncOperation{
						ElementType: OrgSyncOperationElementType.Resource,
						ElementName: fullResName,
						IsAdded:     true,
					})
					continue
				}
				if err := org.ResourceSubscribe(resName, resCat); err != nil {
					return ops, err
				}
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Resource,
					ElementName: fullResName,
					IsAdded:     true,
				})
			}
			continue
		}

		for _, resName := range resNames {
			_, found := orgResCat[resName]
			fullResName := fmt.Sprintf("%s/%s", resCat, resName)
			if found {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Resource,
					ElementName: fullResName,
				})
				continue
			}
			if options.IsDryRun {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Resource,
					ElementName: fullResName,
					IsAdded:     true,
				})
				continue
			}
			if err := org.ResourceSubscribe(resName, resCat); err != nil {
				return ops, err
			}
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Resource,
				ElementName: fullResName,
				IsAdded:     true,
			})
		}
	}

	if !options.IsForce {
		return ops, nil
	}

	if len(resources) == 0 {
		return ops, nil
	}

	// Only remove resources if it is present in the config.
	// This avoids unexpected disabling of all configs.
	for orgResCat, orgResNames := range orgResources {
		var resNames []string
		found := false
		if orgResCat == "replicant" || orgResCat == "service" {
			// Check for the replicant -> service possible alias.
			resNames = mergeStringSets(resources["replicant"], resources["service"])
			found = resNames != nil
		} else {
			resNames, found = resources[orgResCat]
		}
		if !found {
			continue
		}
		for orgResName := range orgResNames {

			found := false
			for _, resNameToFind := range resNames {
				found = resNameToFind == orgResName
				if found {
					break
				}
			}
			if found {
				continue
			}

			fullResName := fmt.Sprintf("%s/%s", orgResCat, orgResName)
			if options.IsDryRun {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Resource,
					ElementName: fullResName,
					IsRemoved:   true,
				})
				continue
			}
			if err := org.ResourceUnsubscribe(orgResName, orgResCat); err != nil {
				return ops, err
			}
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Resource,
				ElementName: fullResName,
				IsRemoved:   true,
			})
		}
	}

	return ops, nil
}

func (org *Organization) syncExtensions(extensions orgSyncExtensions, options SyncOptions) ([]OrgSyncOperation, error) {
	if !options.IsForce && len(extensions) == 0 {
		return nil, nil
	}

	ops := []OrgSyncOperation{}
	oe, err := org.Extensions()
	if err != nil {
		return ops, fmt.Errorf("list subscribed extensions: %v", err)
	}
	orgExtensions := map[ExtensionName]struct{}{}
	for _, ext := range oe {
		orgExtensions[ext] = struct{}{}
	}

	newExtensions := map[ExtensionName]struct{}{}
	for _, ext := range extensions {
		newExtensions[ext] = struct{}{}
	}

	for ext := range newExtensions {
		_, isFound := orgExtensions[ext]
		if isFound {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Extension,
				ElementName: ext,
			})
		} else {
			if !options.IsDryRun {
				if err := org.SubscribeToExtension(ext); err != nil {
					return ops, fmt.Errorf("subscribing to %s: %v", ext, err)
				}
			}
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.Extension,
				ElementName: ext,
				IsAdded:     true,
			})
		}
	}

	if !options.IsForce {
		return ops, nil
	}

	for ext := range orgExtensions {
		_, isFound := newExtensions[ext]
		if isFound {
			continue
		}
		if !options.IsDryRun {
			if err := org.UnsubscribeFromExtension(ext); err != nil {
				return ops, fmt.Errorf("unsubscribing to %s: %v", ext, err)
			}
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.Extension,
			ElementName: ext,
			IsRemoved:   true,
		})
	}

	return ops, nil
}

func slicesContainSameItems(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	elementExists := make(map[string]bool)
	for _, item := range slice1 {
		elementExists[item] = true
	}

	for _, item := range slice2 {
		if !elementExists[item] {
			return false
		}
	}

	return true
}

func mergeStringSets(a []string, b []string) []string {
	if a == nil && b == nil {
		return nil
	}
	s := map[string]struct{}{}
	for _, e := range a {
		s[e] = struct{}{}
	}
	for _, e := range b {
		s[e] = struct{}{}
	}
	out := []string{}
	for e := range s {
		out = append(out, e)
	}
	return out
}
