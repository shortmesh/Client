package users

import (
	"database/sql"
	"log/slog"
	"runtime/debug"

	"github.com/shortmesh/core/configs"
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
		Username: client.UserID.String(),
		Filepath: "db/" + client.UserID.String() + ".db",
	}

	err := usersDb.Init()
	if err != nil {
		return nil, err
	}

	return &usersDb, err
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

func RemoveUser(client *mautrix.Client) error {
	clientDb, err := GetClientDB()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = clientDb.DeleteUser(client.UserID.String())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
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
