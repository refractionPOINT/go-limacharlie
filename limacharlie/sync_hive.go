package limacharlie

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"sync"

	"gopkg.in/yaml.v3"
)

type SyncHiveConfigData map[string]SyncHiveData
type SyncHiveData struct {
	Data   map[string]interface{} `json:"data" yaml:"data,omitempty"`
	UsrMtd UsrMtd                 `json:"usr_mtd" yaml:"usr_mtd"`
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
	var mOps sync.Mutex
	eg := errgroup.Group{}
	eg.SetLimit(10)
	for hiveName, newConfigData := range hiveConfigData {
		org.logger.Info(fmt.Sprintf("Syncing hive: %s %s", hiveName, org.GetOID()))

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
			eg.Go(func() error {
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
						mOps.Lock()
						orgOps = append(orgOps, op)
						mOps.Unlock()
						return nil
					}
					err = org.addHiveConfigData(HiveArgs{
						Key:          hiveKey,
						HiveName:     hiveName,
						PartitionKey: orgInfo.OID,
					}, newConfigData[hiveKey])
					if err != nil {
						return err
					}
					mOps.Lock()
					orgOps = append(orgOps, op)
					mOps.Unlock()
				} else {
					// if new config data exists in current config
					// check to see if data is equal if not update
					curData := currentConfigData[hiveKey]
					equals, err := ncd.Equals(curData)
					if err != nil {
						return err
					}
					op := OrgSyncOperation{
						ElementType: OrgSyncOperationElementType.Hives,
						ElementName: hiveName + "/" + hiveKey,
						IsAdded:     false,
						IsRemoved:   false,
					}
					if equals {
						mOps.Lock()
						orgOps = append(orgOps, op)
						mOps.Unlock()
					} else { // not equal run hive update
						if opts.IsDryRun {
							op.IsAdded = true
							mOps.Lock()
							orgOps = append(orgOps, op)
							mOps.Unlock()
							return nil
						}
						err = org.updateHiveConfigData(HiveArgs{
							Key:          hiveKey,
							HiveName:     hiveName,
							PartitionKey: orgInfo.OID},
							ncd)
						if err != nil {
							return err
						}
						op.IsAdded = true
						mOps.Lock()
						orgOps = append(orgOps, op)
						mOps.Unlock()
					}
				}
				return nil
			})
		}

		// only remove values from org if IsForce is set
		if !opts.IsForce {
			continue
		}

		// now that keys have been added or updated for this hive
		// identify what keys should be removed
		for k := range currentConfigData {
			eg.Go(func() error {
				if _, ok := newConfigData[k]; !ok {
					op := OrgSyncOperation{
						ElementType: OrgSyncOperationElementType.Hives,
						ElementName: hiveName + "/" + k,
						IsAdded:     false,
						IsRemoved:   true,
					}
					// If tags are passed ensure all tags match before removing
					if opts.Tags != nil && len(opts.Tags) != 0 {
						if !slicesContainSameItems(currentConfigData[k].UsrMtd.Tags, opts.Tags) {
							return nil // tags do not match do not remove
						}
					}

					if opts.IsDryRun {
						mOps.Lock()
						orgOps = append(orgOps, op)
						mOps.Unlock()
						return nil
					}

					err := org.removeHiveConfigData(HiveArgs{Key: k, PartitionKey: orgInfo.OID, HiveName: hiveName})
					if err != nil {
						return err
					}
					mOps.Lock()
					orgOps = append(orgOps, op)
					mOps.Unlock()
				}
				return nil
			})
		}
	}

	if err := eg.Wait(); err != nil {
		return orgOps, err
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
