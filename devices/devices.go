package devices

import (
	"log/slog"
	"runtime/debug"

	"maunium.net/go/mautrix"
)

func GetDeviceDB(client *mautrix.Client) DeviceDB {
	return DeviceDB{
		Username: client.UserID.Localpart(),
		Filepath: "db/" + client.UserID.Localpart() + ".db",
	}
}

func IsDevice(client *mautrix.Client, deviceId string) (bool, error) {
	deviceDb := GetDeviceDB(client)
	devices, err := deviceDb.fetchDevice(deviceId)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if len(*devices) > 0 {
		return true, nil
	}

	return false, nil
}
