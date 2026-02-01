package devices

import (
	"database/sql"
	"log/slog"
	"runtime/debug"

	"maunium.net/go/mautrix"
)

type Devices struct {
	Client     *mautrix.Client
	DeviceId   string
	BridgeName string
}

func GetDeviceDB(client *mautrix.Client) (*DeviceDB, error) {
	deviceDb := DeviceDB{
		Username: client.UserID.Localpart(),
		Filepath: "db/" + client.UserID.Localpart() + ".db",
	}
	err := deviceDb.Init()
	if err != nil {
		return nil, err
	}
	return &deviceDb, err
}

func IsDevice(client *mautrix.Client, deviceId string) (bool, error) {
	deviceDb, err := GetDeviceDB(client)
	devices, err := deviceDb.fetchDevice(deviceId)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	if len(*devices) > 0 {
		return true, nil
	}

	return false, nil
}

func (d *Devices) Save() error {
	devicesDb, err := GetDeviceDB(d.Client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if err := devicesDb.Save(d.DeviceId, d.BridgeName); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}
