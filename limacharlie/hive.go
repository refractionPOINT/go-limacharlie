package limacharlie

import (
	"bytes"
	"compress/gzip"
	"context"
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
	ARL          string
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
type UIAction struct {
	Label    string `json:"label" yaml:"label"`
	Location string `json:"location" yaml:"location"`
}

type UsrMtd struct {
	Enabled   bool       `json:"enabled" yaml:"enabled"`
	Expiry    int64      `json:"expiry" yaml:"expiry"`
	Tags      []string   `json:"tags" yaml:"tags"`
	Comment   string     `json:"comment" yaml:"comment"`
	UIActions []UIAction `json:"ui_actions,omitempty" yaml:"ui_actions,omitempty"`
}

type HiveName = string

type HiveKey = string

type HiveBatch struct {
	h        *HiveClient
	requests []Dict
}

type GetRecordRequest struct {
	Record RecordID `json:"record_id" yaml:"record_id"`
}

type MutationRequest struct {
	Record RecordID             `json:"record_id" yaml:"record_id"`
	Config ConfigRecordMutation `json:"record" yaml:"record"`
}

type MtdMutationRequest struct {
	Record RecordID `json:"record_id" yaml:"record_id"`
	UsrMtd UsrMtd   `json:"usr_mtd" yaml:"usr_mtd"`
	SysMtd SysMtd   `json:"sys_mtd" yaml:"sys_mtd"`
}

type MutationRenameRequest struct {
	Record  RecordID `json:"record_id" yaml:"record_id"`
	NewName string   `json:"new_name" yaml:"new_name"`
}

type DelRecordRequest struct {
	Record RecordID `json:"record_id" yaml:"record_id"`
}

type RecordName string
type GlobalID string
type PartitionID string

type HiveID struct {
	Name      HiveName    `json:"name" datastore:"name" yaml:"name"`
	Partition PartitionID `json:"partition" datastore:"partition" yaml:"partition"`
}

type RecordID struct {
	Hive HiveID     `json:"hive" datastore:"hive,flatten" yaml:"hive"`
	Name RecordName `json:"name" datastore:"name" yaml:"name"`
	GUID GlobalID   `json:"guid" datastore:"guid" yaml:"guid"`
}

type ConfigRecordMutation struct {
	SysMtd *SysMtd `json:"sys_mtd" yaml:"sys_mtd"`
	UsrMtd *UsrMtd `json:"usr_mtd" yaml:"usr_mtd"`
	Data   Dict    `json:"data" yaml:"data"`
	ARL    string  `json:"arl,omitempty" yaml:"arl,omitempty"`
}

type hiveBatchResponses struct {
	Responses []BatchResponse `json:"responses"`
}
type BatchResponse struct {
	Data  Dict   `json:"data"`
	Error string `json:"error"`
}

func NewHiveClient(org *Organization) *HiveClient {
	return &HiveClient{Organization: org}
}

// List returns every record of a hive, with its data.
//
// A single request returning a whole hive can be too big for the server to
// build. When the server refuses with HIVE_LISTING_TOO_LARGE (HTTP 413), List
// transparently falls back to walking the hive page by page and returns the
// combined map. Callers that only need metadata should use ListMtd, and
// callers that want to bound their own memory should use ListPages.
func (h *HiveClient) List(args HiveArgs) (HiveConfigData, error) {
	var hiveSet HiveConfigData
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), fullListingRequest(&hiveSet)); err != nil {
		if IsHiveListingTooLarge(err) {
			return h.ListAllPages(args, ListPageOptions{Limit: fallbackHivePageSize})
		}
		return nil, err
	}

	return hiveSet, nil
}

