package limacharlie

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// syncHivePageSize is the page size used when reading hives for a sync. The
// server still bounds the byte size of each page.
const syncHivePageSize = 500

// serverManagedTagPrefixes are tags Hive writes onto a record itself. Sync
// leaves them out of what it reads, exports and compares, and never sends
// them: Hive stamps them again on every write. Without this, a record written
// without the tag (the normal way to write one) would never compare equal to
// what Hive stores, and every sync would rewrite it.
//
// mailsec_provider: Hive tags each connection "mailsec:provider:<provider>"
// from its provider field, for routing and the one-enabled-connection-per-
// provider check.
var serverManagedTagPrefixes = map[string][]string{
	"mailsec_provider": {"mailsec:provider:"},
}

// withoutServerManagedTags returns tags minus the ones Hive manages for hive.
// It never modifies tags in place.
func withoutServerManagedTags(hive string, tags []string) []string {
	prefixes := serverManagedTagPrefixes[hive]
	if len(prefixes) == 0 || len(tags) == 0 {
		return tags
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		managed := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(tag, prefix) {
				managed = true
				break
			}
		}
		if !managed {
			out = append(out, tag)
		}
	}
	return out
}

// orderedSyncHiveNames returns the hives of a configuration in the order
// syncHive applies them: KnownHives order first (which puts a product's
// provider records before the records that use them), then any other hive by
// name. Map order would make which hive fails first, and what was already
// applied when it does, differ from one run to the next.
func orderedSyncHiveNames(hives orgSyncHives) []string {
	names := make([]string, 0, len(hives))
	seen := make(map[string]struct{}, len(hives))
	for _, hive := range KnownHives {
		if _, ok := hives[hive]; ok {
			names = append(names, hive)
			seen[hive] = struct{}{}
		}
	}
	var rest []string
	for hive := range hives {
		if _, ok := seen[hive]; !ok {
			rest = append(rest, hive)
		}
	}
	sort.Strings(rest)
	return append(names, rest...)
}

type SyncHiveConfigData map[string]SyncHiveData
type SyncHiveData struct {
	// Apply-time version only; never serialized into Git.
	etag   string
	Data   map[string]interface{} `json:"data" yaml:"data,omitempty"`
	UsrMtd UsrMtd                 `json:"usr_mtd" yaml:"usr_mtd"`
}

// syncFetchHive reads every hive enabled in syncHiveOpts, concurrently.
//
// With skipUnauthorized false, the first failure (in hive name order) fails
// the fetch. With it true, a hive the credentials may not read (the API's
// UNAUTHORIZED error) is left out of the result entirely and reported in the
// returned list, sorted by hive name; any other failure is still fatal.
//
// A hive the API does not know (UNKNOWN_HIVE: the deployment has it disabled)
// holds nothing to back up, so it is always skipped, whatever skipUnauthorized
// says, and reported with reason SyncSkipReasonUnknownHive.
//
// Every fetch is waited for before returning, so no goroutine outlives the
// call and the outcome does not depend on which request failed first.
func (org *Organization) syncFetchHive(syncHiveOpts map[string]bool, skipUnauthorized bool) (orgSyncHives, []SyncSkippedHive, error) {
	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, nil, err
	}

	names := make([]string, 0, len(syncHiveOpts))
	for hiveName, enabled := range syncHiveOpts {
		if enabled {
			names = append(names, hiveName)
		}
	}
	sort.Strings(names)

	type fetchResult struct {
		data SyncHiveConfigData
		err  error
	}
	// One slot per hive, each written by exactly one goroutine, and read only
	// after wg.Wait().
	results := make([]fetchResult, len(names))
	var wg sync.WaitGroup
	for i, hiveName := range names {
		wg.Add(1)
		go func(i int, hive string) {
			defer wg.Done()
			data, err := org.fetchHiveConfigData(HiveArgs{HiveName: hive, PartitionKey: orgInfo.OID})
			results[i] = fetchResult{data: data, err: err}
		}(i, hiveName)
	}
	wg.Wait()

	hiveSync := orgSyncHives{}
	skipped := []SyncSkippedHive{}
	for i, hiveName := range names {
		res := results[i]
		if res.err != nil {
			if unknownHiveAPIError(res.err) {
				skipped = append(skipped, SyncSkippedHive{
					Hive:   hiveName,
					Reason: SyncSkipReasonUnknownHive,
					Error:  res.err.Error(),
				})
				continue
			}
			if skipUnauthorized {
				if permission, ok := unauthorizedAPIError(res.err); ok {
					skipped = append(skipped, SyncSkippedHive{
						Hive:       hiveName,
						Reason:     SyncSkipReasonUnauthorized,
						Permission: permission,
						Error:      res.err.Error(),
					})
					continue
				}
			}
			return nil, nil, res.err
		}
		hiveSync[hiveName] = res.data
	}
	return hiveSync, skipped, nil
}

