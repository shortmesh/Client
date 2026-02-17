package rooms

import (
	"database/sql"
	"log/slog"
	"runtime/debug"
)

type RoomsDB struct {
	connection *sql.DB
	Username   string
	Filepath   string
}

func (r *RoomsDB) Init() error {
	db, err := sql.Open("sqlite3", r.Filepath)
	if err != nil {
		return err
	}

	r.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS rooms ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	device_id TEXT,
	bridge_name TEXT,
	room_id TEXT NOT NULL,
	contact_name TEXT NULL,
	is_bridge_bot BOOLEAN NULL,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, 
	UNIQUE(device_id, bridge_name, contact_name)
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func (r *RoomsDB) FetchRoomByRoomId(roomId string) (string, error) {
	stmt, err := r.connection.Prepare(
		"select bridge_name from rooms where room_id = ?",
	)
	if err != nil {
		return "", err
	}

	defer stmt.Close()

	rows, err := stmt.Query(roomId)
	if err != nil {
		return "", err
	}

	defer rows.Close()

	if rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", err
		}

		return name, err
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	return "", sql.ErrNoRows
}

func (r *RoomsDB) FetchRoomByName(name string, isBridgeBot bool) (*[]string, error) {
	stmt, err := r.connection.Prepare(
		"select room_id from rooms where bridge_name = ? AND is_bridge_bot = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(name, isBridgeBot)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var roomIds []string
	for rows.Next() {
		var roomId string

		err = rows.Scan(&roomId)
		if err != nil {
			return nil, err
		}

		roomIds = append(roomIds, roomId)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &roomIds, nil
}

func (r *RoomsDB) Clear(bridgeName string, isBridgeBot bool) error {
	tx, err := r.connection.Begin()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	stmt, err := tx.Prepare(`DELETE FROM rooms WHERE bridge_name = ? AND is_bridge_bot = ?`)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(bridgeName, isBridgeBot)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = tx.Commit()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func (r *RoomsDB) Delete(roomId string) error {
	tx, err := r.connection.Begin()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	stmt, err := tx.Prepare(`DELETE FROM rooms WHERE room_id = ?`)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(roomId)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = tx.Commit()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func (r *RoomsDB) Save(
	roomId string,
	bridgeName string,
	memberId string,
	deviceId string,
	isBridgeBot bool,
) error {
	tx, err := r.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO rooms (device_id, bridge_name, room_id, contact_name, is_bridge_bot, timestamp) 
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(deviceId, bridgeName, roomId, memberId, isBridgeBot)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
