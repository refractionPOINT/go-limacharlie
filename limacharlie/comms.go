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

type newPostResp struct {
	Message struct {
		ID string `json:"mid"`
	} `json:"message"`
}

type MessageText struct {
	Text string `json:"text"`
}

type MessageTasking struct {
	Task    string   `json:"task"`
	Sensors []string `json:"sensors"`
}

type MessageTaskingResponse struct {
	Response map[string]interface{} `json:"response"`
}

type MessageError struct {
	Code    string `json:"code"`
	Message string `json:"msg"`

	// IsMaterial indicates the error relates
	// to an action/process/change that had a
	// real impact on a Room or other LimaCharlie
	// resource. It is up to the UI to determine
	// if/how non-material errors should be displayed.
	IsMaterial string `json:"is_material"`
}

type MessageCommandAck struct {
	CommandName string `json:"name"`
	CommandID   string `json:"cid"`
}

var CommsMessageTypes = struct {
	Chat           string
	Search         string
	SearchResponse string
	Task           string
	TaskResponse   string
	Error          string
	CommandAck     string
	Markdown       string
	Json           string
	Yaml           string
}{
	Chat:           "chat",
	Search:         "search",
	SearchResponse: "search-response",
	Task:           "task",
	TaskResponse:   "task-response",
	Error:          "error",
	CommandAck:     "cmdack",
	Markdown:       "markdown",
	Json:           "json",
	Yaml:           "yaml",
}

var CommsCoreStatuses = struct {
	Open     string
	Closed   string
	Archived string
}{
	Open:     "open",
	Closed:   "closed",
	Archived: "archived",
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
	if err := c.o.client.reliableRequest(http.MethodPost, "comms/room", request); err != nil {
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

func (r *Room) Post(message NewMessage) (string, error) {
	resp := newPostResp{}
	request := makeDefaultRequest(&resp).withFormData(message).withURLRoot("/")
	if err := r.c.o.client.reliableRequest(http.MethodPost, fmt.Sprintf("comms/messages/%s", r.ID), request); err != nil {
		return "", err
	}
	return resp.Message.ID, nil
}

func (r *Room) ChangeStatus(status RoomStatus) error {
	return r.ChangeStatusWithReason(status, "")
}

func (r *Room) ChangeStatusWithReason(status RoomStatus, reason string) error {
	resp := GenericJSON{}
	request := makeDefaultRequest(&resp).withFormData(Dict{
		"status": status,
		"reason": reason,
	}).withURLRoot("/")
	if err := r.c.o.client.reliableRequest(http.MethodPost, fmt.Sprintf("comms/room/%s", r.ID), request); err != nil {
		return err
	}
	r.Status = status
	return nil
}
