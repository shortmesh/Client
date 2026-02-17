package cmd

import (
	// 	"context"
	// 	"fmt"

	"fmt"
	"log/slog"
	"runtime/debug"
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
				if event.Op == fsnotify.Create {
					slog.Debug("Client Watcher", "created file:", event.Name)
					go syncAll()
				}
				if event.Op == fsnotify.Remove {
					slog.Debug("Client Watcher", "removed file:", event.Name)
					go syncAll()
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

	slog.Debug("Syncing details", "#users", len(fetchedUsers))

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
	conf, err := configs.GetConf()
	if err != nil {
		return err
	}

	bridgeConfs := conf.Bridges

	for _, confBridge := range bridgeConfs {
		slog.Debug("Adding bridge", "name", confBridge.Name)

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

		if err := bridge.Clear(); err != nil {
			return err
		}
		slog.Debug("Bridge rooms cleared", "name", bridge.BridgeConfig.BotName)

		if err := bridge.Save(); err != nil {
			return err
		}

		slog.Debug("Room created", "room_id", bridge.RoomID)
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
	slog.Debug("Bridges", "contactusername", contactUsername, "deviceusername", deviceIdUsername)

	bridge, err := (&bridges.Bridges{
		Client: room.Client,
	}).LookupBridgeByName(bridgeName)
	if err != nil {
		return nil, err
	}

	botUsername := bridge.BridgeConfig.BotName
	slog.Debug("Bridges", "Botusername", botUsername)

	roomId, err := room.CreateRoom([]id.UserID{
		id.UserID(contactUsername),
		id.UserID(deviceIdUsername),
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

	if roomIdStr == nil {
		slog.Debug("Creating contact room!")
		_roomId, err := createContactRoom(room, bridgeName, contactUsername, deviceIdUsername)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		roomId = *_roomId
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
