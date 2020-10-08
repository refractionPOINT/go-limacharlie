package limacharlie

import (
	"net/url"
)

type mapString map[string]string

func (m mapString) urlEncode() string {
	values := url.Values{}
	for key, value := range m {
		values.Add(key, value)
	}
	return values.Encode()
}
