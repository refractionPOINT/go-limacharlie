package limacharlie

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
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

	SyncDRRules     bool `json:"sync_dr"`
	SyncOutputs     bool `json:"sync_outputs"`
	SyncResources   bool `json:"sync_resources"`
	SyncIntegrity   bool `json:"sync_integrity"`
	SyncFPRules     bool `json:"sync_fp"`
	SyncExfil       bool `json:"sync_exfil"`
	SyncArtifacts   bool `json:"sync_artifacts"`
	SyncNetPolicies bool `json:"sync_net_policies"`

	IncludeLoader IncludeLoaderCB `json:"-"`
}

type IncludeLoaderCB = func(parentFilePath string, filePathToInclude string) ([]byte, error)

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
	LastUpdated uint64   `json:"updated" yaml:"updated"`
	CreatedBy   string   `json:"by" yaml:"by"`
	Patterns    []string `json:"patterns" yaml:"patterns"`
	Tags        []string `json:"tags" yaml:"tags"`
	Platforms   []string `json:"platforms" yaml:"platforms"`
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

type orgSyncResources map[ResourceName][]string
type orgSyncDRRules map[DRRuleName]CoreDRRule
type orgSyncFPRules map[FPRuleName]OrgSyncFPRule
type orgSyncOutputs map[OutputName]OutputConfig
type orgSyncIntegrityRules map[IntegrityRuleName]OrgSyncIntegrityRule
type orgSyncExfilRules ExfilRulesType
type orgSyncArtifacts map[ArtifactRuleName]OrgSyncArtifactRule
type orgSyncNetPolicies NetPoliciesByName

type OrgConfig struct {
	Version     int                   `json:"version" yaml:"version"`
	Includes    []string              `json:"-" yaml:"-"`
	Resources   orgSyncResources      `json:"resources" yaml:"resources"`
	DRRules     orgSyncDRRules        `json:"rules" yaml:"rules"`
	FPRules     orgSyncFPRules        `json:"fps" yaml:"fps"`
	Outputs     orgSyncOutputs        `json:"outputs" yaml:"outputs"`
	Integrity   orgSyncIntegrityRules `json:"integrity" yaml:"integrity"`
	Exfil       orgSyncExfilRules     `json:"exfil" yaml:"exfil"`
	Artifacts   orgSyncArtifacts      `json:"artifact" yaml:"artifact"`
	NetPolicies orgSyncNetPolicies    `json:"net-policy" yaml:"net-policy"`
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
		D []string `yaml::"include"`
	}{}

	if err := unmarshal(&singleInclude); err == nil {
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
	o.Resources = o.Resources.merge(conf.Resources)
	o.DRRules = o.DRRules.merge(conf.DRRules)
	o.FPRules = o.FPRules.merge(conf.FPRules)
	o.Outputs = o.Outputs.merge(conf.Outputs)
	o.Integrity = o.Integrity.merge(conf.Integrity)
	o.Exfil = o.Exfil.merge(conf.Exfil)
	o.Artifacts = o.Artifacts.merge(conf.Artifacts)
	o.NetPolicies = o.NetPolicies.merge(conf.NetPolicies)
	return o
}

