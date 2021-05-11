package limacharlie

import (
	"encoding/json"
	"fmt"
	"strings"
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
}

type DRRuleName = string

type OrgSyncFPRule struct {
	Detection Dict `json:"detect" yaml:"detect"`
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

type OrgConfig struct {
	Resources map[ResourceName][]string                  `json:"resources" yaml:"resources"`
	DRRules   map[DRRuleName]CoreDRRule                  `json:"rules" yaml:"rules"`
	FPRules   map[FPRuleName]OrgSyncFPRule               `json:"fps" yaml:"fps"`
	Outputs   map[OutputName]OutputConfig                `json:"outputs" yaml:"outputs"`
	Integrity map[IntegrityRuleName]OrgSyncIntegrityRule `json:"integrity" yaml:"integrity"`
}

var OrgSyncOperationElementType = struct {
	DRRule    string
	FPRule    string
	Output    string
	Resource  string
	Integrity string
}{
	DRRule:    "dr-rule",
	FPRule:    "fp-rule",
	Output:    "output",
	Resource:  "resource",
	Integrity: "integrity",
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

func (org Organization) SyncFetch(options SyncOptions) (OrgConfig, error) {
	return OrgConfig{}, ErrorNotImplemented
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
		return ops, ErrorNotImplemented
	}
	if options.SyncExfil {
		return ops, ErrorNotImplemented
	}
	if options.SyncNetPolicies {
		return ops, ErrorNotImplemented
	}

	return ops, nil
}

func (org Organization) syncIntegrity(integrity map[IntegrityRuleName]OrgSyncIntegrityRule, options SyncOptions) ([]OrgSyncOperation, error) {
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

func (org Organization) syncOutputs(outputs map[OutputName]OutputConfig, options SyncOptions) ([]OrgSyncOperation, error) {
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

func (org Organization) syncFPRules(rules map[FPRuleName]OrgSyncFPRule, options SyncOptions) ([]OrgSyncOperation, error) {
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

func (org Organization) syncDRRules(who whoAmIJsonResponse, rules map[DRRuleName]CoreDRRule, options SyncOptions) ([]OrgSyncOperation, error) {
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

	ops := []OrgSyncOperation{}
	existingRules := map[DRRuleName]CoreDRRule{}

	// Get rules from all the namespaces we have access to.
	for ns := range availableNamespaces {
		tmpRules, err := org.DRRules(WithNamespace(ns))
		if err != nil {
			return ops, fmt.Errorf("DRRules %s: %v", ns, err)
		}
		for ruleName, rule := range tmpRules {
			parsedRule := CoreDRRule{}
			if err := rule.UnMarshalToStruct(&parsedRule); err != nil {
				return ops, fmt.Errorf("UnMarshalToStruct %s: %v", ruleName, err)
			}
			existingRules[ruleName] = parsedRule
		}
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

func (org Organization) syncResources(resources map[ResourceName][]string, options SyncOptions) ([]OrgSyncOperation, error) {
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
