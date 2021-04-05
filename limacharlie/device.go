package limacharlie

import (
	"fmt"
	"net/http"
	"time"
)

type Device struct {
	DID          string
	Organization *Organization
}

func (d *Device) AddTag(tag string, ttl time.Duration) error {
	req := makeDefaultRequest(&Dict{}).withFormData(Dict{
		"tags":   tag,
		"ttl":    ttl / time.Second,
		"is_did": true,
	})
	if err := d.Organization.client.reliableRequest(http.MethodPost, fmt.Sprintf("%s/tags", d.DID), req); err != nil {
		return err
	}
	return nil
}