// ListMtd returns every record of a hive without its data. The server does
// not load the record data at all. Servers that predate the mtd_only
// parameter answer with the full records, so Data is always cleared here.
func (h *HiveClient) ListMtd(args HiveArgs) (HiveConfigData, error) {
	var hiveSet HiveConfigData
	req := fullListingRequest(&hiveSet).withQueryData(Dict{"mtd_only": "true"})
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), req); err != nil {
		if IsHiveListingTooLarge(err) {
			return h.ListAllPages(args, ListPageOptions{Limit: fallbackHivePageSize, MtdOnly: true})
		}
		return nil, err
	}

	stripHiveData(hiveSet)
	return hiveSet, nil
}

// hiveFullListingTimeout is the client timeout of a whole-hive listing. It is
// longer than the API gateway's own limit for these calls, so the gateway's
// answer (a result, 413 or 504) is what the caller sees rather than a client
// deadline, which would be retried.
const hiveFullListingTimeout = 150 * time.Second

// fullListingRequest is the request of a whole-hive listing. These are heavy
// for the backend, so a gateway timeout is not replayed.
func fullListingRequest(response interface{}) restRequest {
	return makeDefaultRequest(response).withTimeout(hiveFullListingTimeout).withoutGatewayTimeoutRetry()
}

// DefaultHivePageSize is the number of records requested per page when
// ListPageOptions.Limit is not set.
const DefaultHivePageSize = 50

// fallbackHivePageSize is the page size requested when a listing has to be
// walked page by page. The listing is already known to be big, and the server
// bounds the byte size of each page, so ask for as many records as it allows.
const fallbackHivePageSize = 500

// ListPageOptions controls one page of a hive listing.
type ListPageOptions struct {
	// Limit is the maximum number of records in the page. Values <= 0 use
	// DefaultHivePageSize. The server may return fewer records (it also
	// bounds the size of a page) and caps the limit it will honor.
	Limit int
	// Cursor is the NextCursor returned by the previous page. Empty for the
	// first page.
	Cursor string
	// MtdOnly returns the records without their data.
	MtdOnly bool
	// TagPrefix, when set, only returns records with a tag starting with it.
	TagPrefix string
	// MaxBytes optionally lowers the server-side size bound of a page.
	MaxBytes int
}

// ListPage returns one page of a hive listing, in record name order, and the
// cursor to pass in ListPageOptions.Cursor to get the next page. An empty
// cursor means this was the last page.
//
// A server that predates paging ignores the limit and cursor and answers with
// the whole hive as a plain name->record map. That is detected and returned as
// a single, final page, so callers behave the same against old and new servers.
func (h *HiveClient) ListPage(args HiveArgs, opts ListPageOptions) (HiveConfigData, string, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultHivePageSize
	}
	// "limit" must always be sent: it is what selects the paged response.
	q := Dict{"limit": fmt.Sprintf("%d", limit)}
	if opts.Cursor != "" {
		q["cursor"] = opts.Cursor
	}
	if opts.MtdOnly {
		q["mtd_only"] = "true"
	}
	if opts.TagPrefix != "" {
		q["tag_prefix"] = opts.TagPrefix
	}
	if opts.MaxBytes > 0 {
		q["max_bytes"] = fmt.Sprintf("%d", opts.MaxBytes)
	}

	var raw json.RawMessage
	req := makeDefaultRequest(&raw).withQueryData(q)
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodGet,
		fmt.Sprintf("hive/%s/%s", args.HiveName, args.PartitionKey), req); err != nil {
		return nil, "", err
	}

	records, next, err := decodeHiveListing(raw)
	if err != nil {
		return nil, "", err
	}
	if opts.MtdOnly {
		// Only a server that ignored mtd_only returns data here.
		stripHiveData(records)
	}
	return records, next, nil
}

