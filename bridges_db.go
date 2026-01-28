package main

import (
	"database/sql"
	"log"
)

func (clientDb *ClientDB) FetchByRoomID(roomId string) (*Bridges, error) {
	log.Println("Fetching bridge rooms for", roomId)
	stmt, err := clientDb.connection.Prepare(
		"select name from rooms where roomId = ?",
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

	// for rows.Next() {
	// 	var name string

	// 	err = rows.Scan(&name)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	return (&Bridges{
	// 		BridgeConfig: BridgeConfig{
	// 			Name: name,
	// 		},
	// 	}), err
	// }

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

func (clientDb *ClientDB) StoreBridge(
	roomId string,
	name string,
	memberId string,
	deviceId string,
) error {
	tx, err := clientDb.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO rooms (roomId, name, memberId, deviceId, timestamp) 
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(roomId, name, memberId, deviceId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
