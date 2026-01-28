package main

func (clientDb *ClientDB) FetchByRoomID(roomId string) (*Bridges, error) {
	// log.Println("Fetching bridge rooms for", username, clientDb.filepath)
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

	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		if err != nil {
			return (&Bridges{
				BridgeConfig: BridgeConfig{
					Name: name,
				},
			}), err
		}
	}

	return nil, err
}

func (clientDb *ClientDB) StoreBridge(
	roomId string,
	name string,
	isGroup bool,
	memberId string,
	deviceId string,
) error {
	tx, err := clientDb.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO clients (roomId, name, isGroup, memberId, deviceId, timestamp) 
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(roomId, name, isGroup, memberId, deviceId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
