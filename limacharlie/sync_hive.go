package limacharlie

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
)

type HiveConfig struct {
	Version int            `json:"version" yaml:"version"`
	Data    HiveConfigData `json:"data,omitempty" yaml:"data,omitempty"`
}

type HiveConfigData map[string]HiveSyncData

type HiveSyncData struct {
	Data   map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`
	UsrMtd UsrMtdConfig           `json:"usr_mtd,omitempty" yaml:"usr_mtd,omitempty"`
}

type UsrMtdConfig struct {
	Enabled *bool     `json:"enabled" yaml:"enabled"`
	Expiry  *int64    `json:"expiry" yaml:"expiry"`
	Tags    *[]string `json:"tags" yaml:"tags"`
}

func (org Organization) HiveSyncPush(newConfig HiveConfig, args HiveArgs, isDryRun bool) ([]OrgSyncOperation, error) {
	curConfig, err := org.fetchHiveConfigData(args)
	if err != nil {
		return nil, err
	}

	return org.hiveSyncData(newConfig.Data, curConfig, args, isDryRun)
}

func (org Organization) HiveSyncPushFromFiles(config string, args HiveArgs, isDryRun bool, cb IncludeLoaderCB, includes []string) ([]OrgSyncOperation, error) {

	if cb == nil {
		cb = localFileIncludeLoader
	}

	// Start with the base unmarshal.
	hiveConfig := HiveConfig{}
	err := yaml.Unmarshal([]byte(config), &hiveConfig)
	if err != nil {
		fmt.Println("failed to parse yaml ", err)
	}

	return org.HiveSyncPush(hiveConfig, args, isDryRun)
}

func (org Organization) hiveSyncData(newConfigData, currentConfigData HiveConfigData, args HiveArgs, isDryRun bool) ([]OrgSyncOperation, error) {

	var orgOps []OrgSyncOperation

	// now check if we need to update or add new data
	for k, ncd := range newConfigData {
		// if key does not exist in current config data
		// new data needs to be added
		if _, ok := currentConfigData[k]; !ok {
			args.Key = k
			data, err := json.Marshal(newConfigData[k].Data)
			if err != nil {
				return orgOps, err
			}
			args.Key = k
			args.Data = &data
			args.Enabled = newConfigData[k].UsrMtd.Enabled
			args.Expiry = newConfigData[k].UsrMtd.Expiry
			args.Tags = newConfigData[k].UsrMtd.Tags
			fmt.Println("I would be adding data key ", k)
			fmt.Printf("this is args %+v \n", args)
			err = org.addHiveConfigData(args)
			if err != nil {
				return orgOps, err
			}
			continue
		}

		// if new config data exists in current config
		// then check to see if data is equal if not update
		curData := currentConfigData[k]
		equals, err := ncd.Equals(curData)
		if err != nil {
			return nil, nil
		}

		if !equals { // not equal run hive data update
			data, err := json.Marshal(newConfigData[k].Data)
			if err != nil {
				return orgOps, err
			}
			args.Key = k
			args.Data = &data
			args.Enabled = newConfigData[k].UsrMtd.Enabled
			args.Expiry = newConfigData[k].UsrMtd.Expiry
			args.Tags = newConfigData[k].UsrMtd.Tags
			fmt.Println("I would be updating key here key ", k)
			fmt.Printf("this is args in update %+v \n ", args)
			err = org.updateHiveConfigData(args)
			if err != nil {
				return orgOps, err
			}
		}
	}

	// identify what keys should be removed
	// as keys do not prior data does not exists in current
	removeKeys := make([]string, 0)
	for k, _ := range currentConfigData {
		if _, ok := newConfigData[k]; !ok {
			removeKeys = append(removeKeys, k)
		}
	}

	for _, key := range removeKeys { // perform actual remove
		args.Key = key
		fmt.Println("I would be removing key here ", key)
		org.removeHiveConfigData(args)
	}

	return orgOps, nil
}

func (org *Organization) fetchHiveConfigData(args HiveArgs) (HiveConfigData, error) {
	hiveClient := NewHiveClient(org)

	dataSet, err := hiveClient.List(args)
	if err != nil {
		fmt.Println("error inside of hiveClient list ", err)
		return nil, err
	}

	currentHiveDataConfig := map[string]HiveSyncData{}
	for k, v := range dataSet {
		currentHiveDataConfig[k] = HiveSyncData{
			Data: v.Data,
			UsrMtd: UsrMtdConfig{
				Enabled: &v.UsrMtd.Enabled,
				Expiry:  &v.UsrMtd.Expiry,
				Tags:    &v.UsrMtd.Tags,
			},
		}
	}

	return currentHiveDataConfig, nil
}

func (org *Organization) updateHiveConfigData(args HiveArgs) error {
	hiveClient := NewHiveClient(org)

	_, err := hiveClient.Update(args)
	if err != nil {
		return err
	}

	return nil
}

func (org *Organization) addHiveConfigData(args HiveArgs) error {
	hiveClient := NewHiveClient(org)

	_, err := hiveClient.Add(args)
	if err != nil {
		return err
	}

	return nil
}

func (org *Organization) removeHiveConfigData(args HiveArgs) error {
	hiveClient := NewHiveClient(org)

	_, err := hiveClient.Remove(args, false)
	if err != nil {
		return err
	}

	return nil
}

func (hsd *HiveSyncData) Equals(cData HiveSyncData) (bool, error) {
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
