package limacharlie

import (
	"fmt"
	"net/http"
	"time"
)

func (c *Client) Outputs() (map[string]interface{}, error) {
	outputs := map[string]map[string]interface{}{}
	if err := c.reliableRequest(http.MethodGet, fmt.Sprintf("outputs/%s", c.options.OID), restRequest{
		nRetries: 3,
		timeout:  10 * time.Second,
		response: &outputs,
	}); err != nil {
		return nil, err
	}

	orgOutputs, ok := outputs[c.options.OID]
	if !ok {
		return nil, ResourceNotFoundError
	}
	return orgOutputs, nil
}
