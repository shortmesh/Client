package cmd

import (
	// 	"context"
	// 	"fmt"

	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Controller struct {
	Client *mautrix.Client
}

func SyncUsers() error {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	// ? Read all users from the database
	user := configs.UsersConfig{
		Username:         conf.User.Username,
		AccessToken:      conf.User.AccessToken,
		RecoveryKey:      conf.User.RecoveryKey,
		HomeServer:       conf.HomeServer,
		HomeServerDomain: conf.HomeServerDomain,
	}

	syncUser(user)
	return nil
}

func syncUser(user configs.UsersConfig) error {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	client, err := mautrix.NewClient(
		user.HomeServer,
		id.NewUserID(user.Username, user.HomeServerDomain),
		user.AccessToken,
	)
	if err != nil {
		panic(err)
	}

	err = rooms.ParseRoomSubroutine(client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	client.DeviceID = id.DeviceID(conf.User.DeviceId)
	cryptoHelper, err := SetupCryptoHelper(client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	mc := MatrixClient{
		Client:       client,
		CryptoHelper: cryptoHelper,
	}

	mc.Client.Crypto = cryptoHelper

	var wg sync.WaitGroup
	wg.Add(1)

	ch := make(chan *event.Event)

	go func() {
		for {
			evt := <-ch
			if evt == nil {
				continue
			}

			json, err := json.MarshalIndent(evt, "", "")

			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				continue
			}
			fmt.Printf("%s\n", json)

			go (&bridges.Bridges{
				Client: client,
			}).ProcessIncomingMessages(evt)
		}
	}()

	err = mc.Sync(ch)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}

func (c *Controller) AddDevice(bridgeName string) error {
	bridge, err := (&bridges.Bridges{
		Client: c.Client,
	}).LookupBridgeByName(bridgeName)
	// log.Printf("Found bridge room: %s\n", bridge.RoomID)

	if err != nil {
		return err
	}

	err = bridge.AddDevice()
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) AddBridges() error {
	// ? User authenticates themselves
	// ? Bridges are added to users account
	// ?   read configs for bridges
	// ?   for(bridges):
	// ?    If bridges not already added --> add them
	// ?    addbridge():
	// ?     invite bridge to join the room - multiple rooms get created
	// ?     add bridge to database

	conf, err := configs.GetConf()
	if err != nil {
		return err
	}

	bridgeConfs := conf.Bridges

	for i, confBridge := range bridgeConfs {
		log.Printf("[+] (%d\\%d) Bridge: %s\n", i+1, len(bridgeConfs), confBridge.Name)

		//TODO: CheckRoomExists(client):
		bridge := bridges.Bridges{
			BridgeConfig: confBridge,
			Client:       c.Client,
		}
		roomId, err := bridge.JoinManagementRooms()
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		bridge.RoomID = &roomId
		if err := bridge.Save(); err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		log.Printf("Room created: %s\n", bridge.RoomID)
	}

	return nil

}

// !Danger if room already exist, this won't fail but would create a failed room
// !Have something that records all existing rooms into a db at start
func createContactRoom(room rooms.Rooms, bridgeName, contact, deviceId string) (*id.RoomID, error) {
	cfg, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	contactUsername, err := cfg.FormatUsername(bridgeName, contact)
	deviceIdUsername, err := cfg.FormatUsername(bridgeName, deviceId)
	slog.Debug("Contactusername: " + contactUsername)
	slog.Debug("Deviceusername: " + deviceIdUsername)

	bridge, err := (&bridges.Bridges{
		Client: room.Client,
	}).LookupBridgeByName(bridgeName)
	if err != nil {
		return nil, err
	}

	botUsername := bridge.BridgeConfig.BotName
	slog.Debug("Botusername: " + botUsername)

	roomId, err := (&rooms.Rooms{
		Client:   room.Client,
		IsBridge: true,
	}).JoinRoom([]id.UserID{
		id.UserID(contactUsername),
		id.UserID(deviceIdUsername),
		id.UserID(botUsername),
	})

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	err = room.Save(
		bridgeName,
		contact,
		deviceId,
		false,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return &roomId, nil

}

func (c *Controller) SendMessage(bridgeName, deviceId, contact, message string) (*id.RoomID, error) {
	// contact = strings.ReplaceAll(contact, "+", "")
	cfg, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	contactUsername, err := cfg.FormatUsername(bridgeName, contact)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	deviceIdUsername, err := cfg.FormatUsername(bridgeName, deviceId)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	roomIdStr, err := users.FetchMessageContact(
		c.Client,
		deviceIdUsername,
		bridgeName,
		contactUsername,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	roomId := id.RoomID(*roomIdStr)
	room := rooms.Rooms{
		Client: c.Client,
		ID:     &roomId,
	}

	if room.ID == nil {
		slog.Debug("Creating contact room!")
		roomId, err := createContactRoom(room, bridgeName, contactUsername, deviceIdUsername)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		room.ID = roomId
	}

	err = (&MatrixClient{Client: c.Client}).SendMessage(*room.ID, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return room.ID, nil
}
