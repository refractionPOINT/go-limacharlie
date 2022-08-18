package limacharlie

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
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
	Enabled bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Expiry  int64    `json:"expiry,omitempty" yaml:"expiry,omitempty"`
	Tags    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

func NewHiveConfig() *HiveConfig {
	return &HiveConfig{}
}

func (org Organization) HiveSyncPush(newConfig HiveConfig, args HiveArgs, isDryRun bool) ([]OrgSyncOperation, error) {
	curConfig, err := org.fetchHiveConfigData(args)
	if err != nil {
		return nil, err
	}

	return org.hiveSyncData(newConfig.Data, curConfig, args, isDryRun)
}

func (org Organization) hiveSyncData(newConfigData, currentConfigData HiveConfigData, args HiveArgs, isDryRun bool) ([]OrgSyncOperation, error) {

	var orgOps []OrgSyncOperation

	equals, err := newConfigData.Equals(&currentConfigData)
	if err != nil {
		return nil, err
	}

	if equals { // nothing to do config data is equal
		return orgOps, nil
	}

	// now check if we need to update or add new data
	for k, hsd := range newConfigData {
		if _, ok := currentConfigData[k]; !ok { // new data found run add
			err := org.addHiveConfigData(args)
			if err != nil {
				return orgOps, err
			}
			continue
		}

		// check if current data matches new config data
		curData := currentConfigData[k]
		equals, err := hsd.Equals(curData)
		if err != nil {
			return nil, nil
		}

		if !equals { // not equal run hive data update
			err := org.updateHiveConfigData(args)
			if err != nil {
				return orgOps, err
			}
		}
	}

	// identify what keys should be removed
	// as keys do not prior data does not exists in current
	removeKeys := make([]string, 0)
	if len(newConfigData) != len(currentConfigData) {
		for k, _ := range currentConfigData {
			if _, ok := newConfigData[k]; !ok {
				removeKeys = append(removeKeys, k)
			}
		}
	}

	for _, key := range removeKeys { // perform actual remove
		args.Key = key
		org.removeHiveConfigData(args)
	}

	return orgOps, nil
}

func (org Organization) HiveSyncPushFromFiles(config string, args HiveArgs, isDryRun bool) ([]OrgSyncOperation, error) {

	// Start with the base unmarshal.
	hiveConfig := HiveConfig{}
	err := yaml.Unmarshal([]byte(config), &hiveConfig)
	if err != nil {
		fmt.Println("failed to parse yaml ", err)
	}

	return org.HiveSyncPush(hiveConfig, args, isDryRun)
}

func (org *Organization) fetchHiveConfigData(args HiveArgs) (HiveConfigData, error) {
	hiveClient := NewHiveClient(org)

	dataSet, err := hiveClient.List(args)
	if err != nil {
		return nil, err
	}

	var currentHiveDataConfig map[string]HiveSyncData
	for k, v := range dataSet {
		currentHiveDataConfig[k] = HiveSyncData{
			Data: v.Data,
			UsrMtd: UsrMtdConfig{
				Enabled: v.UsrMtd.Enabled,
				Expiry:  v.UsrMtd.Expiry,
				Tags:    v.UsrMtd.Tags,
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

func (hcd *HiveConfigData) Equals(newConfig *HiveConfigData) (bool, error) {

	current, err := json.Marshal(hcd)
	if err != nil {
		return false, err
	}

	nConfig, err := json.Marshal(newConfig)
	if err != nil {
		return false, err
	}

	if string(current) != string(nConfig) {
		return false, nil
	}

	return true, nil
}

func (hsd *HiveSyncData) Equals(cData HiveSyncData) (bool, error) {

	currentData, err := json.Marshal(hsd.Data)
	if err != nil {
		fmt.Println("data one failed to marshal ", err)
	}

	newData, err := json.Marshal(cData.Data)
	if err != nil {
		fmt.Println("data two failed to marshal ", err)
	}

	if string(currentData) != string(newData) {
		return false, nil
	}

	curUsrMtd, err := json.Marshal(hsd.UsrMtd)
	if err != nil {
		return false, err
	}

	newUsrMTd, err := json.Marshal(cData.UsrMtd)

	if string(curUsrMtd) != string(newUsrMTd) {
		return false, nil
	}

	return true, nil
}
