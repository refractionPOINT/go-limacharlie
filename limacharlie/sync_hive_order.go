package limacharlie

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// The records of one hive are applied in a fixed order, never in Go map order.
// With map order, which records were applied before a refused write differed
// from one run to the next, and a push that changed a hive's records together
// with records that depend on them could be refused or accepted by chance.
//
// The order is: by record name for every hive. For the investigation_plan hive
// the records of `kind: check` come first (a check that calls another check
// comes after the check it calls, when the push holds both), then every other
// record, each group by name.

const (
	investigationPlanHive = "investigation_plan"
	// investigationCheckUse is how a template calls a check: `use: template:<name>`,
	// and `on_lease_end.check: template:<name>`.
	investigationCheckUse = "template:"
)

// orderedSyncKeys returns the keys of data in the order syncHive writes them.
func orderedSyncKeys(hive string, data SyncHiveConfigData) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if hive != investigationPlanHive {
		return keys
	}
	return checksFirst(keys, func(k string) map[string]interface{} { return data[k].Data })
}

// orderedStaleKeys returns the keys of current that desired does not hold, in
// the order they are removed: the reverse of the write order, so what calls a
// record is removed before the record it calls.
func orderedStaleKeys(hive string, current, desired SyncHiveConfigData) []string {
	stale := SyncHiveConfigData{}
	for k, v := range current {
		if _, ok := desired[k]; !ok {
			stale[k] = v
		}
	}
	keys := orderedSyncKeys(hive, stale)
	if hive == investigationPlanHive {
		for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
			keys[i], keys[j] = keys[j], keys[i]
		}
	}
	return keys
}

// checksFirst orders sorted keys: the checks (called before caller), then the
// other records in name order.
func checksFirst(sorted []string, dataOf func(string) map[string]interface{}) []string {
	calls := map[string][]string{}
	var checks, rest []string
	for _, k := range sorted {
		isCheck, called := investigationRecordShape(dataOf(k))
		if !isCheck {
			rest = append(rest, k)
			continue
		}
		checks = append(checks, k)
		calls[k] = called
	}
	return append(callsFirst(checks, calls), rest...)
}

// callsFirst orders names (sorted) so a name comes after every name it calls
// that is also in names. Among the names that are ready the smallest goes
// first; names that wait on each other (a cycle) are released in name order.
func callsFirst(names []string, calls map[string][]string) []string {
	inSet := make(map[string]bool, len(names))
	for _, n := range names {
		inSet[n] = true
	}
	// waiting[n]: the names n still waits on; callers[c]: who waits on c.
	waiting := make(map[string]map[string]bool, len(names))
	callers := map[string][]string{}
	for _, n := range names {
		waiting[n] = map[string]bool{}
		for _, c := range calls[n] {
			if c == n || !inSet[c] || waiting[n][c] {
				continue
			}
			waiting[n][c] = true
			callers[c] = append(callers[c], n)
		}
	}
	out := make([]string, 0, len(names))
	done := make(map[string]bool, len(names))
	release := func(n string) {
		done[n] = true
		out = append(out, n)
		for _, caller := range callers[n] {
			delete(waiting[caller], n)
		}
	}
	for len(out) < len(names) {
		next := ""
		for _, n := range names { // names is sorted: the first ready one is the smallest
			if !done[n] && len(waiting[n]) == 0 {
				next = n
				break
			}
		}
		if next == "" { // every name left waits on another one: a cycle
			for _, n := range names {
				if !done[n] {
					next = n
					break
				}
			}
		}
		release(next)
	}
	return out
}

// investigationRecordShape reads an investigation_plan record's template
// source (data.yaml): whether it is a check, and the checks it calls. A record
// whose source is missing or unreadable is not a check; the hive judges it.
func investigationRecordShape(data map[string]interface{}) (isCheck bool, called []string) {
	source, ok := data["yaml"].(string)
	if !ok || source == "" {
		return false, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(source), &doc); err != nil || len(doc.Content) == 0 {
		return false, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false, nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "kind" {
			isCheck = root.Content[i+1].Value == "check"
		}
	}
	if !isCheck {
		return false, nil
	}
	seen := map[string]bool{}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, val := n.Content[i].Value, n.Content[i+1]
				if (key == "use" || key == "check") && val.Kind == yaml.ScalarNode {
					if name, ok := strings.CutPrefix(val.Value, investigationCheckUse); ok && name != "" && !seen[name] {
						seen[name] = true
						called = append(called, name)
					}
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(root)
	sort.Strings(called)
	return true, called
}
