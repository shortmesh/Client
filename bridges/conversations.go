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
	"maunium.net/go/mautrix/event"
)

func (b *Bridges) checkIfLoginMessage(message string) (bool, error) {
	cmd := b.BridgeConfig.Cmd["list-logins"]
	cmd = regexp.QuoteMeta(cmd)
	regexPattern := strings.ReplaceAll(cmd, "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if matched {
		deviceId, err := utils.ExtractBracketContent(message)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}

		cfg, err := configs.GetConf()
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}
		deviceId, err = cfg.FormatUsername(b.BridgeConfig.Name, deviceId)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}

		slog.Debug("Saving device", "bridgeName", b.BridgeConfig.Name)

		err = (&devices.Devices{
			Client:     b.Client,
			DeviceId:   deviceId,
			BridgeName: b.BridgeConfig.Name,
		}).Save()
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}
		slog.Debug("Saved new device", "name", deviceId)
	}
	return false, nil
}

func (b *Bridges) checkIfSuccess(message string) (bool, error) {
	regexPattern := strings.ReplaceAll(b.BridgeConfig.Cmd["success"], "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if matched {
		extractedMessage := strings.Fields(message)

		// cfg, err := configs.GetConf()
		// if err != nil {
		// 	slog.Error(err.Error())
		// 	debug.PrintStack()
		// 	return false, err
		// }

		// deviceId, err := cfg.FormatUsername(b.BridgeConfig.Name, extractedMessage[len(extractedMessage)-1])
		deviceId := strings.ReplaceAll(extractedMessage[len(extractedMessage)-1], "+", "")

		err := (&devices.Devices{
			Client:     b.Client,
			DeviceId:   deviceId,
			BridgeName: b.BridgeConfig.Name,
		}).Save()
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}

		if err = rabbitmq.DeleteQueue(b.Client, b.Client.UserID.Localpart()); err != nil {
			slog.Error(err.Error())
			return false, err
		}
		slog.Debug("Saved new device", "name", deviceId)
	}
	return false, nil
}

func (b *Bridges) checkIfMatchDevice(evt *event.Event) (bool, error) {
	exchange := RMQExchanges{}
	defaults.Set(&exchange)

	bindingKey := RMQBindingKeys{}
	defaults.Set(&bindingKey)

	if evt.Content.AsMessage().FileName != "" &&
		evt.Content.AsMessage().FileName == b.BridgeConfig.Cmd["login-qr-filename"] {
		slog.Debug("Login QR found", "bridge", b.BridgeConfig.Name)

		err := rabbitmq.Sender(
			b.Client,
			evt.Content.AsMessage().Body,
			exchange.AddNewDevice,
			bindingKey.AddNewDevice,
		)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}
		return true, nil
	}

	regexPattern := strings.ReplaceAll(b.BridgeConfig.Cmd["login-qr-failed"], "%s", ".*")
	matched, err := regexp.MatchString(regexPattern, evt.Content.AsMessage().Body)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if matched {
		slog.Debug("Failed Login QR found", "bridge", b.BridgeConfig.Name)
		err = rabbitmq.DeleteQueue(b.Client, b.Client.UserID.Localpart())
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}
		slog.Debug("Queue deleted", "queueName", b.Client.UserID)
	}

	return matched, nil
}

// func (b *Bridges) checkIfStartConversation(evt *event.Event) (bool, error) {
// 	// regexPattern := strings.ReplaceAll(b.BridgeConfig.Cmd["success"], "%s", ".*")
// 	// matched, err := regexp.MatchString(regexPattern, message)
// 	message := evt.Content.AsMessage().Body
// 	if strings.Contains(message, "Created chat with") {
// 		b.BridgeConfig.BotName
// 		return true, nil
// 	}

// 	return false, nil

// }
