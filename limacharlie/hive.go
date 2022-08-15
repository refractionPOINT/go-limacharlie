package limacharlie

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type HiveClient struct {
	Organization *Organization
}

type HiveArgs struct {
	HiveName     string
	PartitionKey string
	Key          string
	Data         *[]byte
	Expiry       *int64
	Enabled      *bool
	Tags         *[]string
	ETag         *string
}

type HiveData struct {
	Data   map[string]interface{} `json:"data"`
	SysMtd SysMtd                 `json:"sys_mtd"`
	UsrMtd UsrMtd                 `json:"usr_mtd"`
}

type HiveInfo struct {
	Name      string `json:"name"`
	Partition string `json:"partition"`
}

type HiveResp struct {
	Guid string   `json:"guid"`
	Hive HiveInfo `json:"hive"`
	Name string   `json:"name"`
}

type SysMtd struct {
	CreatedBy   string `json:"created_by"`
	Etag        string `json:"etag"`
	GUID        string `json:"guid"`
	LastAuthor  string `json:"last_author"`
	LastError   string `json:"last_error"`
	LastErrorTs int64  `json:"last_error_ts"`
	LastMod     int64  `json:"last_mod"`
}
type UsrMtd struct {
	Enabled bool     `json:"enabled"`
	Expiry  int64    `json:"expiry"`
	Tags    []string `json:"tags"`
}

func NewHiveClient(org *Organization) *HiveClient {
	return &HiveClient{Organization: org}
}

func (h *HiveClient) List(args HiveArgs) (map[string]HiveData, error) {
	var hiveSet map[string]HiveData
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveSet)); err != nil {
		return nil, err
	}

	return hiveSet, nil
}

func (h *HiveClient) ListMtd(args HiveArgs) (map[string]HiveData, error) {
	var hiveSet map[string]HiveData
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveSet)); err != nil {
		return nil, err
	}

	// remove data field from return set
	for k, v := range hiveSet {
		hiveSet[k] = HiveData{SysMtd: v.SysMtd, UsrMtd: v.UsrMtd}
	}

	return hiveSet, nil
}

func (h *HiveClient) Get(args HiveArgs) (*HiveData, error) {
	if args.Key == "" {
		return nil, errors.New("key is required")
	}

	var hiveSet HiveData
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s/%s/data", args.HiveName, args.PartitionKey, args.Key), makeDefaultRequest(&hiveSet)); err != nil {
		return nil, err
	}

	return &hiveSet, nil
}

func (h *HiveClient) GetMTD(args HiveArgs) (*HiveData, error) {
	if args.Key == "" {
		return nil, errors.New("key is required")
	}

	var hd HiveData
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s/%s/mtd", args.HiveName, args.PartitionKey, args.Key), makeDefaultRequest(&hd)); err != nil {
		return nil, err
	}
	hd.Data = nil

	return &hd, nil
}

func (h *HiveClient) Add(args HiveArgs) (*HiveResp, error) {
	if args.Key == "" {
		return nil, errors.New("key required")
	}

	target := "mtd" // if no data set default to target type mtd
	var data map[string]interface{}
	if args.Data != nil {
		// ensure passed data can unmarshal correctly
		err := json.Unmarshal(*args.Data, &data)
		if err != nil {
			return nil, err
		}
		target = "data"
	}

	var userMtd UsrMtd // set UsrMtd Data
	if args.Expiry != nil {
		userMtd.Expiry = *args.Expiry
	}
	if args.Enabled != nil {
		userMtd.Enabled = *args.Enabled
	}
	if args.Tags != nil {
		userMtd.Tags = *args.Tags
	}

	reqDict := Dict{
		"data":    data,
		"usr_mtd": userMtd,
	}

	if args.ETag != nil {
		reqDict["etag"] = args.ETag
	}

	var hiveResp HiveResp
	req := makeDefaultRequest(&hiveResp).withQueryData(reqDict)
	if err := h.Organization.client.reliableRequest(http.MethodPost,
		fmt.Sprintf("hive/%s/%s/%s/%s", args.HiveName, args.PartitionKey, args.Key, target), req); err != nil {
		return nil, err
	}

	return &hiveResp, nil
}

func (h *HiveClient) Update(args HiveArgs) (interface{}, error) {
	if args.Key == "" {
		return nil, errors.New("key required")
	}

	target := "mtd" // if no data set default to target type mtd
	var existing *HiveData
	var err error
	if args.Data != nil {
		var data map[string]interface{}
		err = json.Unmarshal(*args.Data, &data)
		if err != nil {
			return nil, err
		}
		target = "data"

		existing, err = h.Get(args)
		if err != nil {
			return nil, err
		}
		existing.Data = data
	} else {
		existing, err = h.GetMTD(args)
		if err != nil {
			return nil, err
		}
	}

	// set usr mtd data
	if args.Expiry != nil {
		existing.UsrMtd.Expiry = *args.Expiry
	}
	if args.Enabled != nil {
		existing.UsrMtd.Enabled = *args.Enabled
	}
	if args.Tags != nil {
		existing.UsrMtd.Tags = *args.Tags
	}

	// empty data request only update with usr_mtd and etag
	reqData := Dict{}
	if target == "data" {
		reqData["data"] = existing.Data
		reqData["usr_mtd"] = existing.UsrMtd
		reqData["sys_mtd"] = existing.SysMtd
	} else {
		reqData["usr_mtd"] = existing.UsrMtd
		reqData["etag"] = existing.SysMtd.Etag
	}

	var updateResp HiveResp
	req := makeDefaultRequest(&updateResp).withQueryData(reqData)
	if err := h.Organization.client.reliableRequest(http.MethodPost,
		fmt.Sprintf("hive/%s/%s/%s/%s", args.HiveName, args.PartitionKey, args.Key, target), req); err != nil {
		return nil, err
	}

	return updateResp, nil
}

func (h *HiveClient) Remove(args HiveArgs, isPrint bool) (interface{}, error) {
	var delResp interface{}
	if err := h.Organization.client.reliableRequest(http.MethodDelete,
		fmt.Sprintf("hive/%s/%s/%s", args.HiveName, args.PartitionKey, args.Key), makeDefaultRequest(&delResp)); err != nil {
		return nil, err
	}

	return delResp, nil
}
