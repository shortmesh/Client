package users

import (
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
	Client *mautrix.Client
	UserID *id.UserID
}

func GetUserDB(client *mautrix.Client) UsersDB {
	return UsersDB{
		Username: client.UserID.Localpart(),
		Filepath: "db/" + client.UserID.Localpart() + ".db",
	}
}

func (u *Users) GetTypeUser() (UserType, error) {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return -1, err
	}

	for _, bridgeConf := range conf.Bridges {
		if *u.UserID == id.UserID(bridgeConf.BotName) {
			return BridgeBot, nil
		}
	}

	if *u.UserID == u.Client.UserID {
		return User, nil
	}

	return -1, nil
}

func FetchMessageContact(
	client *mautrix.Client,
	deviceId,
	bridgeName,
	contact string,
) (*string, error) {
	usersDb := GetUserDB(client)

	if err := usersDb.Init(); err != nil {
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
