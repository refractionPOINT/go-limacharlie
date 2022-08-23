package limacharlie

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"path/filepath"
)

const (
	HiveConfigLatestVersion = 1
)

type HiveConfig struct {
	Version  int            `json:"version" yaml:"version"`
	Data     HiveConfigData `json:"data,omitempty" yaml:"data,omitempty"`
	Includes []string       `json:"-" yaml:"-"`
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

func (org Organization) HiveSyncPushFromFiles(rootConfigFile string, opts HiveSyncOptions, includes []string) ([]OrgSyncOperation, error) {

	if opts.IncludeLoader == nil {
		opts.IncludeLoader = localFileIncludeLoader
	}

	conf, err := loadHiveEffectiveConfig("", rootConfigFile, opts)
	if err != nil {
		return nil, err
	}

	return org.HiveSyncPush(conf, opts)
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

func (hc HiveConfig) Merge(config HiveConfig) HiveConfig {
	if hc.Data == nil && config.Data == nil {
		return HiveConfig{}
	}

	n := map[string]interface{}{}
	for k, v := range hc.Data {
		n[k] = v
	}
	for k, v := range config.Data {
		n[k] = v
	}

	return HiveConfig{}
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

func loadHiveConfWithOptions(parent string, configFile string, options HiveSyncOptions) (HiveConfig, error) {
	conf, err := options.IncludeLoader(parent, configFile)
	if err != nil {
		return HiveConfig{}, err
	}

	hiveConfig := HiveConfig{}
	if err := yaml.Unmarshal(conf, &hiveConfig); err != nil {
		return HiveConfig{}, err
	}

	if hiveConfig.Version <= 0 {
		return HiveConfig{}, fmt.Errorf("invalid version found (%s): %v", configFile, hiveConfig.Version)
	}
	if hiveConfig.Version > HiveConfigLatestVersion {
		return HiveConfig{}, fmt.Errorf("version not supported (%s): %v", configFile, hiveConfig.Version)
	}
	return hiveConfig, nil
}

func loadHiveEffectiveConfig(parent string, configFile string, opts HiveSyncOptions) (HiveConfig, error) {
	thisConfig, err := loadHiveConfWithOptions(parent, configFile, opts)
	if err != nil {
		return HiveConfig{}, err
	}

	includePath := filepath.Join(filepath.Dir(parent), configFile)
	for _, toInclude := range thisConfig.Includes {
		incConf, err := loadHiveEffectiveConfig(includePath, toInclude, opts)
		if err != nil {
			return HiveConfig{}, err
		}
		thisConfig = thisConfig.Merge(incConf)
	}
	return thisConfig, nil
}
