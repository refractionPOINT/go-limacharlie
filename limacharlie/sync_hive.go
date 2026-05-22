package limacharlie

import (
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"
)

type SyncHiveConfigData map[string]SyncHiveData

// SyncHiveData carries one hive record's data and metadata through the sync
// flow. The presence field is populated by UnmarshalYAML and records which
// usr_mtd keys were authored in the source YAML, so that the sync write path
// can send only those fields on the wire (and so Equals only diffs them).
// Unauthored fields are preserved by HiveClient.Update via a client-side
// merge against the existing record's metadata, gated by the existing etag
// CAS to keep concurrent writers safe.
//
// presenceUsed distinguishes "presence info is reliable" (YAML decode, or
// AsSyncData from a fetched record) from "no presence info available, fall
// back to legacy semantics" (a struct literal built by an external caller).
// Direct callers that want presence-aware semantics should populate via
// AsSyncData or the SyncHiveData methods.
type SyncHiveData struct {
	Data         map[string]interface{} `json:"data" yaml:"data,omitempty"`
	UsrMtd       UsrMtd                 `json:"usr_mtd" yaml:"usr_mtd"`
	presence     usrMtdPresence         `json:"-" yaml:"-"`
	presenceUsed bool                   `json:"-" yaml:"-"`
}

// usrMtdPresence tracks which usr_mtd keys the YAML actually authored. Nil
// pointer means the YAML omitted the field; non-nil means the YAML set it
// (including to its zero value).
type usrMtdPresence struct {
	Enabled *bool
	Expiry  *int64
	Tags    *[]string
	Comment *string
}

