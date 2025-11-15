package limacharlie

import (
	"context"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Payload struct {
	Name      string `json:"name"`
	Oid       string `json:"oid"`
	Size      uint64 `json:"size"`
	By        string `json:"by"`
	CreatedOn uint64 `json:"created"`
}
type payloadsList struct {
	Payloads map[PayloadName]Payload `json:"payloads"`
}
type PayloadName = string
type payloadGetPointer struct {
	URL string `json:"get_url"`
}
type payloadPutPointer struct {
	URL string `json:"put_url"`
}

// List all the Payloads in an LC organization.
func (org Organization) Payloads() (map[PayloadName]Payload, error) {
	resp := payloadsList{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodGet, fmt.Sprintf("payload/%s", org.client.options.OID), request); err != nil {
		return nil, err
	}
	return resp.Payloads, nil
}

// Download the content of a Payload in an LC organization.
func (org Organization) Payload(name PayloadName) ([]byte, error) {
	resp := payloadGetPointer{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodGet, fmt.Sprintf("payload/%s/%s", org.client.options.OID, url.PathEscape(name)), request); err != nil {
		return nil, err
	}
	httpResp, err := http.Get(resp.URL)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	data, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Delete a Payload from within an LC organization.
func (org Organization) DeletePayload(name PayloadName) error {
	resp := Dict{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, fmt.Sprintf("payload/%s/%s", org.client.options.OID, url.PathEscape(name)), request); err != nil {
		return err
	}
	return nil
}

// Create a Payload in an LC organization.
func (org Organization) CreatePayloadFromBytes(name PayloadName, data []byte) error {
	return org.CreatePayloadFromReader(name, bytes.NewBuffer(data))
}

func (org Organization) CreatePayloadFromReader(name PayloadName, data io.Reader) error {
	resp := payloadPutPointer{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodPost, fmt.Sprintf("payload/%s/%s", org.client.options.OID, url.PathEscape(name)), request); err != nil {
		return err
	}
	c := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, resp.URL, data)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	httpResp, err := c.Do(req)
	if err != nil {
		return err
	}
	if httpResp.StatusCode != 200 {
		return fmt.Errorf("failed to PUT payload, http status: %d", httpResp.StatusCode)
	}
	return nil
}
