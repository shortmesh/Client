package cmd

import (
	// 	"context"
	// 	"fmt"

	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/contacts"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/messages"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/syncers"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Controller struct {
	Client *mautrix.Client
}

var syncWatcher syncers.SyncWatcher

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
		cryptoHelper, err := syncers.SetupCryptoHelper(c.Client, pickleKey)
		if err != nil {
			return err
		}

		recoveryKey, err := syncers.GenerateAndUploadClientKeys(cryptoHelper)
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
	mc := &syncers.MatrixClient{Client: c.Client}
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

	cryptoHelper, err := syncers.SetupCryptoHelper(c.Client, pickleKey)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	recoveryKey, err := syncers.GenerateAndUploadClientKeys(cryptoHelper)
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
		syncers.RegisterSyncMessageListener(&syncers.SyncEventCallback{
			Callback: func(evt *event.Event) error {

				/**
				Bridges listener, responsible for outgoing messages
				**/
				go func() {
					err = bridges.SyncCallback(user.Client, evt)
					if err != nil {
						slog.Error(err.Error())
						debug.PrintStack()
					}
				}()

				/**
				Contact listener, responsible for incoming messages
				**/
				go func() {
					err = contacts.SyncCallback(user.Client, evt)
					if err != nil {
						slog.Error(err.Error())
						debug.PrintStack()
					}
				}()

				return nil
			},
			ID: user.Client.UserID.String(),
		})
		err := syncWatcher.Add(user)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
	}
	return nil
}

func BootupSyncUsers() error {
	syncWatcher = syncers.SyncWatcher{
		Cache:    make([]id.UserID, 0),
		Wg:       &sync.WaitGroup{},
		SyncUser: syncers.Sync,
	}

	syncAll("SyncUsers")
	go onDatabaseChangeDaemon()

	syncWatcher.Wg.Wait()
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

func (c *Controller) AddBridges() error {
	conf, err := configs.GetConf()
	if err != nil {
		return err
	}

	bridgeConfs := conf.Bridges

	for _, confBridge := range bridgeConfs {
		err := bridges.AddBridge(c.Client, confBridge)
		if err != nil {
			return err
		}
	}

	return nil

}

func (c *Controller) SendMessage(
	bridgeName,
	deviceId,
	receiver,
	message,
	fileExtension,
	fileContent string,
) (*id.RoomID, error) {
	slog.Debug("[+] Sending message", "bridgeName", bridgeName, "deviceId", deviceId, "receiver", receiver)

	bridgeCfg, err := configs.GetBridgeConfig(bridgeName)
	if err != nil && err != sql.ErrNoRows {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	roomId, err := noisyRoomIdRequest(
		c.Client,
		bridgeCfg,
		receiver,
		deviceId,
	)

	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	if roomId == nil {
		err := fmt.Errorf("Room empty for conversation, this is wrong! clientID=%s", c.Client.UserID)
		slog.Error(err.Error())
		return nil, err
	}

	if fileExtension != "" && fileContent != "" {
		filepath, err := createTmpFile(fileExtension, fileContent)
		defer deleteFile(filepath)

		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}

		err = messages.SendMediaMessage(c.Client, *roomId, *filepath, message)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
	} else {
		err = messages.SendMessage(c.Client, *roomId, message)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
	}

	return roomId, nil
}

func deleteFile(filePath *string) error {
	if filePath == nil {
		return nil // Or return an error if you expect it to always exist
	}

	err := os.Remove(*filePath)
	if err != nil {
		slog.Error("failed to delete file", "path", *filePath, "error", err)
		return err
	}

	slog.Info("file deleted successfully", "path", *filePath)
	return nil
}

func createTmpFile(fileExtension, fileContent string) (*string, error) {
	data, err := base64.StdEncoding.DecodeString(fileContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	mediaDir := "media"
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		slog.Error("failed to create media directory", "error", err)
		return nil, err
	}

	pattern := fmt.Sprintf("tmp-*.%s", fileExtension)
	tmpFile, err := os.CreateTemp(mediaDir, pattern)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name())
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	relativePath := filepath.Join(mediaDir, filepath.Base(tmpFile.Name()))

	return &relativePath, nil
}

func noisyRoomIdRequest(
	client *mautrix.Client,
	bridgeCfg *configs.BridgeConfig,
	receiver,
	deviceId string,
) (*id.RoomID, error) {
	var wg sync.WaitGroup
	var roomId *id.RoomID

	wg.Add(1)

	callbackEventId := client.UserID.String() + bridgeCfg.Name + receiver

	syncers.RegisterSyncMessageListener(&syncers.SyncEventCallback{
		ID:        callbackEventId,
		EventType: "m.room.message",
		Callback: func(evt *event.Event) error {
			slog.Debug("[+] SendMessage response received", "msg", evt.Content.AsMessage().Body)
			_roomId, err := isContactCallback(client, evt, &receiver)
			if err != nil {
				slog.Error(err.Error())
				return err
			}
			roomId = _roomId
			defer wg.Done()

			syncers.UnRegisterSyncMessageListener(callbackEventId)

			return nil
		},
	})

	err := bridges.StartConversation(client, bridgeCfg, deviceId, receiver)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	wg.Wait()
	return roomId, nil
}

func isContactCallback(client *mautrix.Client, evt *event.Event, receiver *string) (*id.RoomID, error) {
	contactUserIds := evt.Content.AsMessage().Mentions.UserIDs
	if len(contactUserIds) != 1 {
		slog.Debug("[+] SendMessage response received - false", "#ids", len(contactUserIds))
		return nil, fmt.Errorf("Not 1 contact found, found %d", len(contactUserIds))
	}
	err := contacts.CreateContact(client, *receiver, &contactUserIds[0])
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	roomId, err := rooms.ExtractMatrixRoomID(evt.Content.AsMessage().Body)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return roomId, nil
}
