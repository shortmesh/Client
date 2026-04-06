package rooms

import (
	"testing"

	"maunium.net/go/mautrix/id"
)

func TestExtractRoom(t *testing.T) {
	text := "Created chat with `068387e0-9bb4-4de2-b807-c0436296b735` SMSWithoutBorders: https:matrix.to#!rrdTtjPkksWibPRuVg1:matrix.example.com"
	roomId, err := ExtractMatrixRoomID(text)
	if err != nil {
		t.Error(err)
	}

	expected := id.RoomID("!rrdTtjPkksWibPRuVg1:matrix.example.com")
	if roomId.String() != expected.String() {
		t.Errorf("wanted %s, got %s", expected.String(), roomId.String())
	}
}
