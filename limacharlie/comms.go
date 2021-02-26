package limacharlie

import (
	"fmt"
	"net/http"
	"time"
)

type EpochSecond = int64
type UserID = string
type RoomStatus = string
type RoomBucketingRule = string
type RoomPriority = int

type Comms struct {
	o *Organization
}

type Room struct {
	c *Comms

	ID         string      `json:"rid"`
	Nickname   string      `json:"nickname"`
	Assignees  []Assignee  `json:"assignees"`
	LastChange EpochSecond `json:"last_change"`
	CreateOn   EpochSecond `json:"created_on"`
	CreatedBy  UserID      `json:"created_by"`
	Status     RoomStatus  `json:"status"`
	Oid        string      `json:"oid"`

	BucketingRule RoomBucketingRule `json:"bucket_rule"`

	Priority RoomPriority `json:"priority"`
}

type Assignee struct {
	User       UserID      `json:"email"`
	Room       string      `json:"rid"`
	AssignedOn EpochSecond `json:"assigned_on"`
	AssignedBy string      `json:"assigned_by"`
}

type NewMessage struct {
	Type    string      `json:"type"`
	Parent  string      `json:"parent,omitempty"`
	Tags    []string    `json:"tag,omitempty"`
	Content interface{} `json:"content"`
}

type MessageText struct {
	Text string `json:"text"`
}

func (c *Comms) Room(roomID string) *Room {
	return &Room{
		c:  c,
		ID: roomID,
	}
}

func (c *Comms) CreateRoom(nickname string) (*Room, error) {
	r := &Room{
		c: c,
	}
	request := makeDefaultRequest(r).withFormData(Dict{
		"oid":      c.o.client.options.OID,
		"nickname": nickname,
	}).withTimeout(10 * time.Second).withURLRoot("/")
	if err := c.o.client.reliableRequest(http.MethodPost, fmt.Sprintf("comms/room"), request); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Room) Update() error {
	request := makeDefaultRequest(r).withURLRoot("/")
	if err := r.c.o.client.reliableRequest(http.MethodGet, fmt.Sprintf("comms/room/%s", r.ID), request); err != nil {
		return err
	}
	return nil
}

func (r *Room) Delete() error {
	resp := &GenericJSON{}
	request := makeDefaultRequest(resp).withURLRoot("/")
	if err := r.c.o.client.reliableRequest(http.MethodDelete, fmt.Sprintf("comms/room/%s", r.ID), request); err != nil {
		return err
	}
	return nil
}

func (r *Room) Post(message NewMessage) error {
	resp := GenericJSON{}
	request := makeDefaultRequest(&resp).withFormData(message).withURLRoot("/")
	if err := r.c.o.client.reliableRequest(http.MethodPost, fmt.Sprintf("comms/messages/%s", r.ID), request); err != nil {
		return err
	}
	return nil
}

func (r *Room) ChangeStatus(status RoomStatus) error {
	resp := GenericJSON{}
	request := makeDefaultRequest(&resp).withFormData(Dict{
		"status": status,
	}).withURLRoot("/")
	if err := r.c.o.client.reliableRequest(http.MethodPost, fmt.Sprintf("comms/room/%s", r.ID), request); err != nil {
		return err
	}
	r.Status = status
	return nil
}
