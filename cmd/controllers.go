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
var syncMutex sync.Mutex

var dbChangeWatchers = make(map[string]func() (bool, error))
var syncQueue = make(map[string]bool)

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

	// err = c.AddBridges()
	// if err != nil {
	// 	return err
	// }

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

func onDatabaseChangeDaemon() error {
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
				if event.Op == fsnotify.Write {
					if strings.HasSuffix(event.Name, "clients.db") {
						go syncAll("onDatabaseChangeDaemon")
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

func syncAll(source string) error {
	slog.Debug("Syncing all users", "source", source)
	fetchedUsers, err := users.FetchAllUsers()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	// slog.Debug("Syncing All", "#users", len(fetchedUsers))

	for _, user := range fetchedUsers {
		err := syncWatcher.Add(user)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
	}
	return nil
}

func BootupSyncUsers() error {
	syncWatcher = SyncWatcher{
		cache:    make([]id.UserID, 0),
		wg:       &sync.WaitGroup{},
		syncUser: Sync,
	}

	syncAll("SyncUsers")
	go onDatabaseChangeDaemon()

	syncWatcher.wg.Wait()
	slog.Debug("Syncing details", "status", "completed and exiting")
	return nil
}

func (c *Controller) AddDevice(bridgeName string) error {
	bridgeCfg, err := configs.GetBridgeConfig(bridgeName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = bridges.AddDevice(c.Client, bridgeCfg)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	return nil
}

func addBridge(client *mautrix.Client, bridgeConf configs.BridgeConfig) error {
	_, err := bridges.JoinManagementRooms(client, &bridgeConf)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
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
		err := addBridge(c.Client, confBridge)
		if err != nil {
			return err
		}
	}

	return nil

}

func findTopicRooms(client *mautrix.Client, identifier string, deviceId *id.UserID) (*id.RoomID, error) {
	resp, err := client.JoinedRooms(context.Background())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	joinedRooms := resp.JoinedRooms
	slog.Debug("search info", "#rooms", len(joinedRooms))

	var roomId *id.RoomID
	var wg sync.WaitGroup
	wg.Add(len(joinedRooms))

	for _, room := range joinedRooms {
		go func(room *id.RoomID) {
			if roomId != nil {
				return
			}
			defer wg.Done()
			resp, err := client.JoinedMembers(context.Background(), *room)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			members := resp.Joined
			if len(members) < 3 || len(members) > 5 {
				return
			}

			if _, ok := members[*deviceId]; !ok {
				return
			}

			topic, err := rooms.GetRoomTopic(client, room)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			extracted := utils.ExtractE164Contact(topic)
			if len(extracted) < 1 {
				return
			}

			if extracted == identifier {
				roomId = room
				slog.Debug("Topic room found", "roomId", roomId)
			}
		}(&room)
	}
	wg.Wait()
	return roomId, nil
}

func findContactRooms(client *mautrix.Client, identifier, deviceId *id.UserID) (*id.RoomID, error) {
	resp, err := client.JoinedRooms(context.Background())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	joinedRooms := resp.JoinedRooms

	var roomId *id.RoomID
	var wg *sync.WaitGroup
	wg.Add(len(joinedRooms))
	for _, room := range joinedRooms {
		go func(room *id.RoomID) {
			if roomId != nil {
				return
			}

			defer wg.Done()

			resp, err := client.JoinedMembers(context.Background(), *room)
			if err != nil {
				slog.Error(err.Error())
				return
			}

			if _, ok := resp.Joined[*deviceId]; !ok {
				return
			}

			if _, ok := resp.Joined[*identifier]; !ok {
				return
			}

			if len(resp.Joined) < 3 || len(resp.Joined) > 4 {
				return
			}
			roomId = room
		}(&room)
	}
	wg.Wait()
	return roomId, nil
}

func (c *Controller) SendMessage(bridgeName, deviceId, receiver, message string) (*id.RoomID, error) {
	slog.Debug("[+] Sending message", "bridgeName", bridgeName, "deviceId", deviceId, "receiver", receiver)

	bridgeCfg, err := configs.GetBridgeConfig(bridgeName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	identifier, err := configs.FormatUsername(bridgeName, receiver)
	if err != nil && err != sql.ErrNoRows {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	var roomId *id.RoomID
	deviceIdTemplate := strings.ReplaceAll(bridgeCfg.UsernameTemplate, "{{.}}", deviceId)
	formattedDeviceId := id.NewUserID(deviceIdTemplate, c.Client.UserID.Homeserver())
	if bridgeCfg.AddressInTopic {
		_roomId, err := findTopicRooms(c.Client, receiver, &formattedDeviceId)
		if err != nil && err != sql.ErrNoRows {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		roomId = _roomId
	} else {
		_roomId, err := findContactRooms(c.Client, (*id.UserID)(identifier), &formattedDeviceId)
		if err != nil && err != sql.ErrNoRows {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		roomId = _roomId
	}

	if roomId == nil {
		var waitDb sync.WaitGroup
		waitDb.Add(1)

		callback := func() (bool, error) {
			return func(wg *sync.WaitGroup) (bool, error) {
				if bridgeCfg.AddressInTopic {
					_roomId, err := findTopicRooms(c.Client, receiver, &formattedDeviceId)
					if err != nil && err != sql.ErrNoRows {
						slog.Error(err.Error())
						debug.PrintStack()
						return false, err
					}
					roomId = _roomId
				} else {
					_roomId, err := findContactRooms(c.Client, (*id.UserID)(identifier), &formattedDeviceId)
					if err != nil && err != sql.ErrNoRows {
						slog.Error(err.Error())
						debug.PrintStack()
						return false, err
					}
					roomId = _roomId
				}

				if roomId == nil {
					return false, nil
				}
				slog.Debug("Db watcher", "found room", roomId)

				wg.Done()
				return true, nil
			}(&waitDb)
		}

		dbChangeWatchers[receiver] = callback

		err = bridges.StartConversation(c.Client, bridgeCfg, deviceId, receiver)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}

		waitDb.Wait()
	}

	if roomId == nil {
		err := fmt.Errorf("Room empty for conversation, this is wrong! clientID=%s", c.Client.UserID)
		slog.Error(err.Error())
		return nil, err
	}

	err = rooms.SendMessage(c.Client, *roomId, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return roomId, nil
}

func ParseRoomSubroutine(client *mautrix.Client, shouldRepair bool, roomId *id.RoomID) error {
	err := repairBridges(client)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	return nil
}

func repairBridges(client *mautrix.Client) error {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	for _, bridgeConf := range conf.Bridges {
		roomId, err := bridges.GetBotManagementRoom(client, (*id.UserID)(&bridgeConf.BotName))
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}

		if roomId == nil {
			slog.Debug("[+] Repairing bridge", "name", bridgeConf.Name)
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
