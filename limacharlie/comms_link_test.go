package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommsLinkAddDelete(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	unsubCB, err := findUnsubscribeApiCallback(org, "comms")
	a.NoError(err)
	if unsubCB != nil {
		defer unsubCB()
	}

	room, err := org.Comms().CreateRoom("go-limacharlie-test-room")
	if a.NoError(err) {
		defer room.Delete()
	}

	links, err := room.LinkGet()
	a.NoError(err)
	a.Empty(links.Links)

	mid, err := room.Post(NewMessage{Type: "chat", Content: MessageText{"test-message"}})
	a.NoError(err)

	linkID, err := room.LinkAdd("link-test-type", "link-test-value", mid)
	a.NoError(err)
	a.NotEmpty(linkID)

	links, err = room.LinkGet()
	a.NoError(err)
	a.Equal(1, len(links.Links))
	link := links.Links[0]
	a.Equal(mid, link.MessageID)
	a.Equal("link-test-value", link.Value)
	a.Equal("link-test-type", link.Type)
	a.Equal(room.ID, link.Room)

	a.NoError(room.LinkDelete("link-test-type", "link-test-value"))

	links, err = room.LinkGet()
	a.NoError(err)
	a.Empty(links.Links)
}
