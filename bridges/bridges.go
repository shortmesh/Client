package bridges

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/creasty/defaults"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/rabbitmq"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Bridges struct {
	BridgeConfig configs.BridgeConfig
	RoomID       *id.RoomID
	Client       *mautrix.Client
}

type RMQExchanges struct {
	AddNewDevice string `default:"bridges.topic"`
}

type RMQBindingKeys struct {
	AddNewDevice string `default:"bridges.topic.add_new_device"`
}

func reverseForBridgeBot(client *mautrix.Client, roomId id.RoomID) (*Bridges, error) {
	room := rooms.Rooms{
		Client: client,
		ID:     &roomId,
	}
	members, err := room.GetRoomMembers()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	var bridgeBotContact id.UserID
	for _, member := range members {
		if member != client.UserID {
			bridgeBotContact = member
			break
		}
	}

	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	for _, bridgeConf := range conf.Bridges {
		userType, err := users.GetTypeUser(client, bridgeBotContact)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		if userType == users.BridgeBot {
			roomId := id.RoomID(roomId)
			return (&Bridges{
				BridgeConfig: bridgeConf,
				Client:       client,
				RoomID:       &roomId,
			}), nil
		}
	}

	return nil, nil
}

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
		deviceId := extractedMessage[len(extractedMessage)-1]
		(&devices.Devices{
			Client:     b.Client,
			DeviceId:   deviceId,
			BridgeName: b.BridgeConfig.Name,
		}).Save()

		err := rabbitmq.DeleteQueue(b.Client, b.Client.UserID.Localpart())
		if err != nil {
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

/*
- BAD_CREDENTIALS used when device has been disconnected (this can receive an incoming message), this can be used
when list-devices is ran to delete devices which are deactivated
*/
func processIncomingBotMessage(client *mautrix.Client, roomId id.RoomID, evt *event.Event) (*Bridges, error) {
	bridge, err := reverseForBridgeBot(client, roomId)
	if bridge == nil {
		return nil, nil
	}

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	message := evt.Content.AsMessage().Body

	isManagementRoom, err := rooms.IsManagementRoom(client, roomId, bridge.BridgeConfig.BotName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if isManagementRoom {
		slog.Debug(
			"Management room",
			"roomId", roomId.String(),
			"bridge", bridge.BridgeConfig.Name,
			"message", message,
		)
		isLoginMatched, err := bridge.checkIfLoginMessage(message)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}

		if isLoginMatched {
			return nil, err
		}

		isSuccessMatched, err := bridge.checkIfSuccess(message)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		if isSuccessMatched {
			return nil, err
		}

		isAddNewDeviceMatched, err := bridge.checkIfMatchDevice(evt)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		if isAddNewDeviceMatched {
			return nil, err
		}
	}

	// TODO: insert other possiblities

	return bridge, nil
}

func (b *Bridges) processIncomingMessages(evt *event.Event) error {
	userType, err := users.GetTypeUser(b.Client, evt.Sender)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if userType == users.BridgeBot {
		_, err := processIncomingBotMessage(b.Client, evt.RoomID, evt)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		return nil
	}

	// TODO: process other incoming messages for bridges

	return nil
}

func (b *Bridges) LookupBridgeByName(name string) (*Bridges, error) {
	roomsDb, err := rooms.GetRoomDb(b.Client)

	if err != nil {
		log.Println("Error initializing client db:", err)
		return nil, err
	}

	bridgeRoomIds, err := roomsDb.FetchRoomByName(name, true)
	if err != nil {
		return nil, err
	}

	if len(*bridgeRoomIds) < 1 {
		return nil, errors.New("No bridges found!")
	}

	roomId := id.RoomID((*bridgeRoomIds)[0])
	bridge := Bridges{
		RoomID: &roomId,
	}
	bridge.Client = b.Client

	conf, err := configs.GetConf()
	if err != nil {
		return nil, err
	}

	for _, confBridge := range conf.Bridges {
		if confBridge.Name == name {
			bridge.BridgeConfig = confBridge
		}
	}

	return &bridge, nil
}

func (b *Bridges) LookupBridgeByRoomId(roomId string) (*Bridges, error) {
	roomsDb, err := rooms.GetRoomDb(b.Client)

	if err != nil {
		log.Println("Error initializing client db:", err)
		return nil, err
	}

	bridgeName, err := roomsDb.FetchRoomByRoomId(roomId)
	if err != nil {
		return nil, err
	}

	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	for _, confBridge := range conf.Bridges {
		if confBridge.Name == bridgeName {
			b.BridgeConfig = confBridge
		}
	}

	return (&Bridges{
		BridgeConfig: configs.BridgeConfig{
			Name: bridgeName,
		},
	}), nil
}

func (b *Bridges) queryCommand(query string) error {
	slog.Debug("Bridge query", "name", b.BridgeConfig.Name, "query", query, "roomId", b.RoomID)
	_, err := b.Client.SendText(
		context.Background(),
		*b.RoomID,
		query,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}

func (b *Bridges) RemoveDevice(deviceId string) error {
	cmd := strings.ReplaceAll(b.BridgeConfig.Cmd["logout"], "%s", deviceId)
	slog.Debug("Bridge remove device", "cmd", cmd)
	// cmd = fmt.Sprintf("%s %s", cmd, deviceId)
	if err := b.queryCommand(cmd); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	err := (&devices.Devices{Client: b.Client, DeviceId: deviceId}).Remove()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func (b *Bridges) AddDevice() error {
	if err := b.queryCommand(b.BridgeConfig.Cmd["login"]); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func (b *Bridges) JoinManagementRooms() (id.RoomID, error) {
	_roomId, err := (&rooms.Rooms{
		Client:   b.Client,
		IsBridge: true,
	}).CreateRoom([]id.UserID{
		id.UserID(b.BridgeConfig.BotName),
	}, true)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	bridgeConf, found := conf.GetBridgeConfig(b.BridgeConfig.Name)
	if !found {
		slog.Error("Bridge not found", "name", b.BridgeConfig.Name)
		return "", err
	}

	roomId := id.RoomID(_roomId)
	err = (&rooms.Rooms{
		Client: b.Client,
		ID:     &roomId,
	}).SendMessage(bridgeConf.Cmd["management"])
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	b.RoomID = &roomId
	return roomId, nil
}

func (b *Bridges) Clear() error {
	roomsDb, err := rooms.GetRoomDb(b.Client)
	if err != nil {
		log.Println("Error initializing client db:", err)
		return err
	}

	// TODO: put device id and other params here
	if err := roomsDb.Clear(b.BridgeConfig.Name, true); err != nil {
		return err
	}

	return nil
}

func (b *Bridges) Save() error {
	roomsDb, err := rooms.GetRoomDb(b.Client)
	if err != nil {
		log.Println("Error initializing client db:", err)
		return err
	}

	// TODO: put device id and other params here
	if err := roomsDb.Save(b.RoomID.String(), b.BridgeConfig.Name, "", "", true); err != nil {
		return err
	}

	return nil
}

func (b *Bridges) SyncCallback(evt *event.Event) error {
	b.processIncomingMessages(evt)
	return nil
}
