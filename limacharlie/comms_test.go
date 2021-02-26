package limacharlie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComms(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	testRoomName := "automated-test-room"

	comms := org.Comms()

	// Create a new room
	room, err := comms.CreateRoom(testRoomName)
	a.NoError(err)
	a.NotNil(room)

	if room.ID == "" {
		t.Errorf("missing room id: %+v", room)
	}
	if room.Nickname != testRoomName {
		t.Errorf("wrong room name: %v", room.Nickname)
	}

	// Reset a field and check we can get the room back
	room.Nickname = ""
	err = room.Update()
	a.NoError(err)
	if room.Nickname != testRoomName {
		t.Errorf("wrong room name: %v", room.Nickname)
	}

	// Post a message
	err = room.Post(NewMessage{
		Type:    "chat",
		Content: MessageText{"test-message"},
	})
	a.NoError(err)

	// Close room
	err = room.ChangeStatus(CommsCoreStatuses.Closed)
	a.NoError(err)

	// Check it is now closed
	room.Status = ""
	err = room.Update()
	a.NoError(err)
	if room.Status != CommsCoreStatuses.Closed {
		t.Errorf("wrong room status: %v", room.Status)
	}

	// Delete the room
	err = room.Delete()
	a.NoError(err)

	// Check we cannot get the room again
	err = room.Update()
	a.Error(err)
}