// decodeHiveListing decodes a listing response in either the paged envelope
// {"records": {...}, "next_cursor": "..."} or the legacy plain map of records.
//
// It is an envelope when "records" is an object (or null) and "next_cursor" is
// present as a string (or null). A legacy map cannot match that: every one of
// its values is a record object, so a record named "next_cursor" or "records"
// does not have a string value for the former and is itself a record (carrying
// data/sys_mtd/usr_mtd) for the latter. A response with "records" as its only
// key and no "next_cursor" is still read as an envelope unless that "records"
// value is a record: a missing cursor means the last page, and reading such a
// page as a legacy map would silently drop every record.
func decodeHiveListing(raw json.RawMessage) (HiveConfigData, string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return HiveConfigData{}, "", nil
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, "", fmt.Errorf("invalid hive listing response: %w", err)
	}

	if recordsRaw, ok := top["records"]; ok && isJSONObjectOrNull(recordsRaw) {
		cursorRaw, hasCursor := top["next_cursor"]
		isEnvelope := false
		next := ""
		if hasCursor {
			if string(bytes.TrimSpace(cursorRaw)) == "null" {
				isEnvelope = true
			} else if err := json.Unmarshal(cursorRaw, &next); err == nil {
				isEnvelope = true
			}
		} else if len(top) == 1 && !looksLikeHiveRecord(recordsRaw) {
			isEnvelope = true
		}
		if isEnvelope {
			records := HiveConfigData{}
			if err := json.Unmarshal(recordsRaw, &records); err != nil {
				return nil, "", fmt.Errorf("invalid hive listing records: %w", err)
			}
			if records == nil {
				records = HiveConfigData{}
			}
			return records, next, nil
		}
	}

	records := HiveConfigData{}
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, "", fmt.Errorf("invalid hive listing response: %w", err)
	}
	if records == nil {
		records = HiveConfigData{}
	}
	return records, "", nil
}

func isJSONObjectOrNull(b json.RawMessage) bool {
	t := bytes.TrimSpace(b)
	return len(t) > 0 && (t[0] == '{' || string(t) == "null")
}

// looksLikeHiveRecord returns true when the JSON object is a hive record
// rather than a map of records.
func looksLikeHiveRecord(b json.RawMessage) bool {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return false
	}
	_, hasSys := keys["sys_mtd"]
	_, hasUsr := keys["usr_mtd"]
	return hasSys || hasUsr
}

// ListPages walks a whole hive page by page, calling fn with each page, so that
// only one page needs to be in memory at a time. opts.Cursor is the starting
// point (empty for the beginning of the hive). Returning an error from fn stops
// the walk and returns that error.
func (h *HiveClient) ListPages(args HiveArgs, opts ListPageOptions, fn func(page HiveConfigData) error) error {
	if fn == nil {
		return errors.New("page function required")
	}
	cursor := opts.Cursor
	for {
		opts.Cursor = cursor
		page, next, err := h.ListPage(args, opts)
		if err != nil {
			return err
		}
		if err := fn(page); err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		if next == cursor {
			return errors.New("hive listing did not advance: server returned the same cursor")
		}
		cursor = next
	}
}

