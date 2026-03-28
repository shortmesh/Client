package cmd

import (
	// 	"context"
	// 	"fmt"

	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type Controller struct {
	Client *mautrix.Client
}

var mutex sync.Mutex
var dbChangeWatchers = make(map[string]func() (bool, error))

func (c *Controller) GetDevices() ([]devices.Devices, error) {
	devices, err := (&devices.Devices{Client: c.Client}).GetDevices()
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	return devices, nil
}

func (c *Controller) Store() error {
	user, err := users.FetchUser(c.Client)
	if err != nil {
		return err
	}

	if user.Client == nil { // changing access token
		pickleKey, err := utils.GenerateRandomBytes(32)
		cryptoHelper, err := SetupCryptoHelper(c.Client, pickleKey)
		if err != nil {
			return err
		}

		recoveryKey, err := GenerateAndUploadClientKeys(cryptoHelper)
		if err != nil {
			return err
		}
		user.RecoveryKey = recoveryKey
		user.PickleKey = pickleKey
		user.Client = c.Client
	}

	err = user.Save()
	if err != nil {
		return err
	}

	err = c.AddBridges()
	if err != nil {
		return err
	}

	return nil
}

// !This should be used if account reset is on the table
func (c *Controller) Login(password string) (string, error) {
	mc := &MatrixClient{Client: c.Client}
	err := mc.Login(password)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}
	pickleKey, err := utils.GenerateRandomBytes(32)
	slog.Debug("authenticating",
		"deviceId", c.Client.DeviceID,
		"accessToken", c.Client.AccessToken,
		"password", password,
		"pickleKey", pickleKey,
	)

	err = utils.DeleteFilesWithPattern("./db", fmt.Sprintf("%s-crypto.*", c.Client.UserID.Localpart()))
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	cryptoHelper, err := SetupCryptoHelper(c.Client, pickleKey)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	recoveryKey, err := GenerateAndUploadClientKeys(cryptoHelper)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	err = (&users.Users{
		Client:      c.Client,
		RecoveryKey: recoveryKey,
		PickleKey:   pickleKey,
	}).Save()
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}
	slog.Debug("Saved user", "username", c.Client.UserID)
	return recoveryKey, nil
}

func clientsDbWatcher() error {
	slog.Debug("Client Watcher", "status", "initialized")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op == fsnotify.Create || event.Op == fsnotify.Remove {
					// slog.Debug("Client Watcher", "event", event.Op.String(), "filename", event.Name)
					if strings.HasSuffix(event.Name, ".db") {
						go syncAll()
					}
				}
				if event.Op == fsnotify.Write {
					if strings.HasSuffix(event.Name, ".db") {
						mutex.Lock()
						slog.Debug("Client Watcher", "event", event.Op.String(), "filename", event.Name)

						var deleteCache []string
						for key, callback := range dbChangeWatchers {
							slog.Debug("Client Watcher Iterating", "name", key)
							ok, err := callback()
							if err != nil {
								slog.Error(err.Error())
								debug.PrintStack()
							}
							if ok {
								deleteCache = append(deleteCache, key)
							}
						}
						for _, key := range deleteCache {
							delete(dbChangeWatchers, key)
						}
						mutex.Unlock()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error(err.Error())
				debug.PrintStack()
			}
		}
	}()

	// Add a path to watch (e.g., a directory)
	err = watcher.Add("./db") // Use your desired path
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	// Keep the program running
	<-done
	return nil
}

func syncAll() error {
	fetchedUsers, err := users.FetchAllUsers()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	slog.Debug("Syncing All", "#users", len(fetchedUsers))

	for _, user := range fetchedUsers {
		err := syncWatcher.Add(user)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
	}
	return nil
}

func SyncUsers() error {
	syncWatcher = SyncWatcher{
		cache:    make([]id.UserID, 0),
		wg:       &sync.WaitGroup{},
		syncUser: Sync,
	}

	syncAll()
	go clientsDbWatcher()

	syncWatcher.wg.Wait()
	slog.Debug("Syncing details", "status", "completed and exiting")
	return nil
}

