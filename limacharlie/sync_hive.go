package limacharlie

import (
	"encoding/json"
	"sync"
)

func (org Organization) syncFetchHive(syncHiveOpts map[string]bool) (orgSyncHives, error) {
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
		for hiveName, _ := range syncHiveOpts {
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

func (org Organization) syncHive(hiveConfigData orgSyncHives, opts SyncOptions) ([]OrgSyncOperation, error) {
	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, err
	}

	var orgOps []OrgSyncOperation
	for hiveName, newConfigData := range hiveConfigData {
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
					PartitionKey: orgInfo.OID},
					newConfigData[hiveKey])
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
			return orgOps, nil
		}

		// now that keys have been added or updated for this particular
		// identify what keys should be removed
		for k, _ := range currentConfigData {
			if _, ok := newConfigData[k]; !ok {
				op := OrgSyncOperation{
					ElementType: OrgSyncOperationElementType.Hives,
					ElementName: hiveName + "/" + k,
					IsAdded:     false,
					IsRemoved:   true,
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

func (org Organization) fetchHiveConfigData(args HiveArgs) (HiveConfigData, error) {
	hiveClient := NewHiveClient(&org)

	dataSet, err := hiveClient.List(args)
	if err != nil {
		return nil, err
	}

	currentHiveDataConfig := map[string]HiveData{}
	for k, v := range dataSet {
		currentHiveDataConfig[k] = HiveData{
			Data: v.Data,
			UsrMtd: UsrMtd{
				Enabled: v.UsrMtd.Enabled,
				Expiry:  v.UsrMtd.Expiry,
				Tags:    v.UsrMtd.Tags,
			},
		}
	}
	return currentHiveDataConfig, nil
}

func (org Organization) updateHiveConfigData(ha HiveArgs, hd HiveData) error {
	hiveClient := NewHiveClient(&org)

	data, err := json.Marshal(hd.Data)
	if err != nil {
		return err
	}
	enabled := hd.UsrMtd.Enabled
	expiry := hd.UsrMtd.Expiry
	Tags := hd.UsrMtd.Tags
	args := HiveArgs{
		Key:          ha.Key,
		PartitionKey: ha.PartitionKey,
		HiveName:     ha.HiveName,
		Data:         &data,
		Enabled:      &enabled,
		Expiry:       &expiry,
		Tags:         &Tags,
	}

	_, err = hiveClient.Update(args) // run actual update call
	if err != nil {
		return err
	}
	return nil
}

func (org Organization) addHiveConfigData(ha HiveArgs, hd HiveData) error {
	hiveClient := NewHiveClient(&org)

	mData, err := json.Marshal(hd.Data)
	if err != nil {
		return err
	}

	enabled := hd.UsrMtd.Enabled
	expiry := hd.UsrMtd.Expiry
	Tags := hd.UsrMtd.Tags
	args := HiveArgs{
		Key:          ha.Key,
		PartitionKey: ha.PartitionKey,
		HiveName:     ha.HiveName,
		Data:         &mData,
		Enabled:      &enabled,
		Expiry:       &expiry,
		Tags:         &Tags,
	}

	_, err = hiveClient.Add(args)
	if err != nil {
		return err
	}
	return nil
}

func (org Organization) removeHiveConfigData(args HiveArgs) error {
	hiveClient := NewHiveClient(&org)

	_, err := hiveClient.Remove(args, false)
	if err != nil {
		return err
	}

	return nil
}
