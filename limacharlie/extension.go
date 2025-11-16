package limacharlie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type ExtensionName = string

func (org *Organization) Extensions() ([]ExtensionName, error) {
	d := Dict{}
	if err := org.client.reliableRequest(context.Background(), http.MethodGet,
		fmt.Sprintf("orgs/%s/subscriptions", org.client.options.OID), makeDefaultRequest(&d)); err != nil {
		return nil, err
	}

	l := []ExtensionName{}
	for k := range d {
		l = append(l, k)
	}

	return l, nil
}

func (org *Organization) SubscribeToExtension(name ExtensionName) error {
	d := Dict{}
	if err := org.client.reliableRequest(context.Background(), http.MethodPost,
		fmt.Sprintf("orgs/%s/subscription/extension/%s", org.client.options.OID, url.PathEscape(name)), makeDefaultRequest(&d).withTimeout(1*time.Minute)); err != nil {
		return err
	}
	return nil
}

func (org *Organization) UnsubscribeFromExtension(name ExtensionName) error {
	d := Dict{}
	if err := org.client.reliableRequest(context.Background(), http.MethodDelete,
		fmt.Sprintf("orgs/%s/subscription/extension/%s", org.client.options.OID, url.PathEscape(name)), makeDefaultRequest(&d).withTimeout(1*time.Minute)); err != nil {
		return err
	}
	return nil
}

func (org *Organization) ReKeyExtension(name ExtensionName) error {
	d := Dict{}
	if err := org.client.reliableRequest(context.Background(), http.MethodPatch,
		fmt.Sprintf("orgs/%s/subscription/extension/%s", org.client.options.OID, url.PathEscape(name)), makeDefaultRequest(&d).withTimeout(1*time.Minute)); err != nil {
		return err
	}
	return nil
}