func (c *Controller) AddDevice(bridgeName string) error {
	bridge, err := bridges.LookupBridgeByName(c.Client, bridgeName)
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

func addBridge(client *mautrix.Client, bridgeConf configs.BridgeConfig) error {
	slog.Debug("Adding bridge", "name", bridgeConf.Name)

	bridge := bridges.Bridges{
		BridgeConfig: bridgeConf,
		Client:       client,
	}
	roomId, err := bridge.JoinManagementRooms()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	bridge.RoomID = &roomId

	if err := bridge.Clear(); err != nil {
		return err
	}
	slog.Debug("Bridge rooms cleared", "name", bridge.BridgeConfig.BotName)

	if err := bridge.Save(); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	slog.Debug("Room created", "room_id", bridge.RoomID)
	return nil
}

func (c *Controller) AddBridges() error {
	conf, err := configs.GetConf()
	if err != nil {
		return err
	}

	bridgeConfs := conf.Bridges

	for _, confBridge := range bridgeConfs {
		err := addBridge(c.Client, confBridge)
		if err != nil {
			return err
		}
	}

	return nil

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

	var roomId id.RoomID
	room := rooms.Rooms{
		Client: c.Client,
		ID:     nil,
	}

	var waitDb sync.WaitGroup
	if roomIdStr == nil {
		waitDb.Add(1)

		callback := func() (bool, error) {
			return func(wg *sync.WaitGroup) (bool, error) {
				roomIdStr, err := users.FetchMessageContactOnly(
					c.Client,
					bridgeName,
					contactUsername,
				)

				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return false, err
				}

				slog.Debug("Db watcher called", "reason",
					"possibly new contact room created",
					"roomId", roomIdStr,
				)

				if roomIdStr == "" {
					return false, nil
				}

				roomId = id.RoomID(roomIdStr)

				wg.Done()
				return true, nil
			}(&waitDb)
		}

		dbChangeWatchers[contact] = callback

		bridge, err := bridges.LookupBridgeByName(c.Client, bridgeName)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}

		err = bridge.StartConversation(deviceId, contact)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}

		waitDb.Wait()
		slog.Debug("SendMessage", "status", "callback should be finished")
	} else {
		roomId = id.RoomID(*roomIdStr)
	}

	room.ID = &roomId

	err = room.SendMessage(message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return room.ID, nil
}

func ParseRoomSubroutine(client *mautrix.Client) error {
	ctx := context.Background()
	joinedRooms, err := client.JoinedRooms(ctx)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	slog.Debug("User details", "num_rooms", len(joinedRooms.JoinedRooms))

	for _, roomId := range joinedRooms.JoinedRooms {
		room := rooms.Rooms{
			Client: client,
			ID:     &roomId,
		}
		members, err := room.GetRoomMembers()

		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
		// slog.Debug("User details", "members_in_room", len(members))

		isContactRoom, err := rooms.IsContactRoom(client, members)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}

		if isContactRoom {
			slog.Debug("Room details", "room", roomId, "isContactRoom", isContactRoom)
			err := rooms.ProcessIsContactRoom(client, room, members)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
			}
		}
	}

	err = repairBridges(client)
	if err != nil {
		slog.Error(err.Error())
	}

	return nil
}

func repairBridges(client *mautrix.Client) error {
	slog.Debug("[+] Repairing bridges...")
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	for _, bridgeConf := range conf.Bridges {
		// slog.Debug("Checking bridge", "name", bridgeConf.Name)
		bridge, err := bridges.LookupBridgeByName(client, bridgeConf.Name)
		if err != nil && err != sql.ErrNoRows {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}

		if bridge == nil {
			slog.Debug("repairing bridge", "name", bridgeConf.Name)
			err = addBridge(client, bridgeConf)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return err
			}
		}
	}

	return nil
}

// !Danger if room already exist, this won't fail but would create a failed room
// !Have something that records all existing rooms into a db at start
func createContactRoom(room rooms.Rooms, bridgeName, contact, deviceId string) (*id.RoomID, error) {
	// cfg, err := configs.GetConf()
	// if err != nil {
	// 	slog.Error(err.Error())
	// 	debug.PrintStack()
	// 	return nil, err
	// }
	// contactUsername, err := cfg.FormatUsername(bridgeName, contact)
	// deviceIdUsername, err := cfg.FormatUsername(bridgeName, deviceId)
	// slog.Debug("Bridges", "contactusername", contactUsername, "deviceusername", deviceIdUsername)

	bridge, err := bridges.LookupBridgeByName(room.Client, bridgeName)
	if err != nil {
		return nil, err
	}

	botUsername := bridge.BridgeConfig.BotName
	slog.Debug("Bridges", "Botusername", botUsername)

	roomId, err := room.CreateRoom([]id.UserID{
		id.UserID(contact),
		id.UserID(deviceId),
		id.UserID(botUsername),
	}, false)

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
