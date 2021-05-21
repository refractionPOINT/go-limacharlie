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

	linkType := "linktesttype"

	links, err := room.LinkGet()
	a.NoError(err)
	a.Empty(links.Links)
	linkID, err := room.LinkAdd(linkType, "link-test-value")
	a.NoError(err)
	a.NotEmpty(linkID)

	links, err = room.LinkGet()
	a.NoError(err)
	a.Equal(1, len(links.Links))
	link := links.Links[0]
	a.NotEmpty(link.MessageID)
	a.Equal("link-test-value", link.Value)
	a.Equal(linkType, link.Type)
	a.Equal(room.ID, link.Room)

	a.NoError(room.LinkDelete(linkType, "link-test-value"))

	links, err = room.LinkGet()
	a.NoError(err)
	a.Empty(links.Links)
}