func (a orgSyncResources) merge(b orgSyncResources) orgSyncResources {
	if a == nil && b == nil {
		return nil
	}
	n := map[string][]string{}
	for k, v := range a {
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

func (a orgSyncDRRules) merge(b orgSyncDRRules) orgSyncDRRules {
	if a == nil && b == nil {
		return nil
	}
	n := orgSyncDRRules{}
	for k, v := range a {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a orgSyncFPRules) merge(b orgSyncFPRules) orgSyncFPRules {
	if a == nil && b == nil {
		return nil
	}
	n := orgSyncFPRules{}
	for k, v := range a {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a orgSyncOutputs) merge(b orgSyncOutputs) orgSyncOutputs {
	if a == nil && b == nil {
		return nil
	}
	n := orgSyncOutputs{}
	for k, v := range a {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a orgSyncIntegrityRules) merge(b orgSyncIntegrityRules) orgSyncIntegrityRules {
	if a == nil && b == nil {
		return nil
	}
	n := orgSyncIntegrityRules{}
	for k, v := range a {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a orgSyncExfilRules) merge(b orgSyncExfilRules) orgSyncExfilRules {
	n := orgSyncExfilRules{}
	if a.Performance != nil || b.Performance != nil {
		n.Performance = Dict{}
		for k, v := range a.Performance {
			n.Performance[k] = v
		}
		for k, v := range b.Performance {
			n.Performance[k] = v
		}
	}
	if a.Events != nil || b.Events != nil {
		n.Events = map[string]ExfilRuleEvent{}
		for k, v := range a.Events {
			n.Events[k] = v
		}
		for k, v := range b.Events {
			n.Events[k] = v
		}
	}
	if a.Watches != nil || b.Watches != nil {
		n.Watches = map[string]ExfilRuleWatch{}
		for k, v := range a.Watches {
			n.Watches[k] = v
		}
		for k, v := range b.Watches {
			n.Watches[k] = v
		}
	}
	return n
}

func (a orgSyncArtifacts) merge(b orgSyncArtifacts) orgSyncArtifacts {
	if a == nil && b == nil {
		return nil
	}
	n := orgSyncArtifacts{}
	for k, v := range a {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

func (a orgSyncNetPolicies) merge(b orgSyncNetPolicies) orgSyncNetPolicies {
	if a == nil && b == nil {
		return nil
	}
	n := orgSyncNetPolicies{}
	for k, v := range a {
		n[k] = v
	}
	for k, v := range b {
		n[k] = v
	}
	return n
}

var OrgSyncOperationElementType = struct {
	DRRule     string
	FPRule     string
	Output     string
	Resource   string
	Integrity  string
	ExfilEvent string
	ExfilWatch string
	Artifact   string
	NetPolicy  string
}{
	DRRule:     "dr-rule",
	FPRule:     "fp-rule",
	Output:     "output",
	Resource:   "resource",
	Integrity:  "integrity",
	ExfilEvent: "exfil-list",
	ExfilWatch: "exfil-watch",
	Artifact:   "artifact",
	NetPolicy:  "net-policy",
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

func (org Organization) SyncFetch(options SyncOptions) (orgConfig OrgConfig, err error) {
	if options.SyncResources {
		orgConfig.Resources, err = org.syncFetchResources()
		if err != nil {
			return orgConfig, fmt.Errorf("resources: %v", err)
		}
	}
	if options.SyncDRRules {
		who, err := org.client.whoAmI()
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
	if options.SyncNetPolicies {
		orgConfig.NetPolicies, err = org.syncFetchNetPolicies()
		if err != nil {
			return orgConfig, fmt.Errorf("net-policy: %v", err)
		}
	}
	return orgConfig, nil
}

func (org Organization) syncFetchNetPolicies() (orgSyncNetPolicies, error) {
	orgNetPolicies, err := org.NetPolicies()
	if err != nil {
		return nil, err
	}
	netPolicies := orgSyncNetPolicies{}
	for name, policy := range orgNetPolicies {
		netPolicies[name] = policy
	}
	return netPolicies, nil
}

func (org Organization) syncFetchExfil() (orgSyncExfilRules, error) {
	exfils := orgSyncExfilRules{}
	orgExfil, err := org.ExfilRules()
	if err != nil {
		return exfils, err
	}

	if len(orgExfil.Events) != 0 {
		exfils.Events = make(map[string]ExfilRuleEvent)
	}
	for name, rule := range orgExfil.Events {
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
		exfils.Watches[name] = rule
	}
	return exfils, nil
}

func (org Organization) syncFetchArtifacts() (orgSyncArtifacts, error) {
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

func (org Organization) syncFetchIntegrity() (orgSyncIntegrityRules, error) {
	orgRules, err := org.IntegrityRules()
	if err != nil {
		return nil, err
	}
	rules := orgSyncIntegrityRules{}
	for ruleName, rule := range orgRules {
		rules[ruleName] = OrgSyncIntegrityRule{
			LastUpdated: rule.LastUpdated,
			CreatedBy:   rule.CreatedBy,
			Patterns:    rule.Patterns,
			Tags:        rule.Filters.Tags,
			Platforms:   rule.Filters.Platforms,
		}
	}
	return rules, nil
}

func (org Organization) syncFetchOutputs() (orgSyncOutputs, error) {
	orgOutputs, err := org.Outputs()
	if err != nil {
		return nil, err
	}
	return orgOutputs, nil
}

func (org Organization) syncFetchFPRules() (orgSyncFPRules, error) {
	orgRules, err := org.FPRules()
	if err != nil {
		return nil, err
	}
	rules := orgSyncFPRules{}
	for ruleName, rule := range orgRules {
		rules[ruleName] = OrgSyncFPRule{
			Detection: rule.Detection,
		}
	}
	return rules, nil
}

func (org Organization) syncFetchResources() (orgSyncResources, error) {
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

func (org Organization) syncFetchDRRules(who whoAmIJsonResponse) (orgSyncDRRules, error) {
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
		rules[ruleName] = rule
	}
	return rules, nil
}

func (org Organization) SyncPushFromFiles(rootConfigFile string, options SyncOptions) ([]OrgSyncOperation, error) {
	// If no custom loader was included, default to the built-in
	// local file system loader.
	if options.IncludeLoader == nil {
		options.IncludeLoader = org.localFileIncludeLoader
	}
	conf, err := org.loadEffectiveConfig("", rootConfigFile, options)
	if err != nil {
		return nil, err
	}
	return org.SyncPush(conf, options)
}

func (org Organization) loadEffectiveConfig(parent string, configFile string, options SyncOptions) (OrgConfig, error) {
	thisConfig, err := loadConfWithOptions(parent, configFile, options)
	if err != nil {
		return OrgConfig{}, err
	}

	for _, toInclude := range thisConfig.Includes {
		incConf, err := loadConfWithOptions(configFile, toInclude, options)
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

func (org Organization) localFileIncludeLoader(parent string, toInclude string) ([]byte, error) {
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

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (org Organization) SyncPush(conf OrgConfig, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}

	who, err := org.client.whoAmI()
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
	if options.SyncNetPolicies {
		newOps, err := org.syncNetPolicies(conf.NetPolicies, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("net-policy: %v", err)
		}
	}
	return ops, nil
}

func (org Organization) syncNetPolicies(netPolicies orgSyncNetPolicies, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}
	orgNetPolicies, err := org.NetPolicies()
	if err != nil {
		return ops, err
	}

	for name, policy := range netPolicies {
		policy = policy.WithName(name)
		orgPolicy, found := orgNetPolicies[name]
		if found {
			if policy.EqualsContent(orgPolicy) {
				ops = append(ops, OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.NetPolicy,
					ElementName: name,
				})
				continue
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.NetPolicy,
				ElementName: name,
				IsAdded:     true,
			})
			continue
		}

		if err := org.NetPolicyAdd(policy); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.NetPolicy,
			ElementName: name,
			IsAdded:     true,
		})
	}

	if !options.IsForce {
		return ops, nil
	}

	// remove non existing in config
	orgNetPolicies, err = org.NetPolicies()
	if err != nil {
		return ops, err
	}

	for name := range orgNetPolicies {
		_, found := netPolicies[name]
		if found {
			continue
		}

		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{
				ElementType: OrgSyncOperationElementType.NetPolicy,
				ElementName: name,
				IsRemoved:   true,
			})
			continue
		}

		if err := org.NetPolicyDelete(name); err != nil {
			return ops, err
		}
		ops = append(ops, OrgSyncOperation{
			ElementType: OrgSyncOperationElementType.NetPolicy,
			ElementName: name,
			IsRemoved:   true,
		})

	}
	return ops, nil
}

func (org Organization) syncArtifacts(artifacts orgSyncArtifacts, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}
	orgArtifacts, err := org.ArtifactsRules()
	if err != nil {
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
	if err != nil {
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

func (org Organization) syncExfil(exfil orgSyncExfilRules, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}
	orgRules, err := org.ExfilRules()
	if err != nil {
		return ops, err
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
	if err != nil {
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

func (org Organization) syncIntegrity(integrity orgSyncIntegrityRules, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}
	orgIntRules, err := org.IntegrityRules()
	if err != nil {
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
	if err != nil {
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

func (org Organization) syncOutputs(outputs orgSyncOutputs, options SyncOptions) ([]OrgSyncOperation, error) {
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

func (org Organization) syncFPRules(rules orgSyncFPRules, options SyncOptions) ([]OrgSyncOperation, error) {
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

func (org Organization) resolveAvailableNamespaces(who whoAmIJsonResponse) map[string]struct{} {
	// Check which namespaces we have available.
	availableNamespaces := map[string]struct{}{}
	if who.hasPermissionForOrg(org.client.options.OID, "dr.list") {
		availableNamespaces["general"] = struct{}{}
	}
	if who.hasPermissionForOrg(org.client.options.OID, "dr.list.managed") {
		availableNamespaces["managed"] = struct{}{}
	}
	if who.hasPermissionForOrg(org.client.options.OID, "dr.list.replicant") {
		availableNamespaces["replicant"] = struct{}{}
	}
	return availableNamespaces
}

func (org Organization) drRulesFromNamespaces(namespaces map[string]struct{}) (existingRules orgSyncDRRules, err error) {
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

func (org Organization) syncDRRules(who whoAmIJsonResponse, rules orgSyncDRRules, options SyncOptions) ([]OrgSyncOperation, error) {
	availableNamespaces := org.resolveAvailableNamespaces(who)
	ops := []OrgSyncOperation{}
	existingRules, err := org.drRulesFromNamespaces(availableNamespaces)
	if err != nil {
		return ops, err
	}

	// Start by adding missing rules.
	for ruleName, rule := range rules {
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
			IsEnabled: true,
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

func (org Organization) syncResources(resources orgSyncResources, options SyncOptions) ([]OrgSyncOperation, error) {
	ops := []OrgSyncOperation{}
	orgResources, err := org.Resources()
	if err != nil {
		return ops, err
	}

	for resCat, resNames := range resources {
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
				if err := org.Comms().o.ResourceSubscribe(resName, resCat); err != nil {
					return ops, nil
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
			if err := org.Comms().o.ResourceSubscribe(resName, resCat); err != nil {
				return ops, nil
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
		resNames, found := resources[orgResCat]
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
			if !found {
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
