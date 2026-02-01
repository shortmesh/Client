package bridges

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strings"

	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Bridges struct {
	BridgeConfig configs.BridgeConfig
	RoomID       *id.RoomID
	Client       *mautrix.Client
}

func (b *Bridges) ProcessIncomingMessages(evt *event.Event) error {
	bridge, err := b.LookupBridgeByRoomId(evt.RoomID.String())
	if err != nil || bridge == nil {
		if bridge == nil {
			log.Printf("\n- Couldn't find bridge to publish incoming event for room: %s\n", evt.RoomID)
		}
		return err
	}
	log.Println("[+] New notice for bridge", evt.RoomID, evt.Sender, evt.Timestamp, evt.Type)

	conf, err := configs.GetConf()
	if err != nil {
		return nil
	}

	for _, confBridge := range conf.Bridges {
		if confBridge.Name == bridge.BridgeConfig.Name {
			log.Printf("[+] Found bridge in configs!\n")
			b.BridgeConfig = confBridge
		}
	}

	log.Printf("[+] Checking for bridge with name: %s\n", bridge.BridgeConfig.Name)

	if evt.Sender != b.Client.UserID {
		regexPattern := strings.ReplaceAll(b.BridgeConfig.Cmd["success"], "%s", ".*")
		matched, err := regexp.MatchString(regexPattern, evt.Content.AsMessage().Body)
		if err != nil {
			return err
		}

		if matched {
			// TODO: something important with the new devices added
			// Successfully logged in as +123456789
			log.Println("[+] Device added successfully....")
		}
	}
	return nil
}

func (b *Bridges) LookupBridgeByName(name string) (*Bridges, error) {
	roomsDb := rooms.GetRoomDb(b.Client)

	if err := roomsDb.Init(); err != nil {
		log.Println("Error initializing client db:", err)
		return nil, err
	}

	bridgeRoomIds, err := roomsDb.FetchRoomByName(name)
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
	roomsDb := rooms.GetRoomDb(b.Client)

	if err := roomsDb.Init(); err != nil {
		log.Println("Error initializing client db:", err)
		return nil, err
	}

	bridgeName, err := roomsDb.FetchRoomByRoomId(roomId)
	if err != nil {
		return nil, err
	}

	return (&Bridges{
		BridgeConfig: configs.BridgeConfig{
			Name: bridgeName,
		},
	}), nil
}

func (b *Bridges) queryCommand(query string) error {
	log.Printf("[+] %sBridge| Sending message %s to %v\n", b.BridgeConfig.Name, query, b.RoomID)
	_, err := b.Client.SendText(
		context.Background(),
		*b.RoomID,
		query,
	)

	if err != nil {
		log.Println("Error sending message:", err)
		return err
	}
	return nil
}

func (b *Bridges) checkActiveSessions() (bool, error) {
	var UsersDB = users.UsersDB{
		Username: b.Client.UserID.Localpart(),
		Filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	}

	if err := UsersDB.Init(); err != nil {
		return false, err
	}

	activeSessions, _, err := UsersDB.FetchActiveSessions(b.Client.UserID.Localpart())
	if err != nil {
		return false, err
	}

	if len(activeSessions) == 0 {
		return false, nil
	}

	return true, nil
}

func (b *Bridges) AddDevice() error {
	// var UsersDB = UsersDB{
	// 	username: b.Client.UserID.Localpart(),
	// 	filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	// }

	// if err := UsersDB.Init(); err != nil {
	// 	return err
	// }

	// TODO:
	// activeSessions, err := b.checkActiveSessions()
	// if err != nil {
	// 	log.Println("Failed checking active sessions", err)
	// 	return err
	// }

	if err := b.queryCommand(b.BridgeConfig.Cmd["login"]); err != nil {
		return err
	}

	return nil
}

func (b *Bridges) JoinManagementRooms() (id.RoomID, error) {
	roomId, err := (&rooms.Rooms{
		Client:   b.Client,
		IsBridge: true,
	}).JoinRoom([]id.UserID{
		id.UserID(b.BridgeConfig.BotName),
	})
	if err != nil {
		return "", err
	}

	b.RoomID = &roomId
	return roomId, nil
}

func (b *Bridges) CreateContactRooms() error {
	roomsDb := rooms.GetRoomDb(b.Client)
	if err := roomsDb.Init(); err != nil {
		log.Println("Error initializing client db:", err)
		return err
	}

	cfg, err := configs.GetConf()
	if err != nil {
		return err
	}

	eventSubName := configs.ReverseAliasForEventSubscriber(
		b.Client.UserID.Localpart(),
		b.BridgeConfig.Name,
		cfg.HomeServerDomain,
	)
	eventSubName = eventSubName + "+join"

	return nil
}

func (b *Bridges) Save() error {
	roomsDb := rooms.GetRoomDb(b.Client)

	if err := roomsDb.Init(); err != nil {
		log.Println("Error initializing client db:", err)
		return err
	}

	// TODO: put device id and other params here
	if err := roomsDb.StoreRoom(b.RoomID.String(), b.BridgeConfig.Name, "", "", true); err != nil {
		return err
	}

	return nil
}
