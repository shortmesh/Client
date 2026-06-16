package bridges

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/creasty/defaults"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/rabbitmq"
	"github.com/shortmesh/core/rooms"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func checkIfIdMessage(bridgeConfig configs.BridgeConfig, message string) (string, error) {
	regexPattern := strings.ReplaceAll(bridgeConfig.Cmd["incoming_id"], "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	if matched {
		re := regexp.MustCompile("`([^`]+)`")
		match := re.FindStringSubmatch(message)

		// match[0] is the whole match, match[1] is the captured group
		if len(match) > 1 {
			return match[1], nil
		}
	}
	return "", nil
}

func checkIfSuccess(bridgeConfig configs.BridgeConfig, message string) (*string, error) {
	regexPattern := strings.ReplaceAll(bridgeConfig.Cmd["success"], "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if matched {
		extractedMessage := strings.Fields(message)

		deviceId := strings.ReplaceAll(extractedMessage[len(extractedMessage)-1], "+", "")
		re := regexp.MustCompile("[`()]")
		deviceId = re.ReplaceAllString(deviceId, "")
		return &deviceId, nil
	}
	return nil, nil
}

func checkIsQrLogin(bridgeConfig configs.BridgeConfig, evt *event.Event) (bool, error) {
	if evt.Content.AsMessage().FileName != "" &&
		evt.Content.AsMessage().FileName == bridgeConfig.Cmd["login-qr-filename"] {
		return true, nil
	}
	return false, nil
}

func checkIsFailedLogin(bridgeConfig configs.BridgeConfig, evt *event.Event) (bool, error) {
	// exchange := RMQExchanges{}
	// defaults.Set(&exchange)

	// bindingKey := RMQBindingKeys{}
	// defaults.Set(&bindingKey)

	regexPattern := strings.ReplaceAll(bridgeConfig.Cmd["login-qr-failed"], "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, evt.Content.AsMessage().Body)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	return matched, nil
}

func checkIsLogoutMessage(evt *event.Event) (*map[string]any, error) {
	raw := map[string]any{}
	if err := json.Unmarshal(evt.Content.VeryRaw, &raw); err != nil {
		return nil, err
	}

	if bridgeState, hasBridgeState := evt.Content.Raw["fi.mau.bridge_state"].(map[string]any); hasBridgeState {
		return &bridgeState, nil
	}

	return nil, nil
}

func processIncomingBotMessage(client *mautrix.Client, evt *event.Event, bridgeCfg *configs.BridgeConfig) error {
	message := evt.Content.AsMessage().Body

	rabbitmqQueueName := client.UserID.Localpart() + "_add_new_device"

	resp, err := checkIsLogoutMessage(evt)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if resp != nil {
		bridgeState := *resp
		slog.Debug("Bot state message", "bridgeState", bridgeState)
		if bridgeState["state_event"] == "BAD_CREDENTIALS" {
			deviceId := bridgeState["remote_id"]
			err = RemoveDevice(client, bridgeCfg, deviceId.(string))
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return err
			}
		}
		return nil
	}

	deviceId, err := checkIfSuccess(*bridgeCfg, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	if deviceId != nil {
		err := (&devices.Devices{
			Client:     client,
			DeviceId:   *deviceId,
			BridgeName: bridgeCfg.Name,
		}).Save()
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}

		if err = rabbitmq.DeleteQueue(client, rabbitmqQueueName); err != nil {
			slog.Error(err.Error())
			return err
		}
		return nil
	}

	isQrLogin, err := checkIsQrLogin(*bridgeCfg, evt)
	if isQrLogin {
		exchange := RMQExchanges{}
		defaults.Set(&exchange)

		bindingKey := RMQBindingKeys{}
		defaults.Set(&bindingKey)

		err := rabbitmq.Sender(
			client,
			message,
			exchange.AddNewDevice,
			bindingKey.AddNewDevice,
			rabbitmqQueueName,
		)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		return nil
	}

	isFailedLogin, err := checkIsFailedLogin(*bridgeCfg, evt)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	if isFailedLogin {
		err = rabbitmq.DeleteQueue(client, rabbitmqQueueName)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		return err
	}

	id, err := checkIfIdMessage(*bridgeCfg, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	if id != "" {
		slog.Debug("Bot message ID found", "id", id)
		err = rooms.SaveBridgedId(client, id, evt.RoomID.String(), bridgeCfg.Name)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
	}

	return nil
}
