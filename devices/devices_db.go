package devices

import (
	"database/sql"
)

type DeviceDB struct {
	connection *sql.DB
	Filepath   string
	Username   string
}

func (d *DeviceDB) Init() error {
	db, err := sql.Open("sqlite3", d.Filepath)
	if err != nil {
		return err
	}

	d.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS devices ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	device_id TEXT,
	bridge_name TEXT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, 
	UNIQUE(device_id, bridge_name)
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func (d *DeviceDB) fetchDevice(deviceId string) (*[]string, error) {
	stmt, err := d.connection.Prepare(
		"select device_id, bridge_name from devices where device_id = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(deviceId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var bridgeName string
		var deviceId string

		if err := rows.Scan(&bridgeName, &deviceId); err != nil {
			return nil, err
		}

		return &[]string{bridgeName, deviceId}, err
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nil, sql.ErrNoRows
}
