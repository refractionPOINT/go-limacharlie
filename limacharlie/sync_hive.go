package limacharlie

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
)

type HiveConfig struct {
	Version int            `json:"version" yaml:"version"`
	Data    HiveConfigData `json:"data,omitempty" yaml:"data,omitempty"`
}

type HiveSyncOptions struct {
	IsDryRun      bool            `json:"is_dry_run"`
	IsForce       bool            `json:"is_force"`
	HiveName      string          `json:"hive_name"`
	OID           string          `json:"oid"`
	IncludeLoader IncludeLoaderCB `json:"-"`
}

var OrgSyncOpsHiveType = struct {
	Data string
}{
	Data: "data",
}

//type HiveSyncData struct {
//	Data   map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`
//	UsrMtd UsrMtdConfig           `json:"usr_mtd,omitempty" yaml:"usr_mtd,omitempty"`
//}

//type UsrMtdConfig struct {
//	Enabled *bool     `json:"enabled" yaml:"enabled"`
//	Expiry  *int64    `json:"expiry" yaml:"expiry"`
//	Tags    *[]string `json:"tags" yaml:"tags"`
//}

func (org Organization) HiveSyncPush(newConfig HiveConfig, opts HiveSyncOptions) ([]OrgSyncOperation, error) {

	if opts.HiveName == "" {
		return nil, errors.New("missing hive name")
	}

	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, err
	}

	opts.OID = orgInfo.OID // let set oid
	curConfig, err := org.fetchHiveConfigData(opts)
	if err != nil {
		return nil, err
	}
	return org.hiveSyncData(newConfig.Data, curConfig, opts)
}

func (org Organization) HiveSyncPushFromFiles(config string, opts HiveSyncOptions, includes []string) ([]OrgSyncOperation, error) {

	if opts.HiveName == "" {
		return nil, errors.New("missing hive name")
	}

	orgInfo, err := org.GetInfo()
	if err != nil {
		return nil, err
	}
	opts.OID = orgInfo.OID // lets ensure we are always using correct OID

	if opts.IncludeLoader == nil {
		opts.IncludeLoader = localFileIncludeLoader
	}

	// Start with the base unmarshal.
	hiveConfig := HiveConfig{}
	err = yaml.Unmarshal([]byte(config), &hiveConfig)
	if err != nil {
		return nil, err
	}

	return org.HiveSyncPush(hiveConfig, opts)
}

func (org Organization) hiveSyncData(newConfigData, currentConfigData HiveConfigData, opts HiveSyncOptions) ([]OrgSyncOperation, error) {
	var orgOps []OrgSyncOperation

	// now check if we need to update or add new data
	for k, ncd := range newConfigData {
		// if key does not exist in current config data
		// new data needs to be added
		if _, ok := currentConfigData[k]; !ok {
			data, err := json.Marshal(newConfigData[k].Data)
			if err != nil {
				return orgOps, err
			}
			enabled := newConfigData[k].UsrMtd.Enabled
			expiry := newConfigData[k].UsrMtd.Expiry
			Tags := newConfigData[k].UsrMtd.Tags
			args := HiveArgs{
				Key:          k,
				PartitionKey: opts.OID,
				HiveName:     opts.HiveName,
				Data:         &data,
				Enabled:      &enabled,
				Expiry:       &expiry,
				Tags:         &Tags,
			}
			err = org.addHiveConfigData(args, opts.IsDryRun, &orgOps)
			if err != nil {
				return orgOps, err
			}
		} else {
			// if new config data exists in current config
			// check to see if data is equal if not update
			curData := currentConfigData[k]
			equals, err := ncd.Equals(curData)
			if err != nil {
				return orgOps, err
			}
			if equals {
				orgOps = append(orgOps, OrgSyncOperation{
					ElementType: OrgSyncOpsHiveType.Data,
					ElementName: k,
					IsAdded:     false,
					IsRemoved:   false,
				})
			} else { // not equal run hive update
				data, err := json.Marshal(newConfigData[k].Data)
				if err != nil {
					return orgOps, err
				}
				enabled := newConfigData[k].UsrMtd.Enabled
				expiry := newConfigData[k].UsrMtd.Expiry
				Tags := newConfigData[k].UsrMtd.Tags
				args := HiveArgs{
					Key:          k,
					PartitionKey: opts.OID,
					HiveName:     opts.HiveName,
					Data:         &data,
					Enabled:      &enabled,
					Expiry:       &expiry,
					Tags:         &Tags,
				}
				err = org.updateHiveConfigData(args, opts.IsDryRun, &orgOps)
				if err != nil {
					return orgOps, err
				}
			}
		}
	}

	// only remove values from org if IsForce is set
	if !opts.IsForce {
		return orgOps, nil
	}

	// now that keys have been added or updated
	// identify what keys should be removed
	for k, _ := range currentConfigData {
		if _, ok := newConfigData[k]; !ok {
			args := HiveArgs{Key: k, PartitionKey: opts.OID, HiveName: opts.HiveName}
			err := org.removeHiveConfigData(args, opts.IsDryRun, &orgOps)
			if err != nil {
				return orgOps, err
			}
		}
	}

	return orgOps, nil
}