// UnmarshalYAML decodes a SyncHiveData while recording which usr_mtd keys
// were present in the source YAML. The decode is two-stage: first into a
// map keyed by yaml.Node so key presence is observable, then into typed
// values for each present key. presenceUsed is set unconditionally on a
// successful decode so that "YAML with no usr_mtd block" is distinguishable
// from "struct literal with no presence info."
func (s *SyncHiveData) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Data   map[string]interface{} `yaml:"data"`
		UsrMtd map[string]yaml.Node   `yaml:"usr_mtd"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	s.Data = raw.Data
	s.presenceUsed = true
	if n, ok := raw.UsrMtd["enabled"]; ok {
		var v bool
		if err := n.Decode(&v); err != nil {
			return err
		}
		s.UsrMtd.Enabled = v
		s.presence.Enabled = &v
	}
	if n, ok := raw.UsrMtd["expiry"]; ok {
		var v int64
		if err := n.Decode(&v); err != nil {
			return err
		}
		s.UsrMtd.Expiry = v
		s.presence.Expiry = &v
	}
	if n, ok := raw.UsrMtd["tags"]; ok {
		var v []string
		if err := n.Decode(&v); err != nil {
			return err
		}
		s.UsrMtd.Tags = v
		s.presence.Tags = &v
	}
	if n, ok := raw.UsrMtd["comment"]; ok {
		var v string
		if err := n.Decode(&v); err != nil {
			return err
		}
		s.UsrMtd.Comment = v
		s.presence.Comment = &v
	}
	return nil
}

// resolvedPresence returns the effective presence for write/diff. When the
// caller did not populate presence (struct literal path), every field is
// treated as authored so existing callers keep their old "send all fields"
// semantics. When presence was populated (YAML or AsSyncData), it is
// returned as-is so unauthored fields stay nil and get merged server-side.
func (s *SyncHiveData) resolvedPresence() usrMtdPresence {
	if s.presenceUsed {
		return s.presence
	}
	enabled := s.UsrMtd.Enabled
	expiry := s.UsrMtd.Expiry
	tags := s.UsrMtd.Tags
	comment := s.UsrMtd.Comment
	return usrMtdPresence{
		Enabled: &enabled,
		Expiry:  &expiry,
		Tags:    &tags,
		Comment: &comment,
	}
}

func (org *Organization) syncFetchHive(syncHiveOpts map[string]bool) (orgSyncHives, error) {
	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, err
	}

	m := sync.Mutex{}
	var wg sync.WaitGroup
	waitCh := make(chan struct{})
	errCh := make(chan error)
	hiveSync := orgSyncHives{}
	go func() {
		for hiveName := range syncHiveOpts {
			if syncHiveOpts[hiveName] {
				wg.Add(1)
				go func(hive string) {
					defer wg.Done()
					hiveConfigData, err := org.fetchHiveConfigData(HiveArgs{HiveName: hive, PartitionKey: orgInfo.OID})
					if err != nil {
						errCh <- err
					}

					m.Lock()
					defer m.Unlock()
					hiveSync[hive] = hiveConfigData
				}(hiveName)
			}
		}

		wg.Wait()
		close(waitCh)
	}()

	// if all calls are successful then return sync data
	// if a sync op fails return right away
	select {
	case <-waitCh:
		return hiveSync, nil
	case err := <-errCh:
		return nil, err
	}
}

func (org *Organization) syncHive(hiveConfigData orgSyncHives, opts SyncOptions) ([]OrgSyncOperation, error) {
	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, err
	}

	var orgOps []OrgSyncOperation
	for hiveName, newConfigData := range hiveConfigData {

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
				err = org.addHiveConfigData(HiveArgs{
					Key:          hiveKey,
					HiveName:     hiveName,
					PartitionKey: orgInfo.OID,
				}, newConfigData[hiveKey])
				if err != nil {
					return orgOps, err
				}
				orgOps = append(orgOps, op)
			} else {
				// if new config data exists in current config
				// check to see if data is equal if not update
				curData := currentConfigData[hiveKey]
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
					err = org.updateHiveConfigData(HiveArgs{
						Key:          hiveKey,
						HiveName:     hiveName,
						PartitionKey: orgInfo.OID},
						ncd)
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

	dataSet, err := hiveClient.List(args)
	if err != nil {
		return nil, err
	}

	currentHiveDataConfig := map[string]SyncHiveData{}
	for k, v := range dataSet {
		currentHiveDataConfig[k] = SyncHiveData{
			Data: v.Data,
			UsrMtd: UsrMtd{
				Enabled: v.UsrMtd.Enabled,
				Expiry:  v.UsrMtd.Expiry,
				Tags:    v.UsrMtd.Tags,
				Comment: v.UsrMtd.Comment,
			},
		}
	}
	return currentHiveDataConfig, nil
}

// updateHiveConfigData forwards only the usr_mtd fields the YAML authored to
// HiveClient.Update. Fields the YAML omitted are passed as nil pointers so
// that the update path merges them against the existing record's metadata
// instead of overwriting them with Go zero values. Callers building
// SyncHiveData via struct literal (no presence info) keep the legacy "send
// every field" behavior through resolvedPresence.
func (org *Organization) updateHiveConfigData(ha HiveArgs, hd SyncHiveData) error {
	hiveClient := NewHiveClient(org)

	err := encodeDecodeHiveData(&hd.Data)
	if err != nil {
		return err
	}

	p := hd.resolvedPresence()
	args := HiveArgs{
		Key:          ha.Key,
		PartitionKey: ha.PartitionKey,
		HiveName:     ha.HiveName,
		Data:         hd.Data,
		Enabled:      p.Enabled,
		Expiry:       p.Expiry,
		Tags:         derefTags(p.Tags),
		Comment:      p.Comment,
	}

	_, err = hiveClient.Update(args) // run actual update call
	if err != nil {
		return err
	}
	return nil
}

// addHiveConfigData forwards only the usr_mtd fields the YAML authored to
// HiveClient.Add. New records default to enabled=true when YAML-loaded data
// did not specify enabled, since declaring a rule via IaC almost always
// implies the author wants it active; a record created in the disabled
// state with no explicit intent looks like a successful deploy but never
// fires. Struct-literal callers keep their explicit Enabled value through
// resolvedPresence, so this default only fires for YAML pushes.
func (org *Organization) addHiveConfigData(ha HiveArgs, hd SyncHiveData) error {
	hiveClient := NewHiveClient(org)

	err := encodeDecodeHiveData(&hd.Data)
	if err != nil {
		return err
	}

	p := hd.resolvedPresence()
	enabledPtr := p.Enabled
	if enabledPtr == nil {
		defaultEnabled := true
		enabledPtr = &defaultEnabled
	}

	args := HiveArgs{
		Key:          ha.Key,
		PartitionKey: ha.PartitionKey,
		HiveName:     ha.HiveName,
		Data:         hd.Data,
		Enabled:      enabledPtr,
		Expiry:       p.Expiry,
		Tags:         derefTags(p.Tags),
		Comment:      p.Comment,
	}

	_, err = hiveClient.Add(args)
	if err != nil {
		return err
	}
	return nil
}

// derefTags returns the slice pointed to by p, or nil if p is nil. The
// distinction matters: a nil pointer means "YAML did not author tags"
// (preserve existing); a non-nil pointer to an empty slice means "YAML
// authored an empty tags list" (clear existing).
func derefTags(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
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

// Equals reports whether the YAML-authored record (hsd) matches the current
// server-side record (cData) for the purposes of deciding whether a sync
// push needs to issue an UPDATE. Data is always compared. For usr_mtd, only
// the fields the caller authored are diffed: unauthored fields are equal by
// definition since the sync write path won't change them anyway (they
// merge through to the existing value). This prevents spurious UPDATE
// operations whose only effect would be to re-send the existing mtd, and
// it keeps the diff aligned with what actually gets written on the wire.
//
// For struct-literal callers (no presence info), resolvedPresence treats
// every field as authored, matching the legacy "compare everything"
// behavior so existing callers see no observable diff change.
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

	p := hsd.resolvedPresence()
	if p.Enabled != nil && *p.Enabled != cData.UsrMtd.Enabled {
		return false, nil
	}
	if p.Expiry != nil && *p.Expiry != cData.UsrMtd.Expiry {
		return false, nil
	}
	if p.Tags != nil {
		newTags := *p.Tags
		curTags := cData.UsrMtd.Tags
		// Normalise empty-vs-nil before comparing, mirroring the legacy
		// Equals behavior so `tags: []` and missing-tags-in-current compare
		// as equal when neither has any entries.
		if len(newTags) == 0 {
			newTags = nil
		}
		if len(curTags) == 0 {
			curTags = nil
		}
		if !tagsEqual(newTags, curTags) {
			return false, nil
		}
	}
	if p.Comment != nil && *p.Comment != cData.UsrMtd.Comment {
		return false, nil
	}

	return true, nil
}

// tagsEqual reports whether two tag slices contain the same elements in the
// same order. Order matters because the server preserves tag order on
// writes; treating reordered tags as equal would mask real diffs.
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (hcd HiveConfigData) AsSyncConfigData() SyncHiveConfigData {
	out := SyncHiveConfigData{}
	for k, v := range hcd {
		out[k] = v.AsSyncData()
	}
	return out
}

// AsSyncData converts a fetched HiveData to SyncHiveData with all usr_mtd
// fields marked as present. Server-fetched records have values for every
// metadata field, so a round-trip through SyncPush should treat each field
// as authored and write it back faithfully.
func (hd HiveData) AsSyncData() SyncHiveData {
	enabled := hd.UsrMtd.Enabled
	expiry := hd.UsrMtd.Expiry
	tags := hd.UsrMtd.Tags
	comment := hd.UsrMtd.Comment
	return SyncHiveData{
		Data:   hd.Data,
		UsrMtd: hd.UsrMtd,
		presence: usrMtdPresence{
			Enabled: &enabled,
			Expiry:  &expiry,
			Tags:    &tags,
			Comment: &comment,
		},
		presenceUsed: true,
	}
}
