package cmd

import (
	// 	"context"
	// 	"fmt"

	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
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

	ignoreBotMessage := false

	confBridges, err := configs.GetBridgeConfigs()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(len(confBridges))

	user, err := users.FetchUser(c.Client)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	for _, conf := range confBridges {
		slog.Debug("GetDevices", "devices", conf.Name)
		go func(bridgeCfg *configs.BridgeConfig) {
			callbackEventId := c.Client.UserID.String() + "_get_devices_" + bridgeCfg.Name
			botUsername := id.UserID(conf.BotName)
			mngRoom, err := bridges.GetBotManagementRoom(user.Client, &botUsername)
			if err != nil {
				slog.Error(err.Error())
				return
			}

			syncers.RegisterSyncMessageListener(&syncers.SyncEventCallback{
				ID:        callbackEventId,
				EventType: "m.room.message",
				IgnoreBot: ignoreBotMessage,
				BindRoom:  mngRoom,
				Callback: func(evt *event.Event, user *users.Users) error {
					defer func() {
						syncers.UnRegisterSyncMessageListener(callbackEventId)
						wg.Done()
					}()

					parsedDevices, err := devices.ParseListDevices(evt.Content.AsMessage().Body)
					if err != nil {
						slog.Error(err.Error())
						return err
					}

					device := (&devices.Devices{
						Client:     user.Client,
						DeviceId:   "",
						BridgeName: bridgeCfg.Name,
					})
					err = device.RemoveAllForBridge(bridgeCfg.Name)
					if err != nil {
						slog.Error(err.Error())
						return err
					}

					for _, d := range parsedDevices {
						device.DeviceId = d.Device
						if d.Connected {
							err = device.Save()
							if err != nil {
								slog.Error(err.Error())
								debug.PrintStack()
								return err
							}
						}
					}

					return nil
				},
			})

			err = bridges.QueryDevices(user.Client, bridgeCfg)
			if err != nil {
				slog.Error(err.Error())
				return
			}
		}(conf)
	}

	wg.Wait()
	devices, err := (&devices.Devices{Client: user.Client}).GetDevices()
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

func BootupSyncUsers() error {
	syncWatcher = syncers.SyncWatcher{
		Cache:    make([]string, 0),
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
	fileContent,
	groupUrl,
	replyId string,
	user *users.Users,
) (*id.EventID, error) {
	slog.Debug("[+] Sending message", "bridgeName", bridgeName, "replyId", replyId)

	roomId, bridgeCfg, err := getRoomInfo(
		user.Client,
		bridgeName,
		deviceId,
		groupUrl,
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

		evt, err := messages.SendMediaMessage(c.Client, *roomId, *filepath, message, replyId)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		return evt, nil
	} else {
		evt, err := messages.SendMessage(
			c.Client,
			*roomId,
			message,
			replyId,
			bridgeCfg.DisplayUsernameTemplate,
		)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		return evt, nil
	}
}

func (c *Controller) GetRoomInfo(
	deviceId,
	bridgeName,
	groupUrl string,
) ([]string, error) {
	user, err := users.FetchUser(c.Client)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	roomId, _, err := getRoomInfo(
		user.Client,
		bridgeName,
		deviceId,
		groupUrl,
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	idMembers, err := rooms.GetRoomMembers(c.Client, roomId)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	var members []string
	for _, member := range idMembers {
		members = append(members, member.DisplayName)
	}

	return members, nil
}

func (c *Controller) GetInfo() (*rooms.RoomInfo, error) {
	user, err := users.FetchUser(c.Client)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	roomInfo, err := rooms.GetSumInfo(user.Client)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	return roomInfo, nil
}
