package limacharlie

import (
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
	IsForce bool

	// IgnoreInaccessible ignores elements that are
	// locked and cannot be modified by the credentials
	// currently in use.
	IsIgnoreInaccessible bool

	// Only simulate changes to the Org.
	IsDryRun bool

	SyncDRRules     bool
	SyncOutputs     bool
	SyncResources   bool
	SyncIntegrity   bool
	SyncFPRules     bool
	SyncExfil       bool
	SyncArtifacts   bool
	SyncNetPolicies bool
}

type DRRuleName = string

type OrgConfig struct {
	DRRules map[DRRuleName]CoreDRRule `json:"rules" yaml:"rules"`
}

type OrgSyncOperation struct {
	ElementType string `json:"type"`
	ElementName string `json:"name"`
	IsAdded     bool   `json:"is_added"`
	IsRemoved   bool   `json:"is_removed"`
}

func (org Organization) SyncFetch(options SyncOptions) (OrgConfig, error) {
	// TODO
	return OrgConfig{}, nil
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
		// TODO
	}
	if options.SyncDRRules {
		newOps, err := org.syncDRRules(who, conf.DRRules, options)
		ops = append(ops, newOps...)
		if err != nil {
			return ops, fmt.Errorf("dr-rules: %v", err)
		}
	}
	if options.SyncFPRules {
		// TODO
	}
	if options.SyncOutputs {
		// TODO
	}
	if options.SyncIntegrity {
		// TODO
	}
	if options.SyncArtifacts {
		// TODO
	}
	if options.SyncExfil {
		// TODO
	}
	if options.SyncNetPolicies {
		// TODO
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
				ops = append(ops, OrgSyncOperation{ElementType: "dr-rule", ElementName: ruleName})
				// Nothing to do, move on.
				continue
			}
			// If this is a DryRun, just report the op and move on.
			if options.IsDryRun {
				ops = append(ops, OrgSyncOperation{ElementType: "dr-rule", ElementName: ruleName, IsAdded: true})
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
				if err := org.DRDelRule(ruleName, WithNamespace(existingNs)); err != nil {
					return ops, fmt.Errorf("DRDelRule %s: %v", ruleName, err)
				}
			}
		}
		if options.IsDryRun {
			ops = append(ops, OrgSyncOperation{ElementType: "dr-rule", ElementName: ruleName, IsAdded: true})
			continue
		}
		if err := org.DRRuleAdd(ruleName, rule.Detect, rule.Response, NewDRRuleOptions{
			IsReplace: true,
			Namespace: rule.Namespace,
			IsEnabled: true,
		}); err != nil {
			return ops, fmt.Errorf("DRRuleAdd %s: %v", ruleName, err)
		}
		ops = append(ops, OrgSyncOperation{ElementType: "dr-rule", ElementName: ruleName, IsAdded: true})
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
			ops = append(ops, OrgSyncOperation{ElementType: "dr-rule", ElementName: ruleName, IsRemoved: true})
			continue
		}
		if err := org.DRDelRule(ruleName, WithNamespace(rule.Namespace)); err != nil {
			return ops, fmt.Errorf("DRDelRule %s: %v", ruleName, err)
		}
		ops = append(ops, OrgSyncOperation{ElementType: "dr-rule", ElementName: ruleName, IsRemoved: true})
	}

	return ops, nil
}
