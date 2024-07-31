package limacharlie

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HiveClient struct {
	Organization *Organization
}

type HiveArgs struct {
	HiveName     string
	PartitionKey string
	Key          string
	Data         Dict
	Expiry       *int64
	Enabled      *bool
	Tags         []string
	ETag         *string
	Comment      *string
}

type HiveConfigData map[string]HiveData

type HiveData struct {
	Data   map[string]interface{} `json:"data" yaml:"data,omitempty"`
	SysMtd SysMtd                 `json:"sys_mtd" yaml:"sys_mtd"`
	UsrMtd UsrMtd                 `json:"usr_mtd" yaml:"usr_mtd"`
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
	Etag        string `json:"etag" yaml:"etag"`
	CreatedBy   string `json:"created_by" yaml:"created_by"`
	CreatedAt   int64  `json:"created_at" yaml:"created_at"`
	LastAuthor  string `json:"last_author" yaml:"last_author"`
	LastMod     int64  `json:"last_mod" yaml:"last_mod"`
	GUID        string `json:"guid" yaml:"guid"`
	LastError   string `json:"last_error" yaml:"last_error"`
	LastErrorTs int64  `json:"last_error_ts" yaml:"last_error_ts"`
}
type UsrMtd struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Expiry  int64    `json:"expiry" yaml:"expiry"`
	Tags    []string `json:"tags" yaml:"tags"`
	Comment string   `json:"comment" yaml:"comment"`
}

type HiveName = string

type HiveKey = string

func NewHiveClient(org *Organization) *HiveClient {
	return &HiveClient{Organization: org}
}

func (h *HiveClient) List(args HiveArgs) (HiveConfigData, error) {
	var hiveSet HiveConfigData
	if err := h.Organization.client.reliableRequest(http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), makeDefaultRequest(&hiveSet)); err != nil {
		return nil, err
	}

	return hiveSet, nil
}

func (h *HiveClient) ListMtd(args HiveArgs) (HiveConfigData, error) {
	var hiveSet HiveConfigData
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
		fmt.Sprintf("hive/%s/%s/%s/data", args.HiveName, args.PartitionKey, url.PathEscape(args.Key)), makeDefaultRequest(&hiveSet)); err != nil {
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
		fmt.Sprintf("hive/%s/%s/%s/mtd", args.HiveName, args.PartitionKey, url.PathEscape(args.Key)), makeDefaultRequest(&hd)); err != nil {
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
	if len(args.Data) != 0 {
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
		userMtd.Tags = args.Tags
	}
	if args.Comment != nil {
		userMtd.Comment = *args.Comment
	}

	// Compress and encode the data since we're
	// likely to handle large amounts of it.
	zDat := &bytes.Buffer{}
	b64 := base64.NewEncoder(base64.StdEncoding, zDat)
	z := gzip.NewWriter(b64)
	if err := json.NewEncoder(z).Encode(args.Data); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	if err := b64.Close(); err != nil {
		return nil, err
	}

	reqDict := Dict{
		"gzdata":  zDat.String(),
		"usr_mtd": userMtd,
	}

	if args.ETag != nil {
		reqDict["etag"] = args.ETag
	}

	var hiveResp HiveResp
	req := makeDefaultRequest(&hiveResp).withFormData(reqDict).withTimeout(30 * time.Second)
	if err := h.Organization.client.reliableRequest(http.MethodPost,
		fmt.Sprintf("hive/%s/%s/%s/%s", args.HiveName, args.PartitionKey, url.PathEscape(args.Key), target), req); err != nil {
		return nil, err
	}

	return &hiveResp, nil
}

