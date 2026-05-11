package utils

import "testing"

func TestExtractBrackContent(t *testing.T) {
	data := "That invite link points at Group 3 (`120373408169626061@g.us`)"
	outcome, err := ExtractBracketContent(data)
	if err != nil {
		t.Error(err)
	}
	expected := "`120373408169626061@g.us`"
	if expected != outcome {
		t.Errorf("wanted %s, got %s", expected, outcome)
	}
}
