package rooms

import (
	"testing"

	"github.com/shortmesh/core/utils"
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

func TestExtractRoomIncomplete(t *testing.T) {
	text := "You already have a direct chat with `03c08fc7-381c-453b-89f8-39cc0a4c91be` / Foo Foo at Foo Foo (https://matrix.to/#/!I_eNQbDaCCBB8frOVyYNf4nBsBuqwPrxFBrv31AjkXc)"
	roomId, err := utils.ExtractMatrixRoomID(text)
	if err != nil {
		t.Error(err)
	}

	expected := id.RoomID("!I_eNQbDaCCBB8frOVyYNf4nBsBuqwPrxFBrv31AjkXc")
	if roomId != expected.String() {
		t.Errorf("wanted %s, got %s", expected.String(), roomId)
	}
}