func (org *Organization) syncHive(hiveConfigData orgSyncHives, opts SyncOptions) ([]OrgSyncOperation, error) {
	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, err
	}

	var orgOps []OrgSyncOperation
	for _, hiveName := range orderedSyncHiveNames(hiveConfigData) {
		newConfigData := hiveConfigData[hiveName]

		// Only sync hives that are specified.
		if opts.SyncHives == nil || !opts.SyncHives[hiveName] {
			continue
		}

		// grab current config data as to determine if update or add needs to be processed
		currentConfigData, err := org.fetchHiveConfigData(HiveArgs{
			HiveName:     hiveName,
			PartitionKey: orgInfo.OID,
		})
		if err != nil {
			return orgOps, err
		}

		// now check if we need to update or add new data for this particular hive
		for hiveKey, ncd := range newConfigData {
			// A tag Hive manages is never sent, so a configuration exported
			// before sync dropped it still compares equal.
			ncd.UsrMtd.Tags = withoutServerManagedTags(hiveName, ncd.UsrMtd.Tags)
			// if key does not exist in current config data
			// new data needs to be added
			if _, ok := currentConfigData[hiveKey]; !ok {
				op := OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Hives,
					ElementName: hiveName + "/" + hiveKey,
					IsAdded:     true,
					IsRemoved:   false,
				}
				if opts.IsDryRun {
					orgOps = append(orgOps, op)
					continue
				}
				ha := HiveArgs{Key: hiveKey, HiveName: hiveName, PartitionKey: orgInfo.OID}
				if isInvestigationSyncHive(hiveName) {
					err = org.writeInvestigationHive(ha, ncd, "")
				} else {
					err = org.addHiveConfigData(ha, ncd)
				}
				if err != nil {
					return orgOps, err
				}
				orgOps = append(orgOps, op)
			} else {
				// if new config data exists in current config
				// check to see if data is equal if not update
				curData := currentConfigData[hiveKey]
				// Preserve the established metadata-only update semantics.
				if isInvestigationSyncHive(hiveName) && len(ncd.Data) == 0 {
					ncd.Data = curData.Data
				}
				equals, err := ncd.Equals(curData)
				if err != nil {
					return orgOps, err
				}
				op := OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Hives,
					ElementName: hiveName + "/" + hiveKey,
					IsAdded:     false,
					IsRemoved:   false,
				}
				if equals {
					orgOps = append(orgOps, op)
				} else { // not equal run hive update
					if opts.IsDryRun {
						op.IsAdded = true
						orgOps = append(orgOps, op)
						continue
					}
					ha := HiveArgs{Key: hiveKey, HiveName: hiveName, PartitionKey: orgInfo.OID}
					if isInvestigationSyncHive(hiveName) {
						if curData.etag == "" {
							return orgOps, fmt.Errorf("%s/%s: missing native etag; reload before applying", hiveName, hiveKey)
						}
						err = org.writeInvestigationHive(ha, ncd, curData.etag)
					} else {
						err = org.updateHiveConfigData(ha, ncd)
					}
					if err != nil {
						return orgOps, err
					}
					op.IsAdded = true
					orgOps = append(orgOps, op)
				}
			}
		}

		// only remove values from org if IsForce is set
		if !opts.IsForce {
			continue
		}

		// now that keys have been added or updated for this hive
		// identify what keys should be removed
		for k, _ := range currentConfigData {
			if _, ok := newConfigData[k]; !ok {
				op := OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Hives,
					ElementName: hiveName + "/" + k,
					IsAdded:     false,
					IsRemoved:   true,
				}
				// If tags are passed ensure all tags match before removing
				if len(opts.Tags) != 0 {
					if !slicesContainSameItems(currentConfigData[k].UsrMtd.Tags, opts.Tags) {
						continue // tags do not match do not remove
					}
				}

				if opts.IsDryRun {
					orgOps = append(orgOps, op)
					continue
				}

				err := org.removeHiveConfigData(HiveArgs{Key: k, PartitionKey: orgInfo.OID, HiveName: hiveName})
				if err != nil {
					return orgOps, err
				}
				orgOps = append(orgOps, op)
			}
		}
	}
	return orgOps, nil
}

