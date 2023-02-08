package limacharlie

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type WebhookSender struct {
	url    string
	client *http.Client
}

func (o *Organization) NewWebhookSender(hookName string, secretValue string) (*WebhookSender, error) {
	urls, err := o.GetURLs()
	if err != nil {
		return nil, fmt.Errorf("failed resolving org URLs: %v", err)
	}
	hookURL, ok := urls["hooks"]
	if !ok {
		return nil, errors.New("hook URL not found in org URLs")
	}
	return &WebhookSender{
		url: fmt.Sprintf("https://%s/%s/%s/%s", hookURL, o.GetOID(), hookName, secretValue),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (w *WebhookSender) Send(data interface{}) error {
	// Send a JSON webhook to a LimaCharlie Webhook Cloud Sensor/Adapter, requires
	// the `data` to be marshalable via the `json` package.
	b := &bytes.Buffer{}
	z := gzip.NewWriter(b)
	e := json.NewEncoder(z)
	if err := e.Encode(data); err != nil {
		return err
	}
	if err := z.Close(); err != nil {
		return err
	}
	r, err := http.NewRequest(http.MethodPost, w.url, b)
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Encoding", "gzip")
	r.Header.Set("User-Agent", "lc-sdk-webhook")

	resp, err := w.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpErrString := [128]byte{}
		resp.Body.Read(tmpErrString[:])
		return fmt.Errorf("http status code %d: %s", resp.StatusCode, string(tmpErrString[:]))
	}
	return nil
}

func (w *WebhookSender) Close() error {
	w.client.CloseIdleConnections()
	return nil
}
