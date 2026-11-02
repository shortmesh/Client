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
	defer tx.Rollback() // Safe rollback if tx.Commit() isn't reached

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

	return tx.Commit()
}

func (r *RoomDB) UpdateRoomId(
	roomId,
	bridgedId,
	bridgeName string,
) error {
	tx, err := r.connection.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// INSERT OR REPLACE automatically deletes the conflicting row and inserts the new one
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO rooms (room_id, bridged_id, bridge_name, timestamp) 
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(roomId, bridgedId, bridgeName)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteRoomsByBridgeName deletes all rooms matching a specific bridge name.
func (r *RoomDB) DeleteRoomsByBridgeId(bridgeId string) error {
	tx, err := r.connection.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Ensures rollback if commit doesn't happen

	_, err = tx.Exec(`
		DELETE FROM rooms 
		WHERE bridged_id = ?
	`, bridgeId)
	if err != nil {
		return err
	}

	return tx.Commit()
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

// Room represents a record in the rooms table.
type Room struct {
	RoomID     string
	BridgedID  string
	BridgeName string
	Timestamp  string // Or time.Time depending on your DB driver and setup
}

// GetAllRooms queries all records from the rooms table.
func (r *RoomDB) GetAllRooms() ([]Room, error) {
	rows, err := r.connection.Query(`
		SELECT room_id, bridged_id, bridge_name, timestamp 
		FROM rooms
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []Room

	// Iterate through the result set row by row
	for rows.Next() {
		var room Room
		if err := rows.Scan(&room.RoomID, &room.BridgedID, &room.BridgeName, &room.Timestamp); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	// Check for any errors encountered during iteration
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rooms, nil
}
