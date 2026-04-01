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
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/contacts"
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
				// if event.Op == fsnotify.Create || event.Op == fsnotify.Remove {
				// 	// slog.Debug("Client Watcher", "event", event.Op.String(), "filename", event.Name)
				// 	if strings.HasSuffix(event.Name, ".db") {
				// 		syncQueue[event.Name] = false
				// 	}
				// }
				if event.Op == fsnotify.Write {
					if strings.HasSuffix(event.Name, "clients.db") {
						go syncAll("onDatabaseChangeDaemon")
					} else if strings.HasSuffix(event.Name, ".db") {
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

func (c *Controller) SendMessage(bridgeName, deviceId, receiver, message string) (*id.RoomID, error) {
	bridgeCfg, err := configs.GetBridgeConfig(bridgeName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	contact, err := (&contacts.Contacts{
		DbFilename: string(c.Client.UserID)}).Find(receiver)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if contact == nil {
		var waitDb sync.WaitGroup
		waitDb.Add(1)

		callback := func() (bool, error) {
			return func(wg *sync.WaitGroup) (bool, error) {
				contact, err := (&contacts.Contacts{
					DbFilename: string(c.Client.UserID)}).Find(receiver)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return false, err
				}

				if contact == nil {
					return false, nil
				}

				wg.Done()
				time.Sleep(3 * time.Second)
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

	roomId, err := (&rooms.Rooms{
		Client:     c.Client,
		DbFilename: c.Client.UserID.String(),
	}).FindConversationRoom(*contact.Username, id.UserID(bridgeCfg.BotName))
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	if roomId == nil {
		return nil, fmt.Errorf("Room empty for conversation, this is wrong! clientID=%s", c.Client.UserID)
	}

	err = rooms.SendMessage(c.Client, *roomId, message)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return roomId, nil
}

func parseRoom(client *mautrix.Client, roomId *id.RoomID) error {
	slog.Debug("* Parsing room", "room_id", roomId)
	members, err := client.JoinedMembers(context.Background(), *roomId)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	room := rooms.Rooms{
		DbFilename: client.UserID.String(),
	}
	for member, _ := range members.Joined {
		err := room.Save(*roomId, member)
		if err != nil {
			debug.PrintStack()
			return err
		}
	}
	return nil
}

func ParseRoomSubroutine(client *mautrix.Client, shouldRepair bool, roomId *id.RoomID) error {
	ctx := context.Background()

	if roomId != nil {
		err := parseRoom(client, roomId)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
		return nil
	} else {
		joinedRooms, err := client.JoinedRooms(ctx)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}

		for _, roomId := range joinedRooms.JoinedRooms {
			parseRoom(client, &roomId)
		}
	}

	if shouldRepair {
		err := repairBridges(client)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
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
		if err != nil && err != sql.ErrNoRows {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
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
