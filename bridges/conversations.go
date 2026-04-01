package bridges

import (
	"log/slog"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/creasty/defaults"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/rabbitmq"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func checkIfLoginMessage(bridgeConfig configs.BridgeConfig, message string) (*string, error) {
	cmd := bridgeConfig.Cmd["list-logins"]
	cmd = regexp.QuoteMeta(cmd)
	regexPattern := strings.ReplaceAll(cmd, "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if matched {
		extractedDeviceId, err := utils.ExtractBracketContent(message)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}

		deviceId, err := configs.FormatUsername(bridgeConfig.Name, extractedDeviceId)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}

		return deviceId, nil

	}

	return nil, nil
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
	exchange := RMQExchanges{}
	defaults.Set(&exchange)

	bindingKey := RMQBindingKeys{}
	defaults.Set(&bindingKey)

	regexPattern := strings.ReplaceAll(bridgeConfig.Cmd["login-qr-failed"], "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, evt.Content.AsMessage().Body)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	return matched, nil
}

func checkIfStartConversation(evt *event.Event) (bool, error) {
	expected := "Created chat with"
	message := evt.Content.AsMessage().Body
	if !strings.HasPrefix(message, expected) {
		return false, nil
	}
	return true, nil
}

/*
- BAD_CREDENTIALS used when device has been disconnected (this can receive an incoming message), this can be used
when list-devices is ran to delete devices which are deactivated
*/
func processIncomingBotMessage(client *mautrix.Client, evt *event.Event, bridgeCfg *configs.BridgeConfig) error {
	message := evt.Content.AsMessage().Body
	deviceId, err := checkIfLoginMessage(*bridgeCfg, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if deviceId != nil {
		err = (&devices.Devices{
			Client:     client,
			DeviceId:   *deviceId,
			BridgeName: bridgeCfg.Name,
		}).Save()
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		return err
	}

	deviceId, err = checkIfSuccess(*bridgeCfg, message)
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

		if err = rabbitmq.DeleteQueue(client, client.UserID.Localpart()); err != nil {
			slog.Error(err.Error())
			return err
		}
		return err
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
		err = rabbitmq.DeleteQueue(client, client.UserID.Localpart())
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		return err
	}

	// isStartConversation, err := checkIfStartConversation(evt)
	// if err != nil {
	// 	slog.Error(err.Error())
	// 	debug.PrintStack()
	// 	return err
	// }
	// if isStartConversation {
	// 	address := startConversationExtractE164Contact(message)
	// 	userId := evt.Content.AsMessage().Mentions.UserIDs[0]

	// 	err := (&contacts.Contacts{DbFilename: client.UserID.String()}).Save(address, userId.String())
	// 	if err != nil {
	// 		slog.Error(err.Error())
	// 		return err
	// 	}
	// 	return nil
	// }
	return nil
}
