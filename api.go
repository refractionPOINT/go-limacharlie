package limacharlie

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

//  params = {}, altRoot = None, queryParams = None, rawBody = None, contentType = None, nMaxTotalRetries = 3 ):
type apiCall struct {
	Verb            string
	URL             string
	IsNoAuth        bool
	MaxTotalRetries int
	JWT             string
	APIVersion      int
	RootURL         string
	QueryParams     mapString
	ContentType     *string
	Body            apiCallBody
}

type apiCallBody interface {
	GetBody() string
}

type apiCallBodyEmpty struct {
}

func (body apiCallBodyEmpty) GetBody() string {
	return ""
}

// type apiCallBodyRaw struct {
// 	RawBody string
// }

// func (body *apiCallBodyRaw) GetBody() string {
// 	return body.RawBody
// }

// type apiCallBodyMap struct {
// 	Params mapString
// }

// func (body *apiCallBodyMap) GetBody() string {
// 	return body.Params.urlEncode()
// }

type apiCallResponse struct {
	returnCode int
	result     []byte
	err        error
}

func makeAPIResponseSuccess(body []byte) *apiCallResponse {
	return &apiCallResponse{200, body, nil}
}

func makeAPIResponseError(err error) *apiCallResponse {
	return &apiCallResponse{0, []byte{}, err}
}

func (apiCall *apiCall) buildQueryParams() string {
	queryParams := apiCall.QueryParams.urlEncode()
	if len(queryParams) > 1 {
		queryParams = "&" + queryParams
	}
	return queryParams
}

func (apiCall *apiCall) buildHeaders() http.Header {
	headers := http.Header{}
	headers.Add("User-Agent", "lc-go-api")
	if !apiCall.IsNoAuth {
		headers.Add("Authorization", "bearer "+apiCall.JWT)
	}
	if apiCall.ContentType != nil {
		headers.Add("Content-Type", *apiCall.ContentType)
	}
	return headers
}

func (apiCall *apiCall) buildURL() string {
	queryParams := apiCall.buildQueryParams()
	url := fmt.Sprintf("%s/v%d/%s", apiCall.RootURL, apiCall.APIVersion, apiCall.URL)
	return url + queryParams
}

func (apiCall *apiCall) buildBody() string {
	return apiCall.Body.GetBody()
}

func (apiCall *apiCall) restCall() *apiCallResponse {
	headers := apiCall.buildHeaders()
	url := apiCall.buildURL()
	body := apiCall.buildBody()
	req, err := http.NewRequest(apiCall.Verb, url, bytes.NewBufferString(body))
	if err == nil {
		return makeAPIResponseError(fmt.Errorf("Failed to create REST request: %s", err))
	}
	req.Header = headers
	client := http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return makeAPIResponseError(fmt.Errorf("Failed to execute request: %s", err))
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return makeAPIResponseError(fmt.Errorf("Failed to read response body: %s", err))
	}
	return makeAPIResponseSuccess(responseBody)
}
