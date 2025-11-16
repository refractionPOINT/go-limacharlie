package limacharlie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (org *Organization) GetIngestionKeys() (Dict, error) {
	resp := map[string]Dict{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodGet, fmt.Sprintf("insight/%s/ingestion_keys", org.GetOID()), request); err != nil {
		return nil, err
	}
	keys, ok := resp["keys"]
	if !ok {
		return nil, fmt.Errorf("no ingestion keys")
	}

	return keys, nil
}

func (org *Organization) SetIngestionKeys(name string) (Dict, error) {
	resp := Dict{}
	req := Dict{
		"name": name,
	}
	request := makeDefaultRequest(&resp).withFormData(req)
	if err := org.client.reliableRequest(context.Background(), http.MethodPost, fmt.Sprintf("insight/%s/ingestion_keys", org.GetOID()), request); err != nil {
		return nil, err
	}
	return resp, nil
}

func (org *Organization) DelIngestionKeys(name string) (Dict, error) {
	resp := Dict{}
	req := Dict{}
	escapedName := url.QueryEscape(name)
	request := makeDefaultRequest(&resp).withFormData(req)
	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, fmt.Sprintf("insight/%s/ingestion_keys?name=%s", org.GetOID(), escapedName), request); err != nil {
		return nil, err
	}
	return resp, nil
}