// ListAllPages walks a whole hive page by page and returns the combined map.
func (h *HiveClient) ListAllPages(args HiveArgs, opts ListPageOptions) (HiveConfigData, error) {
	all := HiveConfigData{}
	if err := h.ListPages(args, opts, func(page HiveConfigData) error {
		for k, v := range page {
			all[k] = v
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return all, nil
}

// IsHiveListingTooLarge returns true when err is the server refusing to list a
// whole hive in one response (HTTP 413, HIVE_LISTING_TOO_LARGE). The listing
// has to be fetched page by page instead.
func IsHiveListingTooLarge(err error) bool {
	var restErr RESTError
	if errors.As(err, &restErr) && restErr.StatusCode() == http.StatusRequestEntityTooLarge {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "HIVE_LISTING_TOO_LARGE")
}

func stripHiveData(set HiveConfigData) {
	for k, v := range set {
		set[k] = HiveData{SysMtd: v.SysMtd, UsrMtd: v.UsrMtd}
	}
}

func (h *HiveClient) Get(args HiveArgs) (*HiveData, error) {
	if args.Key == "" {
		return nil, errors.New("key is required")
	}

	var hiveSet HiveData
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodGet,
		fmt.Sprintf("hive/%s/%s/%s/data", args.HiveName, args.PartitionKey, url.PathEscape(args.Key)), makeDefaultRequest(&hiveSet)); err != nil {
		return nil, err
	}

	return &hiveSet, nil
}

// GetPublicByGUID retrieves a public hive record using its GUID
func (h *HiveClient) GetPublicByGUID(args HiveArgs, guid string) (*HiveData, error) {
	if args.HiveName == "" {
		return nil, errors.New("hive name is required")
	}
	if args.PartitionKey == "" {
		return nil, errors.New("partition key is required")
	}
	if guid == "" {
		return nil, errors.New("GUID is required")
	}

	reqData := Dict{
		"guid": guid,
	}

	if args.ETag != nil {
		reqData["etag"] = args.ETag
	}

	var hiveData HiveData
	req := makeDefaultRequest(&hiveData).withFormData(reqData)

	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodPost,
		fmt.Sprintf("hive-public/%s/%s", args.HiveName, args.PartitionKey), req); err != nil {
		return nil, err
	}

	return &hiveData, nil
}

func (h *HiveClient) GetMTD(args HiveArgs) (*HiveData, error) {
	if args.Key == "" {
		return nil, errors.New("key is required")
	}

	var hd HiveData
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodGet,
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

	// Always create via the data endpoint. set_record_mtd only mutates an
	// existing record's metadata and returns RECORD_NOT_FOUND otherwise, so
	// it cannot be used to create a record — even when Data is empty.
	target := "data"

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

	if args.ARL != "" {
		reqDict["arl"] = args.ARL
	}

	var hiveResp HiveResp
	req := makeDefaultRequest(&hiveResp).withFormData(reqDict).withTimeout(30 * time.Second)
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodPost,
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
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodPost,
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
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodDelete,
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

	target := "rename"
	params := url.Values{}
	params.Add("new_name", newName)

	var hiveResp HiveResp
	req := makeDefaultRequest(&hiveResp).withFormData(params).withTimeout(30 * time.Second)
	if err := h.Organization.client.reliableRequest(context.Background(), http.MethodPost,
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

func (h *HiveClient) NewBatchOperations() *HiveBatch {
	return &HiveBatch{h: h, requests: []Dict{}}
}

func (b *HiveBatch) GetRecord(record RecordID) {
	b.requests = append(b.requests, Dict{
		"get_record": GetRecordRequest{Record: record},
	})
}

func (b *HiveBatch) GetRecordMtd(record RecordID) {
	b.requests = append(b.requests, Dict{
		"get_record_mtd": GetRecordRequest{Record: record},
	})
}

func (b *HiveBatch) SetRecord(record RecordID, config ConfigRecordMutation) {
	b.requests = append(b.requests, Dict{
		"set_record": MutationRequest{Record: record, Config: config},
	})
}

func (b *HiveBatch) SetRecordMtd(record RecordID, usrMtd UsrMtd, sysMtd SysMtd) {
	b.requests = append(b.requests, Dict{
		"set_record_mtd": MtdMutationRequest{Record: record, UsrMtd: usrMtd, SysMtd: sysMtd},
	})
}

func (b *HiveBatch) DelRecord(record RecordID) {
	b.requests = append(b.requests, Dict{
		"delete_record": DelRecordRequest{Record: record},
	})
}

func (b *HiveBatch) Execute() ([]BatchResponse, error) {
	// An empty batch is already successfully applied. Avoid sending a request
	// without any "request" form values, which the Hive endpoint rejects.
	if len(b.requests) == 0 {
		return []BatchResponse{}, nil
	}

	reqs := url.Values{}
	for _, req := range b.requests {
		d, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		reqs.Add("request", string(d))
	}
	resp := hiveBatchResponses{}
	req := makeDefaultRequest(&resp).withURLValues(reqs).withTimeout(5 * time.Minute)
	if err := b.h.Organization.client.reliableRequest(context.Background(), http.MethodPost, "hive", req); err != nil {
		return nil, err
	}

	return resp.Responses, nil
}
