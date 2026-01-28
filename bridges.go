package main

import (
	"context"
	"log"
	"regexp"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Bridges struct {
	BridgeConfig BridgeConfig
	RoomID       id.RoomID
	Client       *mautrix.Client
}

// func (b *Bridges) ProcessIncomingLoginDaemon(bridgeCfg *BridgeConfig) {
// 	log.Println("Processing incoming login daemon for:", b.Name)
// 	var clientDb = ClientDB{
// 		username: b.Client.UserID.Localpart(),
// 		filepath: "db/" + b.Client.UserID.Localpart() + ".db",
// 	}

// 	if err := clientDb.Init(); err != nil {
// 		log.Println("Error initializing client db:", err)
// 		return
// 	}

// 	eventSubName := ReverseAliasForEventSubscriber(b.Client.UserID.Localpart(), b.Name, cfg.HomeServerDomain) + "+loginDaemon"
// 	eventSubscriber := EventSubscriber{
// 		Name:    eventSubName,
// 		MsgType: nil,
// 		ExcludeMsgTypes: []event.MessageType{
// 			event.MsgText,
// 		},
// 		RoomID: b.RoomID,
// 		Callback: func(evt *event.Event) {
// 			log.Println("Received event in login:", evt.RoomID, evt.Sender, evt.Timestamp, evt.Type)
// 			if evt.Sender != b.Client.UserID && evt.Type == event.EventMessage {
// 				failedCmd := bridgeCfg.Cmd["failed"]

// 				matchesSuccess, err := cfg.CheckSuccessPattern(b.Name, evt.Content.AsMessage().Body)

// 				if err != nil {
// 					clientDb.RemoveActiveSessions(b.Client.UserID.Localpart())
// 				}

// 				if evt.Content.Raw["msgtype"] == "m.notice" {
// 					if strings.Contains(evt.Content.AsMessage().Body, failedCmd) || matchesSuccess {
// 						clientDb.RemoveActiveSessions(b.Client.UserID.Localpart())
// 					}
// 				}

// 				if evt.Content.AsMessage().MsgType.IsMedia() {
// 					url := evt.Content.AsMessage().URL
// 					file, err := ParseImage(b.Client, string(url))
// 					if err != nil {
// 						log.Println("Error parsing image:", err)
// 						clientDb.RemoveActiveSessions(b.Client.UserID.Localpart())
// 					}

// 					// return file, nil
// 					clientDb.StoreActiveSessions(b.Client.UserID.Localpart(), file)
// 				}
// 			}

// 			// defer func() {
// 			// 	for index, subscriber := range EventSubscribers {
// 			// 		if subscriber.Name == eventSubName {
// 			// 			EventSubscribers = append(EventSubscribers[:index], EventSubscribers[index+1:]...)
// 			// 			break
// 			// 		}
// 			// 	}
// 			// }()
// 		},
// 	}
// 	EventSubscribers = append(EventSubscribers, eventSubscriber)
// }

func (b *Bridges) ProcessIncomingMessages(evt *event.Event) error {
	log.Println("[+] New notice for bridge", evt.RoomID, evt.Sender, evt.Timestamp, evt.Type)
	bridge, err := b.lookupBridge(evt.RoomID.String())
	if err != nil || bridge == nil {
		if bridge == nil {
			log.Printf("\n- Couldn't find bridge to publish incoming event for room: %s\n", evt.RoomID)
		}
		return err
	}

	log.Printf("[+] Checking for bridge with name: %s\n", bridge.BridgeConfig.Name)
	conf, err := cfg.getConf()
	if err != nil {
		return err
	}

	for _, confBridge := range conf.Bridges {
		if confBridge.Name == bridge.BridgeConfig.Name {
			log.Printf("[+] Found bridge in configs!\n")
			b.BridgeConfig = confBridge
		}
	}

	if evt.Sender != b.Client.UserID {
		regexPattern := strings.ReplaceAll(b.BridgeConfig.Cmd["success"], "%s", ".*")
		matched, err := regexp.MatchString(regexPattern, evt.Content.AsMessage().Body)
		if err != nil {
			return err
		}

		if matched {
			log.Println("[+] Device added successfully....")
		}
	}
	return nil
}

func (b *Bridges) lookupBridge(roomId string) (*Bridges, error) {
	//TODO: should check for which device is making this call, user can have multiple
	var clientDb = ClientDB{
		username: b.Client.UserID.Localpart(),
		filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	}

	if err := clientDb.Init(); err != nil {
		log.Println("Error initializing client db:", err)
		return nil, err
	}

	bridge, err := clientDb.FetchByRoomID(roomId)
	if err != nil {
		return nil, err
	}
	return bridge, nil
}

func (b *Bridges) queryCommand(query string) error {
	log.Printf("[+] %sBridge| Sending message %s to %v\n", b.BridgeConfig.Name, query, b.RoomID)
	_, err := b.Client.SendText(
		context.Background(),
		b.RoomID,
		query,
	)

	if err != nil {
		log.Println("Error sending message:", err)
		return err
	}
	return nil
}

func (b *Bridges) checkActiveSessions() (bool, error) {
	var clientDb = ClientDB{
		username: b.Client.UserID.Localpart(),
		filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	}

	if err := clientDb.Init(); err != nil {
		return false, err
	}

	activeSessions, _, err := clientDb.FetchActiveSessions(b.Client.UserID.Localpart())
	if err != nil {
		return false, err
	}

	if len(activeSessions) == 0 {
		return false, nil
	}

	return true, nil
}

func (b *Bridges) AddDevice(ch *chan []byte) error {
	// var clientDb = ClientDB{
	// 	username: b.Client.UserID.Localpart(),
	// 	filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	// }

	// if err := clientDb.Init(); err != nil {
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
	// joinedRooms, err := b.Client.JoinedRooms(context.Background())
	// log.Println("Joined rooms:", joinedRooms)

	// if err != nil {
	// 	return err
	// }

	// var clientDb = ClientDB{
	// 	username: b.Client.UserID.Localpart(),
	// 	filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	// }
	// clientDb.Init()

	// for _, room := range joinedRooms.JoinedRooms {
	// 	room := Rooms{
	// 		Client: b.Client,
	// 		ID:     room,
	// 	}

	// 	isManagementRoom, err := room.IsManagementRoom(b.BotName)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	log.Println("Is management room:", room.ID, isManagementRoom)

	// 	if isManagementRoom {
	// 		b.RoomID = room.ID
	// 		break
	// 	}
	// }

	log.Println("[+] Creating management room for:", b.BridgeConfig.Name)
	resp, err := b.Client.CreateRoom(context.Background(), &mautrix.ReqCreateRoom{
		Invite:   []id.UserID{id.UserID(b.BridgeConfig.BotName)},
		IsDirect: true,
		// Preset:     "private_chat",
		Preset:     "trusted_private_chat",
		Visibility: "private",
	})
	if err != nil {
		return "", err
	}

	// * Begins encryption
	_, err = b.Client.SendStateEvent(
		context.Background(),
		resp.RoomID,
		event.StateEncryption,
		"",
		&event.EncryptionEventContent{
			Algorithm: id.AlgorithmMegolmV1, // "m.megolm.v1.aes-sha2"
		},
	)

	if err != nil {
		return "", err
	}

	b.RoomID = resp.RoomID
	return resp.RoomID, nil
}

// func (b *Bridges) ListDevices() ([]string, error) {
// 	log.Println("Listing devices for:", b.Name, b.RoomID)
// 	ch := make(chan []string)
// 	eventSubName := ReverseAliasForEventSubscriber(b.Client.UserID.Localpart(), b.Name, cfg.HomeServerDomain) + "+devices"
// 	eventType := event.MsgNotice
// 	eventSince := time.Now().UTC()
// 	eventSubscriber := EventSubscriber{
// 		Name:    eventSubName,
// 		MsgType: &eventType,
// 		Since:   &eventSince,
// 		RoomID:  b.RoomID,
// 		Callback: func(event *event.Event) {
// 			devicesRaw := strings.Split(event.Content.AsMessage().Body, "\n")
// 			devices := make([]string, 0)
// 			for _, device := range devicesRaw {
// 				deviceName, err := ExtractBracketContent(device)
// 				if err != nil {
// 					log.Println("Failed extracting device name", err, device)
// 					continue
// 				}
// 				devices = append(devices, deviceName)
// 			}
// 			ch <- devices

// 			defer func() {
// 				for index, subscriber := range EventSubscribers {
// 					if subscriber.Name == eventSubName {
// 						EventSubscribers = append(EventSubscribers[:index], EventSubscribers[index+1:]...)
// 						break
// 					}
// 				}
// 			}()
// 		},
// 	}

// 	EventSubscribers = append(EventSubscribers, eventSubscriber)

// 	bridgeCfg, ok := cfg.GetBridgeConfig(b.Name)
// 	if !ok {
// 		return nil, fmt.Errorf("bridge config not found for: %s", b.Name)
// 	}
// 	log.Println("Event subscriber name:", eventSubName)

// 	_, err := b.Client.SendText(
// 		context.Background(),
// 		b.RoomID,
// 		bridgeCfg.Cmd["devices"],
// 	)

// 	if err != nil {
// 		return nil, err
// 	}

// 	devices := <-ch

// 	return devices, nil
// }

func (b *Bridges) CreateContactRooms() error {
	log.Println("Joining member rooms for:", b.BridgeConfig.Name)

	clientDb := ClientDB{
		username: b.Client.UserID.Localpart(),
		filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	}
	clientDb.Init()

	eventSubName := ReverseAliasForEventSubscriber(b.Client.UserID.Localpart(), b.BridgeConfig.Name, cfg.HomeServerDomain)
	eventSubName = eventSubName + "+join"

	processedRooms := make(map[id.RoomID]bool)

	eventSubscriber := EventSubscriber{
		Name:    eventSubName,
		MsgType: nil,
		ExcludeMsgTypes: []event.MessageType{
			event.MsgNotice, event.MsgVerificationRequest, event.MsgLocation,
		},
		Callback: func(evt *event.Event) {
			// log.Println("Received event:", event.RoomID, event.Content.AsMessage().Body)
			if evt.RoomID != "" {
				room := Rooms{
					Client: b.Client,
					ID:     evt.RoomID,
				}

				if _, ok := processedRooms[evt.RoomID]; ok {
					return
				}

				processedRooms[evt.RoomID] = true

				powerLevels, err := room.GetPowerLevelsUser()
				if err != nil {
					log.Println("Failed getting power levels", err)
					return
				}
				log.Println("Power levels:", powerLevels)
				powerLevelsEvents, err := room.GetPowerLevelsEvents()
				if err != nil {
					log.Println("Failed getting power levels events", err)
					return
				}
				log.Println("Power levels events:", powerLevelsEvents)

				isManagementRoom, err := room.IsManagementRoom(b.BridgeConfig.BotName)
				if err != nil {
					log.Println("Failed checking if room is management room", err)
					return
				}
				log.Println("Is management room:", evt.RoomID, isManagementRoom)
				processedRooms[evt.RoomID] = true

				if !isManagementRoom {
					members, err := room.GetRoomMembers(b.Client, evt.RoomID)
					if err != nil {
						log.Println("Failed getting room members", err)
						return
					}

					foundDevice := false
					foundMembers := make([]string, 0)
					foundDeviceUserName := ""

					for _, member := range members {
						log.Println("Checking member:", member.String())
						matched, err := cfg.CheckUsernameTemplate(b.BridgeConfig.BotName, member.String())
						if err != nil {
							log.Println("Failed checking username template", err)
							return
						}
						if !matched {
							continue
						}

						devices := ClientDevices[b.Client.UserID.Localpart()][b.BridgeConfig.Name]
						log.Println("Devices:", devices)

						for _, device := range devices {
							formattedUsername, err := cfg.FormatUsername(b.BridgeConfig.Name, device)
							if err != nil {
								log.Println("Failed formatting username", err, device)
								continue
							}
							if member.String() == formattedUsername {
								foundDevice = true
								foundDeviceUserName = formattedUsername
								log.Println("Found device:", foundDeviceUserName)
								break
							} else {
								foundMembers = append(foundMembers, member.String())
							}
						}
					}

					if foundDevice && len(foundMembers) == 0 {
						log.Println("Found device but no members, adding device to members", foundDeviceUserName)
						foundMembers = append(foundMembers, foundDeviceUserName)
					}

					if foundDevice && len(foundMembers) > 0 {
						for _, fMember := range foundMembers {
							clientDb.StoreRooms(evt.RoomID.String(), b.BridgeConfig.Name, foundDeviceUserName, fMember, false)
							// log.Println("Stored room:", event.RoomID.String(), b.Name, fMember, false, foundDeviceUserName)
						}
					}
				}
			}
		},
	}

	EventSubscribers = append(EventSubscribers, eventSubscriber)

	return nil
}

func (b *Bridges) GetRoomInvitesDaemon() error {
	log.Println("Getting room invites for:", b.BridgeConfig.Name, b.RoomID)

	resp, err := b.Client.SyncRequest(context.Background(), 30000, "", "", true, event.PresenceOnline)
	if err != nil {
		log.Fatal(err)
	}

	for roomID := range resp.Rooms.Invite {
		log.Printf("You have been invited to room: %s\n", roomID)
		_, err := b.Client.JoinRoomByID(context.Background(), roomID)
		if err != nil {
			log.Println("Failed joining room", err)
		}
	}

	eventSubName := ReverseAliasForEventSubscriber(b.Client.UserID.Localpart(), b.BridgeConfig.Name, cfg.HomeServerDomain) + "+invites"
	eventSubscriber := EventSubscriber{
		Name:    eventSubName,
		MsgType: nil,
		Callback: func(evt *event.Event) {
			// log.Println("Received event:", evt.RoomID, evt.Content.AsMember())
			room := Rooms{
				Client: b.Client,
				ID:     evt.RoomID,
			}
			room.GetInvites(evt)
		},
	}

	EventSubscribers = append(EventSubscribers, eventSubscriber)

	return nil
}

func (b *Bridges) Save() error {
	var clientDb = ClientDB{
		username: b.Client.UserID.Localpart(),
		filepath: "db/" + b.Client.UserID.Localpart() + ".db",
	}

	if err := clientDb.Init(); err != nil {
		log.Println("Error initializing client db:", err)
		return err
	}

	// TODO: put device id and other params here
	if err := clientDb.StoreBridge(b.RoomID.String(), b.BridgeConfig.Name, "", ""); err != nil {
		return err
	}

	return nil
}
