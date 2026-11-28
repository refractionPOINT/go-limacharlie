package limacharlie

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
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

// payloadTransferTimeout limits one payload upload or download to storage,
// so a stalled connection cannot block the caller forever.
const payloadTransferTimeout = 10 * time.Minute

// ErrPayloadNotFound matches, with errors.Is, a *PayloadTransferError for a
// 404 from storage: the payload record exists but its content does not.
var ErrPayloadNotFound = errors.New("payload content not found")

// PayloadTransferError is returned when storage answers a payload upload
// (PUT) or download (GET) with a status other than 200.
type PayloadTransferError struct {
	Op         string
	StatusCode int
}

func (e *PayloadTransferError) Error() string {
	return fmt.Sprintf("failed to %s payload, http status: %d", e.Op, e.StatusCode)
}

// Is reports whether a 404 is compared with ErrPayloadNotFound.
func (e *PayloadTransferError) Is(target error) bool {
	return target == ErrPayloadNotFound && e.StatusCode == http.StatusNotFound
}

type payloadGetPointer struct {
	URL string `json:"get_url"`
}
type payloadPutPointer struct {
	URL string `json:"put_url"`
}

// List all the Payloads in an LC organization.
func (org *Organization) Payloads() (map[PayloadName]Payload, error) {
	resp := payloadsList{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodGet, fmt.Sprintf("payload/%s", org.client.options.OID), request); err != nil {
		return nil, err
	}
	return resp.Payloads, nil
}

// Download the content of a Payload in an LC organization.
func (org *Organization) Payload(name PayloadName) ([]byte, error) {
	return org.PayloadWithContext(context.Background(), name)
}

// PayloadWithContext downloads a payload like Payload, applying ctx to the
// API request, its retries and the download. The download also stops after
// payloadTransferTimeout. A storage answer other than 200 is a
// *PayloadTransferError, never payload content; a 404 also matches
// ErrPayloadNotFound with errors.Is.
func (org *Organization) PayloadWithContext(ctx context.Context, name PayloadName) ([]byte, error) {
	resp := payloadGetPointer{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(ctx, http.MethodGet, fmt.Sprintf("payload/%s/%s", org.client.options.OID, url.PathEscape(name)), request); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, payloadTransferTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.URL, nil)
	if err != nil {
		return nil, err
	}
	httpResp, err := org.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, &PayloadTransferError{Op: "GET", StatusCode: httpResp.StatusCode}
	}
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Delete a Payload from within an LC organization.
func (org *Organization) DeletePayload(name PayloadName) error {
	resp := Dict{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodDelete, fmt.Sprintf("payload/%s/%s", org.client.options.OID, url.PathEscape(name)), request); err != nil {
		return err
	}
	return nil
}

// Create a Payload in an LC organization.
func (org *Organization) CreatePayloadFromBytes(name PayloadName, data []byte) error {
	return org.CreatePayloadFromReader(name, bytes.NewBuffer(data))
}

func (org *Organization) CreatePayloadFromReader(name PayloadName, data io.Reader) error {
	resp := payloadPutPointer{}
	request := makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(context.Background(), http.MethodPost, fmt.Sprintf("payload/%s/%s", org.client.options.OID, url.PathEscape(name)), request); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), payloadTransferTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, resp.URL, data)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	httpResp, err := org.client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return &PayloadTransferError{Op: "PUT", StatusCode: httpResp.StatusCode}
	}
	return nil
}
