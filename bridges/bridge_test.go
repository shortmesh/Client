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

var idMessage = "This room is bridged to `123456789@s.net` on WhatsApp"

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

func TestIsIdMessage(t *testing.T) {
	re := regexp.MustCompile("`([^`]+)`")
	match := re.FindStringSubmatch(idMessage)

	expected := "123456789@s.net"
	if expected != match[1] {
		t.Errorf("wanted %s, got %s", expected, match[1])
	}
}

func TestE164Extraction(t *testing.T) {
	sample := "sample_+23712345678"
	re := regexp.MustCompile(`\+[1-9]\d{6,14}`)
	if sample == re.FindString(sample) {
		t.Errorf("wanted false, got true")
	}
}
