package messages

import (
	"slices"
	"testing"

	"github.com/shortmesh/core/utils"
)

func TestExtractingContacts(t *testing.T) {
	message := "@+237123456789 hello world"
	expected := []string{"+237123456789"}

	output := utils.ExtractAllE164ForTagging(message)

	if !slices.Equal(expected, output) {
		t.Errorf("wanted %s(%d), got %s(%d)", expected, len(expected), output, len(output))
		return
	}

	message = "hello @+237123456789 hello world and 237123456789 @+237123456781"
	expected = []string{"+237123456789", "+237123456781"}
	output = utils.ExtractAllE164ForTagging(message)
	if !slices.Equal(expected, output) {
		t.Errorf("wanted %s, got %s", expected, output)
		return
	}
}

func TestMatchesTemplate(t *testing.T) {
	name := "237652156811 (WA)"
	template := "{{.}} (WA)"

	if !utils.MatchesTemplate(template, name) {
		t.Errorf("wanted true, got false: %s - %s", template, name)
	}
}

func TestExtractContents(t *testing.T) {
	name := "+237652156811 (WA)"
	template := "{{.}} (WA)"
	expected := "+237652156811"
	output := utils.ExtractTemplateValue(template, name)

	if output != expected {
		t.Errorf("wanted %s, got %s", expected, output)
	}
}
