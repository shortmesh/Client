package bridges

import (
	"regexp"
	"strings"
	"testing"

	"github.com/shortmesh/core/utils"
)

// var conversationStartChat = "Created chat with `abcdefg-381c-453b-89f8-39cc0a4c91be` / +237123456789: https://example.to/#/!AowHgfnYrEphgpnVQL:matrix.example.com"
var conversationStartChat = "Signal private chat with +237123456789"

var successMessage = "Successfully logged in as example (`12345678`)"

func TestCheckIfStartConversation(t *testing.T) {
	expected := "+237123456789"
	output := utils.ExtractE164Contact(conversationStartChat)
	if expected != output {
		t.Errorf("wanted %s, got %s", expected, output)
	}
}

func TestSuccessParser(t *testing.T) {
	extractedMessage := strings.Fields(successMessage)

	deviceId := strings.ReplaceAll(extractedMessage[len(extractedMessage)-1], "+", "")
	re := regexp.MustCompile("[`()]")
	deviceId = re.ReplaceAllString(deviceId, "")

	expected := "12345678"
	if expected != deviceId {
		t.Errorf("wanted %s, got %s", expected, deviceId)
	}
}

// func TestCheckIfLoginMessage() {

// }

// func TestCheckIfSuccess() {

// }

// func TestCheckIfMatchDevice() {

// }
