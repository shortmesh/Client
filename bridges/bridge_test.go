package bridges

import (
	"testing"
)

var conversationStartChat = "Created chat with `abcdefg-381c-453b-89f8-39cc0a4c91be` / +237123456789: https://example.to/#/!AowHgfnYrEphgpnVQL:matrix.example.com"

func TestCheckIfStartConversation(t *testing.T) {
	expected := "+237123456789"
	output := startConversationExtractE164Contact(conversationStartChat)
	if expected != output {
		t.Errorf("wanted %s, got %s", expected, output)
	}
}

// func TestCheckIfLoginMessage() {

// }

// func TestCheckIfSuccess() {

// }

// func TestCheckIfMatchDevice() {

// }
