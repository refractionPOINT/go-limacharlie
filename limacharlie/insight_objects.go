package limacharlie

import (
	"fmt"
	"net/http"
)

type InsightObjectType string

var InsightObjectTypes = struct {
	Domain      InsightObjectType
	Username    InsightObjectType
	IP          InsightObjectType
	FileHash    InsightObjectType
	FilePath    InsightObjectType
	FileName    InsightObjectType
	ServiceName InsightObjectType
	PackageName InsightObjectType
}{
	Domain:      "domain",
	Username:    "user",
	IP:          "ip",
	FileHash:    "file_hash",
	FilePath:    "file_path",
	FileName:    "file_name",
	ServiceName: "service_name",
	PackageName: "package_name",
}

type InsightObjectTypeInfoType string

var InsightObjectTypeInfoTypes = struct {
	Summary  InsightObjectTypeInfoType
	Location InsightObjectTypeInfoType
}{
	Summary:  "summary",
	Location: "locations",
}

type InsightObjectsRequest struct {
	IndicatorName   string
	ObjectType      InsightObjectType
	ObjectTypeInfo  InsightObjectTypeInfoType
	IsCaseSensitive bool
	AllowWildcards  bool
	SearchInLogs    bool
}

type InsightObjectsResponse struct {
	ObjectType    InsightObjectType `json:"type"`
	IndicatorName string            `json:"name"`
	FromCache     bool              `json:"from_cache"`
	Last1Day      int64             `json:"last_1_days"`
	Last7Days     int64             `json:"last_7_days"`
	Last30Days    int64             `json:"last_30_days"`
	Last365Days   int64             `json:"last_365_days"`
}

func (org Organization) InsightObjects(insightReq InsightObjectsRequest) (InsightObjectsResponse, error) {
	var resp InsightObjectsResponse
	if err := org.insightObjects(insightReq, false, &resp); err != nil {
		return InsightObjectsResponse{}, err
	}
	return resp, nil
}

type InsightObjectsPerObjectResponse struct {
	ObjectType    InsightObjectType `json:"type"`
	IndicatorName string            `json:"name"`
	FromCache     bool              `json:"from_cache"`
	Last1Day      Dict              `json:"last_1_days"`
	Last7Days     Dict              `json:"last_7_days"`
	Last30Days    Dict              `json:"last_30_days"`
	Last365Days   Dict              `json:"last_365_days"`
}

func (org Organization) InsightObjectsPerObject(insightReq InsightObjectsRequest) (InsightObjectsPerObjectResponse, error) {
	var resp InsightObjectsPerObjectResponse
	if err := org.insightObjects(insightReq, true, &resp); err != nil {
		return InsightObjectsPerObjectResponse{}, err
	}
	return resp, nil
}

type InsightObjectsBatchRequest struct {
	Objects         map[InsightObjectType][]string
	IsCaseSensitive bool
}

type InsightObjectBatchResponse struct {
	FromCache   bool `json:"from_cache"`
	Last1Day    Dict `json:"last_1_days"`
	Last7Days   Dict `json:"last_7_days"`
	Last30Days  Dict `json:"last_30_days"`
	Last365Days Dict `json:"last_365_days"`
}

func (org Organization) InsightObjectsBatch(insightReq InsightObjectsBatchRequest) (InsightObjectBatchResponse, error) {
	req := Dict{
		"objects":        insightReq.Objects,
		"case_sensitive": insightReq.IsCaseSensitive,
	}
	var resp InsightObjectBatchResponse
	request := makeDefaultRequest(&resp).withFormData(req)
	if err := org.client.reliableRequest(http.MethodPost, fmt.Sprintf("insight/%s/objects", org.client.options.OID), request); err != nil {
		return InsightObjectBatchResponse{}, err
	}
	return resp, nil
}

func (org Organization) insightObjects(insightReq InsightObjectsRequest, perObject bool, resp interface{}) error {
	req := Dict{
		"name":           insightReq.IndicatorName,
		"info":           insightReq.ObjectTypeInfo,
		"case_sensitive": insightReq.IsCaseSensitive,
		"with_wildcards": insightReq.AllowWildcards,
		"per_object":     perObject,
		"origin_type":    "sid",
	}
	if insightReq.SearchInLogs {
		req["origin_type"] = "lsid"
	}
	request := makeDefaultRequest(resp).withQueryData(req)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/objects/%s", org.client.options.OID, insightReq.ObjectType), request); err != nil {
		return err
	}
	return nil
}
