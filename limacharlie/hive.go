package limacharlie

import (
	"fmt"
	"net/http"
)

type Hive struct {
	Organization *Organization
}

type HiveArgs struct {
	HiveName     string
	PartitionKey string
	Key          string
	Data         interface{}
	expiry       string
}

func (h *Hive) List(args HiveArgs, isPrint bool) (interface{}, error) {
	var hiveList interface{}
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	if isPrint {
		fmt.Printf("%+v \n", hiveList)
	}

	return hiveList, nil
}

func (h *Hive) ListMtd(args HiveArgs, isPrint bool) (interface{}, error) {
	var hiveList map[string]interface{}
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	if isPrint {
		// todo figure out response value from call
		//for _, r := range hiveList {
		//	//r["data"] = nil
		//	//
		//}
		fmt.Printf("%+v ", hiveList)
	}

	if isPrint {
		fmt.Printf("%+v \n", hiveList)
	}

	return hiveList, nil
}

func (h *Hive) Get(args HiveArgs, isPrint bool) (interface{}, error) {

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

func (h *Hive) GetMTD(args HiveArgs, isPrint bool) (interface{}, error) {

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

func (h *Hive) Add(args HiveArgs) (interface{}, error) {

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

func (h *Hive) Update(args HiveArgs) (interface{}, error) {

	var hiveList interface{}
	if err := h.Organization.client.reliableRequest(http.MethodGet, fmt.Sprintf("hive/%s/%s", args.HiveName, args.Key), makeDefaultRequest(&hiveList)); err != nil {
		return nil, err
	}

	return hiveList, nil
}

func (h *Hive) Remove(args HiveArgs, isPrint bool) (interface{}, error) {

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