func (org Organization) fetchHiveConfigData(opts HiveSyncOptions) (HiveConfigData, error) {
	hiveClient := NewHiveClient(&org)

	args := HiveArgs{HiveName: opts.HiveName, PartitionKey: opts.OID}
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

func (org Organization) updateHiveConfigData(args HiveArgs, isDryRun bool, orgOps *[]OrgSyncOperation) error {
	hiveClient := NewHiveClient(&org)

	fmt.Println("I would be updating key here key ", args.Key)
	fmt.Printf("this is args in update %+v \n ", args)
	op := OrgSyncOperation{
		ElementType: OrgSyncOpsHiveType.Data,
		ElementName: args.Key,
		IsAdded:     true,
		IsRemoved:   false,
	}
	if isDryRun {
		*orgOps = append(*orgOps, op)
		return nil
	}

	_, err := hiveClient.Update(args) // run actual update call
	if err != nil {
		return err
	}

	*orgOps = append(*orgOps, op)
	return nil
}

func (org Organization) addHiveConfigData(args HiveArgs, isDryRun bool, orgOps *[]OrgSyncOperation) error {
	hiveClient := NewHiveClient(&org)

	fmt.Println("I would be adding data key ", args.Key)
	fmt.Printf("this is args %+v \n", args)
	op := OrgSyncOperation{
		ElementType: OrgSyncOpsHiveType.Data,
		ElementName: args.Key,
		IsAdded:     true,
		IsRemoved:   false,
	}
	if isDryRun {
		*orgOps = append(*orgOps, op)
		return nil // ensure you return dry run
	}

	_, err := hiveClient.Add(args)
	if err != nil {
		return err
	}

	*orgOps = append(*orgOps, op)
	return nil
}

func (org Organization) removeHiveConfigData(args HiveArgs, isDryRun bool, orgOps *[]OrgSyncOperation) error {
	hiveClient := NewHiveClient(&org)

	fmt.Println("I would be removing key here ", args.Key)
	op := OrgSyncOperation{
		ElementType: OrgSyncOpsHiveType.Data,
		ElementName: args.Key,
		IsAdded:     false,
		IsRemoved:   true,
	}
	if isDryRun {
		*orgOps = append(*orgOps, op)
		return nil
	}

	_, err := hiveClient.Remove(args, false)
	if err != nil {
		return err
	}

	*orgOps = append(*orgOps, op)
	return nil
}

func (hsd *HiveData) Equals(cData HiveData) (bool, error) {
	currentData, err := json.Marshal(hsd.Data)
	if err != nil {
		return false, err
	}

	newData, err := json.Marshal(cData.Data)
	if err != nil {
		return false, err
	}
	if string(currentData) != string(newData) {
		return false, nil
	}

	curUsrMtd, err := json.Marshal(hsd.UsrMtd)
	if err != nil {
		return false, err
	}

	newUsrMTd, err := json.Marshal(cData.UsrMtd)
	if err != nil {
		return false, err
	}

	if string(curUsrMtd) != string(newUsrMTd) {
		return false, nil
	}

	return true, nil
}
