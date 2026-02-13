package users

import (
	"database/sql"
	"log/slog"
	"runtime/debug"

	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type UserType int

const (
	User UserType = iota
	BridgeBot
	Device
	Contact
)

type Users struct {
	Client      *mautrix.Client
	RecoveryKey string
	PickleKey   []byte
}

func GetClientDB() (*ClientDB, error) {
	clientDb := ClientDB{
		Filepath: "db/clients.db",
	}

	err := clientDb.Init()
	if err != nil {
		return nil, err
	}

	return &clientDb, err
}

func GetUserDB(client *mautrix.Client) (*UserDB, error) {
	usersDb := UserDB{
		Username: client.UserID.Localpart(),
		Filepath: "db/" + client.UserID.Localpart() + ".db",
	}

	err := usersDb.Init()
	if err != nil {
		return nil, err
	}

	return &usersDb, err
}

func GetTypeUser(client *mautrix.Client, userId id.UserID) (UserType, error) {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return -1, err
	}

	if userId == client.UserID {
		return User, nil
	}

	for _, bridgeConf := range conf.Bridges {
		if userId == id.UserID(bridgeConf.BotName) {
			return BridgeBot, nil
		}
	}

	isDevice, err := devices.IsDevice(client, userId.String())

	if err != nil {
		return -1, err
	}

	if isDevice {
		return Device, nil
	}

	isContact, err := isContact(client, userId.String())
	if err != nil {
		return -1, err
	}

	if isContact {
		return Contact, nil
	}

	return -1, nil
}

func isContact(
	client *mautrix.Client,
	contact string,
) (bool, error) {
	usersDb, err := GetUserDB(client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	roomId, err := usersDb.fetchIsContact(contact)

	if err != nil {
		if err == sql.ErrNoRows {
			cfg, err := configs.GetConf()
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return false, err
			}

			for _, bridgeConf := range cfg.Bridges {
				matched, err := cfg.CheckUserBridgeBotTemplate(bridgeConf.Name, contact)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return false, err
				}

				if matched {
					return true, err
				}
			}
			return false, nil
		}
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if roomId == nil {
		return false, nil
	}

	return true, nil
}

func (u *Users) Save() error {
	usersDb, err := GetUserDB(u.Client)
	if err != nil {
		debug.PrintStack()
		return err
	}

	err = usersDb.Store(
		u.Client.UserID.String(),
		u.Client.AccessToken,
		u.Client.DeviceID.String(),
		u.RecoveryKey,
		u.PickleKey,
	)
	if err != nil {
		debug.PrintStack()
		return err
	}

	clientDb, err := GetClientDB()
	if err != nil {
		debug.PrintStack()
		return err
	}

	err = clientDb.Store(u.Client.UserID.String())
	if err != nil {
		debug.PrintStack()
		return err
	}
	return nil
}

func FetchMessageContact(
	client *mautrix.Client,
	deviceId,
	bridgeName,
	contact string,
) (*string, error) {
	usersDb, err := GetUserDB(client)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	roomId, err := usersDb.FetchDeviceBridgeContact(
		deviceId,
		bridgeName,
		contact,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return roomId, nil
}

func FetchUser(client *mautrix.Client) (*Users, error) {
	userDb, err := GetUserDB(client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	_, accessToken, deviceId, pickleKey, err := userDb.FetchUser(client.UserID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return &Users{}, nil
		}

		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	client.AccessToken = accessToken
	client.DeviceID = id.DeviceID(deviceId)

	return &Users{
		Client:    client,
		PickleKey: pickleKey,
	}, nil
}

func FetchAllUsers() ([]Users, error) {
	clientDb, err := GetClientDB()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	fetchedUsers, err := clientDb.FetchUsers()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	var users []Users
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	for _, username := range fetchedUsers {
		client, err := mautrix.NewClient(
			conf.HomeServer,
			id.UserID(username),
			"",
		)
		user, err := FetchUser(client)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}

		users = append(users, *user)
	}
	return users, nil
}
