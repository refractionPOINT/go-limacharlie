package limacharlie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// payloadStore answers the payload API with a signed URL on the same server,
// and the signed URL with a fixed status and body.
type payloadStore struct {
	status int
	body   string
	stall  chan struct{}
}

func (p *payloadStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/blob" {
		if p.stall != nil {
			select {
			case <-p.stall:
			case <-r.Context().Done():
			}
			return
		}
		w.WriteHeader(p.status)
		_, _ = w.Write([]byte(p.body))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	key := "get_url"
	if r.Method == http.MethodPost {
		key = "put_url"
	}
	_, _ = fmt.Fprintf(w, `{%q:"http://%s/blob"}`, key, r.Host)
}

func TestPayloadDownloadChecksStatus(t *testing.T) {
	t.Run("200 returns the content", func(t *testing.T) {
		org, _ := newSIDsTestOrg(t, &payloadStore{status: http.StatusOK, body: "content"})
		data, err := org.Payload("p")
		require.NoError(t, err)
		require.Equal(t, "content", string(data))
	})
	t.Run("a storage error is an error, not content", func(t *testing.T) {
		for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
			org, _ := newSIDsTestOrg(t, &payloadStore{status: status, body: "<Error>AccessDenied</Error>"})
			data, err := org.Payload("p")
			require.Nil(t, data)
			var te *PayloadTransferError
			require.ErrorAs(t, err, &te)
			require.Equal(t, status, te.StatusCode)
			require.Equal(t, "GET", te.Op)
			require.False(t, errors.Is(err, ErrPayloadNotFound))
		}
	})
	t.Run("404 matches ErrPayloadNotFound", func(t *testing.T) {
		org, _ := newSIDsTestOrg(t, &payloadStore{status: http.StatusNotFound})
		_, err := org.Payload("p")
		require.ErrorIs(t, err, ErrPayloadNotFound)
	})
	t.Run("the context stops a stalled download", func(t *testing.T) {
		stall := make(chan struct{})
		defer close(stall)
		org, _ := newSIDsTestOrg(t, &payloadStore{stall: stall})
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := org.PayloadWithContext(ctx, "p")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Less(t, time.Since(start), 5*time.Second)
	})
}

func TestPayloadUploadChecksStatus(t *testing.T) {
	org, _ := newSIDsTestOrg(t, &payloadStore{status: http.StatusOK})
	require.NoError(t, org.CreatePayloadFromBytes("p", []byte("x")))

	org, _ = newSIDsTestOrg(t, &payloadStore{status: http.StatusForbidden})
	err := org.CreatePayloadFromBytes("p", []byte("x"))
	var te *PayloadTransferError
	require.ErrorAs(t, err, &te)
	require.Equal(t, "PUT", te.Op)
	require.Equal(t, http.StatusForbidden, te.StatusCode)
}
