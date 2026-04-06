package configs

import (
	"testing"
)

func TestCheckUserBridgeBotTemplate(t *testing.T) {
	bridgeConfig := BridgeConfig{
		UsernameTemplate: "signal_{{.}}",
	}

	username := "@signal_068387e0-9bb4-4de2-b807-c0436296b735:matrix.afkanerd.de"
	ok, err := CheckUserBridgeBotTemplate(bridgeConfig, username)
	if err != nil {
		t.Error(err)
	}

	if !ok {
		t.Error("got false, wanted true")
	}
}