func (org *Organization) fetchHiveConfigData(args HiveArgs) (SyncHiveConfigData, error) {
	hiveClient := NewHiveClient(org)

	// Walk the hive page by page: a big hive is never requested in one
	// response (which the server may refuse), and only the fields the sync
	// compares are kept from each page.
	currentHiveDataConfig := map[string]SyncHiveData{}
	err := hiveClient.ListPages(args, ListPageOptions{Limit: syncHivePageSize}, func(page HiveConfigData) error {
		for k, v := range page {
			currentHiveDataConfig[k] = SyncHiveData{
				Data: v.Data,
				UsrMtd: UsrMtd{
					Enabled: v.UsrMtd.Enabled,
					Expiry:  v.UsrMtd.Expiry,
					Tags:    withoutServerManagedTags(args.HiveName, v.UsrMtd.Tags),
					Comment: v.UsrMtd.Comment,
				},
			}
			if isInvestigationSyncHive(args.HiveName) {
				rec := currentHiveDataConfig[k]
				rec.etag = v.SysMtd.Etag
				currentHiveDataConfig[k] = rec
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return currentHiveDataConfig, nil
}

func (org *Organization) updateHiveConfigData(ha HiveArgs, hd SyncHiveData) error {
	hiveClient := NewHiveClient(org)

	err := encodeDecodeHiveData(&hd.Data)
	if err != nil {
		return err
	}

	enabled := hd.UsrMtd.Enabled
	expiry := hd.UsrMtd.Expiry
	tags := hd.UsrMtd.Tags
	comment := hd.UsrMtd.Comment
	args := HiveArgs{
		Key:          ha.Key,
		PartitionKey: ha.PartitionKey,
		HiveName:     ha.HiveName,
		Data:         hd.Data,
		Enabled:      &enabled,
		Expiry:       &expiry,
		Tags:         tags,
		Comment:      &comment,
	}

	_, err = hiveClient.Update(args) // run actual update call
	if err != nil {
		return err
	}
	return nil
}

func (org *Organization) addHiveConfigData(ha HiveArgs, hd SyncHiveData) error {
	hiveClient := NewHiveClient(org)

	err := encodeDecodeHiveData(&hd.Data)
	if err != nil {
		return err
	}

	enabled := hd.UsrMtd.Enabled
	expiry := hd.UsrMtd.Expiry
	tags := hd.UsrMtd.Tags
	comment := hd.UsrMtd.Comment
	args := HiveArgs{
		Key:          ha.Key,
		PartitionKey: ha.PartitionKey,
		HiveName:     ha.HiveName,
		Data:         hd.Data,
		Enabled:      &enabled,
		Expiry:       &expiry,
		Tags:         tags,
		Comment:      &comment,
	}

	_, err = hiveClient.Add(args)
	if err != nil {
		return err
	}
	return nil
}

func (org *Organization) removeHiveConfigData(args HiveArgs) error {
	hiveClient := NewHiveClient(org)

	_, err := hiveClient.Remove(args)
	if err != nil {
		return err
	}

	return nil
}

// encodeDecodeHiveData ensures that any passed hiveData is properly
// encoded using YamlV3 to handle json type of map[interface {}]interface{}
func encodeDecodeHiveData(hd *map[string]interface{}) error {
	out, err := yaml.Marshal(hd)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(out, &hd)
}

func (hsd *SyncHiveData) Equals(cData SyncHiveData) (bool, error) {
	err := encodeDecodeHiveData(&hsd.Data)
	if err != nil {
		return false, err
	}

	newData, err := json.Marshal(hsd.Data)
	if err != nil {
		return false, err
	}
	if string(newData) == "{}" || string(newData) == "null" {
		newData = nil
	}

	currentData, err := json.Marshal(cData.Data)
	if err != nil {
		return false, err
	}
	if string(currentData) == "{}" || string(currentData) == "null" {
		currentData = nil
	}
	if string(currentData) != string(newData) {
		return false, nil
	}

	if len(hsd.UsrMtd.Tags) == 0 {
		hsd.UsrMtd.Tags = nil
	}
	newUsrMTd, err := json.Marshal(hsd.UsrMtd)
	if err != nil {
		return false, err
	}

	if len(cData.UsrMtd.Tags) == 0 {
		cData.UsrMtd.Tags = nil
	}
	curUsrMtd, err := json.Marshal(cData.UsrMtd)
	if err != nil {
		return false, err
	}

	if string(curUsrMtd) != string(newUsrMTd) {
		return false, nil
	}

	return true, nil
}

func (hcd HiveConfigData) AsSyncConfigData() SyncHiveConfigData {
	out := SyncHiveConfigData{}
	for k, v := range hcd {
		out[k] = v.AsSyncData()
	}
	return out
}

func (hd HiveData) AsSyncData() SyncHiveData {
	return SyncHiveData{
		Data:   hd.Data,
		UsrMtd: hd.UsrMtd,
	}
}

func isInvestigationSyncHive(hive string) bool {
	return hive == "investigation_policy" || hive == "investigation_plan"
}

// writeInvestigationHive uses the version from the listing used to compute
// the diff. Native Hive only checks nonempty etags on existing records: an
// empty etag on create follows UpdateTx's convention, not create-only semantics.
func (org *Organization) writeInvestigationHive(ha HiveArgs, hd SyncHiveData, etag string) error {
	if err := encodeDecodeHiveData(&hd.Data); err != nil {
		return err
	}
	_, err := NewHiveClient(org).Add(HiveArgs{
		Key: ha.Key, PartitionKey: ha.PartitionKey, HiveName: ha.HiveName,
		Data: hd.Data, Enabled: &hd.UsrMtd.Enabled, Expiry: &hd.UsrMtd.Expiry,
		Tags: hd.UsrMtd.Tags, Comment: &hd.UsrMtd.Comment, ETag: &etag,
	})
	// Add may replay a request after a gateway timeout even when the first
	// write committed. Reconcile a mismatch by reading, never by rewriting
	// with a newer version. Matching data AND metadata already satisfy sync.
	if err != nil && strings.Contains(err.Error(), "ETAG_MISMATCH") {
		stored, readErr := NewHiveClient(org).Get(ha)
		if readErr == nil {
			if equal, compareErr := hd.Equals(stored.AsSyncData()); compareErr == nil && equal {
				return nil
			}
		}
	}
	return err
}