func (h *HiveClient) Update(args HiveArgs) (*HiveResp, error) {
	if args.Key == "" {
		return nil, errors.New("key required")
	}

	target := "mtd" // if no data set default to target type mtd
	existing := &HiveData{}
	var err error
	if len(args.Data) != 0 {
		target = "data"
		existing, err = h.Get(args)
		if err != nil {
			return nil, err
		}
		existing.Data = args.Data
	} else {
		existing, err = h.GetMTD(args)
		if err != nil {
			return nil, err
		}
	}

	// set usr mtd data
	var usrMtd UsrMtd
	if args.Expiry != nil {
		usrMtd.Expiry = *args.Expiry
	}
	if args.Enabled != nil {
		usrMtd.Enabled = *args.Enabled
	}
	if args.Tags != nil {
		usrMtd.Tags = args.Tags
	}
	if args.Comment != nil {
		usrMtd.Comment = *args.Comment
	}

	// empty data request only update with usr_mtd and etag
	reqData := Dict{}
	if target == "data" {
		reqData["data"] = existing.Data
		reqData["usr_mtd"] = usrMtd
		reqData["sys_mtd"] = existing.SysMtd
	} else {
		reqData["usr_mtd"] = usrMtd
		reqData["etag"] = existing.SysMtd.Etag
	}

	var updateResp HiveResp
	req := makeDefaultRequest(&updateResp).withFormData(reqData)
	if err := h.Organization.client.reliableRequest(http.MethodPost,
		fmt.Sprintf("hive/%s/%s/%s/%s", args.HiveName, args.PartitionKey, url.PathEscape(args.Key), target), req); err != nil {
		return nil, err
	}

	return &updateResp, nil
}

func (h *HiveClient) UpdateTx(args HiveArgs, tx func(record *HiveData) (*HiveData, error)) (*HiveResp, error) {
	// Perform a transactional update of the record
	// by using the "etag" provided by the API to make
	// sure we don't overwrite changes made to the
	// record between a fetch and a set.
	// The cb function will get called with the record
	// and expects a modified record to be returned by
	// it. This may get called more than once with updated
	// records if the transaction hits changes.
	if tx == nil {
		return nil, errors.New("tx function required")
	}
	rec, err := h.Get(args)
	if err != nil && !strings.Contains(err.Error(), "RECORD_NOT_FOUND") {
		return nil, err
	}
	for {
		eTag := ""
		if rec != nil {
			eTag = rec.SysMtd.Etag
		}

		newRec, err := tx(rec)
		if err != nil {
			return nil, err
		}
		if newRec == nil {
			return nil, nil
		}
		rec = newRec
		// Try to update the record.
		updResp, err := h.Add(HiveArgs{
			HiveName:     args.HiveName,
			PartitionKey: args.PartitionKey,
			Key:          args.Key,
			Data:         newRec.Data,
			Expiry:       &newRec.UsrMtd.Expiry,
			Enabled:      &newRec.UsrMtd.Enabled,
			Tags:         newRec.UsrMtd.Tags,
			ETag:         &eTag,
			Comment:      &newRec.UsrMtd.Comment,
		})
		if err != nil {
			if !strings.Contains(err.Error(), "ETAG_MISMATCH") {
				return nil, err
			}
			updResp = nil
		}
		if updResp != nil {
			return updResp, nil
		}
		// The update failed, the record changed on us.
		// Fetch the new record and try again.
		rec, err = h.Get(args)
		if err != nil {
			return nil, err
		}
	}
}

func (h *HiveClient) Remove(args HiveArgs) (interface{}, error) {
	var delResp interface{}
	if err := h.Organization.client.reliableRequest(http.MethodDelete,
		fmt.Sprintf("hive/%s/%s/%s", args.HiveName, args.PartitionKey, url.PathEscape(args.Key)), makeDefaultRequest(&delResp)); err != nil {
		return nil, err
	}

	return delResp, nil
}

// Rename renames a record in the Hive
func (h *HiveClient) Rename(args HiveArgs, newName string) (*HiveResp, error) {
	if args.Key == "" {
		return nil, errors.New("key required")
	}

	if newName == "" {
		return nil, errors.New("new name required")
	}

	target := "re-name"
	params := url.Values{}
	params.Add("new_name", url.PathEscape(newName))

	var hiveResp HiveResp
	req := makeDefaultRequest(&hiveResp).withFormData(params).withTimeout(30 * time.Second)
	if err := h.Organization.client.reliableRequest(http.MethodPost,
		fmt.Sprintf("hive/%s/%s/%s/%s", args.HiveName, args.PartitionKey, url.PathEscape(args.Key), target), req); err != nil {
		return nil, err
	}

	return &hiveResp, nil
}

func (hsd *HiveData) Equals(cData HiveData) (bool, error) {
	err := encodeDecodeHiveData(&hsd.Data)
	if err != nil {
		return false, err
	}

	newData, err := json.Marshal(hsd.Data)
	if err != nil {
		return false, err
	}
	if string(newData) == "null" {
		newData = nil
	}

	currentData, err := json.Marshal(cData.Data)
	if err != nil {
		return false, err
	}
	if string(currentData) == "null" {
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
