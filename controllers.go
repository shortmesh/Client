package main

import (
	// 	"context"
	// 	"fmt"

	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Controller struct {
	Client *mautrix.Client
}

var cfg, cfgError = (&Conf{}).getConf()

var ks = Keystore{
	filepath: cfg.KeystoreFilepath,
}

func SyncUsers() {
	conf, err := cfg.getConf()

	if err != nil {
		panic(err)
	}
	user := User{
		Username:         conf.User.Username,
		AccessToken:      conf.User.AccessToken,
		RecoveryKey:      conf.User.RecoveryKey,
		HomeServer:       conf.HomeServer,
		HomeServerDomain: conf.HomeServerDomain,
	}

	client, err := mautrix.NewClient(
		user.HomeServer,
		id.NewUserID(user.Username, user.HomeServerDomain),
		user.AccessToken,
	)
	if err != nil {
		panic(err)
	}

	client.DeviceID = id.DeviceID(conf.User.DeviceId)
	cryptoHelper, err := SetupCryptoHelper(client)
	if err != nil {
		panic(err)
	}
	mc := MatrixClient{
		Client:       client,
		CryptoHelper: cryptoHelper,
	}

	mc.Client.Crypto = cryptoHelper

	fmt.Printf("[+] DeviceID: %s\n", mc.Client.DeviceID)

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
				panic(err)
			}
			fmt.Printf("%s\n", json)

			go (&Bridges{
				Client: client,
			}).ProcessIncomingMessages(evt)
		}
	}()

	err = mc.Sync(ch)
	if err != nil {
		panic(err)
	}
}

func (c *Controller) AddDevice(bridgeName string) error {
	bridge, err := (&Bridges{
		Client: c.Client,
	}).lookupBridgeByName(bridgeName)
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

	conf, err := cfg.getConf()
	if err != nil {
		return err
	}

	bridges := conf.Bridges

	for i, confBridge := range bridges {
		log.Printf("[+] (%d\\%d) Bridge: %s\n", i+1, len(bridges), confBridge.Name)

		//TODO: CheckRoomExists(client):
		bridge := Bridges{
			BridgeConfig: confBridge,
			Client:       c.Client,
		}
		if bridge.RoomID, err = bridge.JoinManagementRooms(); err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		} else {
			if err := bridge.Save(); err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return err
			}
		}
		log.Printf("Room created: %s\n", bridge.RoomID)
	}

	return nil

}

// !Danger if room already exist, this won't fail but would create a failed room
// !Have something that records all existing rooms into a db at start
func createContactRoom(room Rooms, bridgeName, contact, deviceId string) (*id.RoomID, error) {
	contactUsername, err := cfg.FormatUsername(bridgeName, contact)
	deviceIdUsername, err := cfg.FormatUsername(bridgeName, deviceId)
	slog.Debug("Contactusername: " + contactUsername)
	slog.Debug("Deviceusername: " + deviceIdUsername)

	bridge, err := (&Bridges{
		Client: room.Client,
	}).lookupBridgeByName(bridgeName)
	if err != nil {
		return nil, err
	}

	botUsername := bridge.BridgeConfig.BotName
	slog.Debug("Botusername: " + botUsername)

	roomId, err := (&Rooms{
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
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return &roomId, nil

}

func (c *Controller) SendMessage(bridgeName, deviceId, contact, message string) (*id.RoomID, error) {
	contact = strings.ReplaceAll(contact, "+", "")
	deviceId = strings.ReplaceAll(deviceId, "+", "")

	room := Rooms{Client: c.Client}
	_, err := room.FetchMessageContact(
		deviceId,
		bridgeName,
		contact,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	// fmt.Printf("Exists: %v\n", bridge)

	if room.ID == nil {
		slog.Debug("Creating contact room!")
		roomId, err := createContactRoom(room, bridgeName, contact, deviceId)
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
