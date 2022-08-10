package limacharlie

import (
	"encoding/json"
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
	Data         interface{}
	expiry       string
}

type HiveData struct {
	Data   map[string]interface{} `json:"data"`
	SysMtd SysMtd                 `json:"sys_mtd"`
	UsrMtd UsrMtd                 `json:"usr_mtd"`
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
	Enabled bool        `json:"enabled"`
	Expiry  int         `json:"expiry"`
	Tags    interface{} `json:"tags"`
}

func NewHiveClient(org *Organization) *HiveClient {
	return &HiveClient{Organization: org}
}

func (h *HiveClient) List(args HiveArgs, isPrint bool) (interface{}, error) {
	hiveList := map[string]HiveData{}
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	if isPrint {
		h.printData(hiveList)
	}

	return hiveList, nil
}

func (h *HiveClient) ListMtd(args HiveArgs, isPrint bool) (interface{}, error) {
	hiveList := map[string]HiveData{}
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	// remove data field from return set
	for k, v := range hiveList {
		hiveList[k] = HiveData{SysMtd: v.SysMtd, UsrMtd: v.UsrMtd}
	}

	if isPrint {
		h.printData(hiveList)
	}

	return hiveList, nil
}

func (h *HiveClient) Get(args HiveArgs, isPrint bool) (interface{}, error) {

	var hiveList interface{}
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.Key), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	if isPrint {
		fmt.Printf("%+v \n", hiveList)
	}

	return hiveList, nil
}

func (h *HiveClient) GetMTD(args HiveArgs, isPrint bool) (interface{}, error) {

	var hiveList interface{}
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s/%s/mtd", args.HiveName, args.PartitionKey, args.Key), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	if isPrint {
		fmt.Printf("%+v \n", hiveList)
	}

	return hiveList, nil
}

func (h *HiveClient) Add(args HiveArgs) (interface{}, error) {

	if args.Key == "" {
		fmt.Println("error: Key Required")
	}

	target := "mtd"
	if args.Data != nil {
		// additonal logic here
		target = "data"
	}

	var userMtd interface{}
	// additional logic goes here

	req := makeDefaultRequest(h).withQueryData(Dict{
		"data":   args.Data,
		"usrMtd": userMtd,
	})

	var hiveList interface{}
	if err := h.Organization.client.reliableRequest(http.MethodPost,
		fmt.Sprintf("hive/%s/%s/%s/%s", args.HiveName, args.PartitionKey, args.Key, target), req); err != nil {
		return nil, err
	}

	return hiveList, nil
}

func (h *HiveClient) Update(args HiveArgs) (interface{}, error) {

	var hiveList interface{}
	if err := h.Organization.client.reliableRequest(http.MethodGet, fmt.Sprintf("hive/%s/%s", args.HiveName, args.Key), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	return hiveList, nil
}

func (h *HiveClient) Remove(args HiveArgs, isPrint bool) (interface{}, error) {

	var resp interface{}
	if err := h.Organization.client.reliableRequest(http.MethodDelete,
		fmt.Sprintf("hive/%s/%s/%s", args.HiveName, args.PartitionKey, args.Key), makeDefaultRequest(&resp)); err != nil {
		return nil, err
	}

	if isPrint {
		fmt.Println(resp)
	}

	return resp, nil
}

func (h *HiveClient) printData(data interface{}) {
	dataJson, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		fmt.Printf("%+v ", data)
	}

	fmt.Printf("%s\n", string(dataJson))
}
