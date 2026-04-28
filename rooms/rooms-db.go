package rooms

import (
	"database/sql"
)

type RoomDB struct {
	connection *sql.DB
	Filepath   string
	Username   string
}

func (r *RoomDB) Init() error {
	db, err := sql.Open("sqlite3", r.Filepath)
	if err != nil {
		return err
	}

	r.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS rooms ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	room_id TEXT,
	bridged_id TEXT,
	bridge_name TEXT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, 
	UNIQUE(room_id, bridged_id)
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func (r *RoomDB) Insert(
	roomId,
	bridgedId,
	bridgeName string,
) error {
	tx, err := r.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO rooms (room_id, bridged_id, bridge_name, timestamp) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(roomId, bridgedId, bridgeName)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil

}

func (r *RoomDB) Find(roomId string) (*string, error) {
	stmt, err := r.connection.Prepare(
		"select bridged_id from rooms where room_id = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(roomId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var foundBridgedId string

		if err := rows.Scan(&foundBridgedId); err != nil {
			return nil, err
		}

		return &foundBridgedId, nil
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nil, sql.ErrNoRows

}

func (r *RoomDB) FindBridged(bridgedId string) (*string, error) {
	stmt, err := r.connection.Prepare(
		"select room_id from rooms where bridged_id = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(bridgedId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var foundRoomId string

		if err := rows.Scan(&foundRoomId); err != nil {
			return nil, err
		}

		return &foundRoomId, nil
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nil, sql.ErrNoRows

}
