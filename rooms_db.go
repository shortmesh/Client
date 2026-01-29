package main

import (
	"database/sql"
	"log"

	"maunium.net/go/mautrix/id"
)

func (clientDb *ClientDB) FetchRoomByRoomId(roomId string) (*Bridges, error) {
	log.Println("Fetching bridge rooms for", roomId)
	stmt, err := clientDb.connection.Prepare(
		"select name from rooms where room_id = ?",
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
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		return &Bridges{
			BridgeConfig: BridgeConfig{
				Name: name,
			},
		}, nil
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nil, sql.ErrNoRows
}

func (clientDb *ClientDB) FetchRoomByName(name string) ([]*Bridges, error) {
	log.Println("Fetching bridge rooms for", name)
	stmt, err := clientDb.connection.Prepare(
		"select room_id from rooms where name = ? AND is_bridge_bot = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(name, true)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var bridges []*Bridges
	for rows.Next() {
		var roomId string

		err = rows.Scan(&roomId)
		if err != nil {
			return nil, err
		}

		bridges = append(bridges, &Bridges{
			RoomID: id.RoomID(roomId),
			BridgeConfig: BridgeConfig{
				Name: name,
			},
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return bridges, nil
}

func (clientDb *ClientDB) StoreRoom(
	roomId string,
	name string,
	memberId string,
	deviceId string,
	isBridgeBot bool,
) error {
	tx, err := clientDb.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO rooms (room_id, name, member_id, device_id, is_bridge_bot, timestamp) 
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(roomId, name, memberId, deviceId, isBridgeBot)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
