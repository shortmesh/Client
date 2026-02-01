package users

import (
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
	Client *mautrix.Client
	UserID *id.UserID
}

func GetUserDB(client *mautrix.Client) UsersDB {
	return UsersDB{
		Username: client.UserID.Localpart(),
		Filepath: "db/" + client.UserID.Localpart() + ".db",
	}
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
		slog.Error(err.Error())
		debug.PrintStack()
		return -1, err
	}

	if isDevice {
		return Device, nil
	}

	isContact, err := isContact(client, userId.String())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
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
	usersDb := GetUserDB(client)

	if err := usersDb.Init(); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	roomId, err := usersDb.fetchIsContact(contact)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if roomId == nil {
		return false, nil
	}

	return true, nil
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
