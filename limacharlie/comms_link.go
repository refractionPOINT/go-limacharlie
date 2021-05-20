package limacharlie

import (
	"fmt"
	"net/http"
)

type LinkType = string
type LinkName = string

type CommsRoomLinks struct {
	Links []CommsRoomLink `json:"links" yaml:"links"`
}

type CommsRoomLink struct {
	Type      LinkType    `json:"ltype" yaml:"ltype"`
	Value     LinkName    `json:"name" yaml:"name"`
	CreatedOn EpochSecond `json:"created_on" yaml:"created_on"`
	CreatedBy UserID      `json:"created_by" yaml:"created_by"`
	Room      string      `json:"rid" yaml:"rid"`
	MessageID string      `json:"mid" yaml:"mid"`
}

func (r Room) linkUrl() string {
	return fmt.Sprintf("comms/room/%s/link", r.ID)
}

func (r Room) LinkGet(linkType ...string) (CommsRoomLinks, error) {
	resp := CommsRoomLinks{}
	req := makeDefaultRequest(&resp).withURLRoot("/")
	if len(linkType) > 0 {
		req = req.withQueryData(Dict{"type": linkType[0]})
	}
	if err := r.c.o.client.reliableRequest(http.MethodGet, r.linkUrl(), req); err != nil {
		return CommsRoomLinks{}, err
	}
	return resp, nil
}

func (r Room) LinkAdd(linkType string, linkValue string, messageID string) (string, error) {
	resp := Dict{}
	req := makeDefaultRequest(&resp).withURLRoot("/").withFormData(
		Dict{
			"type":  linkType,
			"value": linkValue,
			"mid":   messageID,
		},
	)
	if err := r.c.o.client.reliableRequest(http.MethodPost, r.linkUrl(), req); err != nil {
		return "", err
	}
	resplinkID, found := resp["id"]
	if !found {
		return "", fmt.Errorf("link: id missing from result: %v", resp)
	}
	linkID, ok := resplinkID.(string)
	if !ok {
		return "", fmt.Errorf("link: expected type string from result got %T", resplinkID)
	}
	return linkID, nil
}

func (r Room) LinkDelete(linkType string, linkValue string) error {
	resp := Dict{}
	req := makeDefaultRequest(&resp).withURLRoot("/").withFormData(
		Dict{
			"type":  linkType,
			"value": linkValue,
		},
	)
	if err := r.c.o.client.reliableRequest(http.MethodPost, r.linkUrl(), req); err != nil {
		return err
	}
	return nil
}
