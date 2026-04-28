package bridges

import (
	"context"
	"log/slog"
	"maps"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/messages"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// type Bridges struct {
// 	BridgeConfig configs.BridgeConfig
// 	RoomID       *id.RoomID
// 	Client       *mautrix.Client
// }

type RMQExchanges struct {
	AddNewDevice string `default:"bridges.topic"`
}

type RMQBindingKeys struct {
	AddNewDevice string `default:"bridges.topic.add_new_device"`
}

func StartConversation(
	client *mautrix.Client,
	bridgeCfg *configs.BridgeConfig,
	deviceId, contact string,
) error {
	query := utils.ReplacePlaceholders(bridgeCfg.Cmd["start-conversation"], deviceId, contact)

	roomId, err := GetBotManagementRoom(client, (*id.UserID)(&bridgeCfg.BotName))
	if err != nil {
		slog.Error(err.Error())
		return nil
	}

	if roomId == nil {
		slog.Error("* Add device Error", "reason", "Bot managment room not found", "bridge", bridgeCfg.Name)
		return nil
	}

	err = queryCommand(client, roomId, query)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func queryCommand(client *mautrix.Client, roomId *id.RoomID, query string) error {
	_, err := client.SendText(
		context.Background(),
		*roomId,
		query,
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func RemoveDevice(client *mautrix.Client, bridgeCfg *configs.BridgeConfig, deviceId string) error {
	cmd := strings.ReplaceAll(bridgeCfg.Cmd["logout"], "%s", deviceId)

	roomId, err := GetBotManagementRoom(client, (*id.UserID)(&bridgeCfg.BotName))
	if err != nil {
		slog.Error(err.Error())
		return nil
	}

	if roomId == nil {
		slog.Error("* Add device Error", "reason", "Bot managment room not found", "bridge", bridgeCfg.Name)
		return nil
	}

	if err := queryCommand(client, roomId, cmd); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = (&devices.Devices{Client: client, DeviceId: deviceId}).Remove()
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	return nil
}

func GetId(client *mautrix.Client, bridgeCfg *configs.BridgeConfig, roomId *id.RoomID) error {
	cmd := bridgeCfg.Cmd["id"]

	if err := queryCommand(client, roomId, cmd); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func AddDevice(client *mautrix.Client, bridgeCfg *configs.BridgeConfig) error {
	cmd := bridgeCfg.Cmd["login"]

	roomId, err := GetBotManagementRoom(client, (*id.UserID)(&bridgeCfg.BotName))
	if err != nil {
		slog.Error(err.Error())
		return nil
	}

	if roomId == nil {
		slog.Error("* Add device Error", "reason", "Bot managment room not found", "bridge", bridgeCfg.Name)
		return nil
	}

	if err := queryCommand(client, roomId, cmd); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func JoinManagementRooms(client *mautrix.Client, bridgeCfg *configs.BridgeConfig) (*id.RoomID, error) {
	slog.Debug("Bridge", "status", "joining management")
	_roomId, err := (&rooms.Rooms{Client: client}).CreateRoom([]id.UserID{
		id.UserID(bridgeCfg.BotName),
	}, true)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	bridgeConf, err := configs.GetBridgeConfig(bridgeCfg.BotName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if bridgeConf == nil {
		return nil, err
	}

	roomId := id.RoomID(_roomId)
	err = messages.SendMessage(client, roomId, bridgeConf.Cmd["management"])
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	return &roomId, nil
}

func SyncCallback(client *mautrix.Client, evt *event.Event) error {
	bridgeCfg, err := configs.GetBridgeConfigByBotname(evt.Sender.String())
	if err != nil {
		debug.PrintStack()
		return err
	}
	if bridgeCfg == nil {
		return nil
	}

	// botUsername := id.UserID(bridgeCfg.BotName)
	// ok, err := isManagementRoom(client, evt.RoomID, botUsername)
	// if err != nil {
	// 	slog.Error(err.Error())
	// 	return err
	// }

	// if !ok {
	// 	return nil
	// }

	err = processIncomingBotMessage(client, evt, bridgeCfg)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil

}

func isManagementRoom(client *mautrix.Client, roomId id.RoomID, botUsername id.UserID) (bool, error) {
	members, err := client.JoinedMembers(context.Background(), roomId)
	if err != nil {
		return false, err
	}

	isSpace, err := isSpaceRoom(client, roomId)
	if err != nil {
		return false, err
	}

	if !isSpace {
		if len(members.Joined) == 2 {
			if _, ok := members.Joined[botUsername]; ok {
				if _, ok := members.Joined[client.UserID]; ok {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func isSpaceRoom(client *mautrix.Client, roomId id.RoomID) (bool, error) {
	var createContent event.CreateEventContent

	err := client.StateEvent(context.Background(), roomId, event.StateCreate, "", &createContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	// Check if "type" field is set to "m.space"
	if createContent.Type == "m.space" {
		return true, nil
	}
	return false, nil
}

func GetBotManagementRoom(client *mautrix.Client, botUsername *id.UserID) (*id.RoomID, error) {
	resp, err := client.JoinedRooms(context.Background())
	if err != nil {
		debug.PrintStack()
		return nil, err
	}

	for _, roomId := range resp.JoinedRooms {
		resp, err := client.JoinedMembers(context.Background(), roomId)
		if err != nil {
			debug.PrintStack()
			return nil, err
		}

		members := slices.Collect(maps.Keys(resp.Joined))
		ok, err := isManagementRoom(client, roomId, *botUsername)
		if err != nil {
			debug.PrintStack()
			return nil, err
		}

		if ok && slices.Contains(members, *botUsername) {
			return &roomId, nil
		}
	}

	return nil, nil
}

func AddBridge(client *mautrix.Client, bridgeConf configs.BridgeConfig) error {
	_, err := JoinManagementRooms(client, &bridgeConf)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func GetBridgeFromRoom(client *mautrix.Client, roomId *id.RoomID) (*configs.BridgeConfig, error) {
	cfg, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	resp, err := client.JoinedMembers(context.Background(), *roomId)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	for _, bridgeCfg := range cfg.Bridges {
		for resp := range resp.Joined {
			if bridgeCfg.BotName == resp.String() {
				return &bridgeCfg, nil
			}
		}
	}
	return nil, nil
}
