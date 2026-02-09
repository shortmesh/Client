package bridges

// func (UserDB *UserDB) FetchActiveSessions(username string) ([]byte, time.Time, error) {
// 	stmt, err := UserDB.connection.Prepare("select sessions, sessionsTimestamp from rooms where clientUsername = ? and isBridge = 1")
// 	if err != nil {
// 		return []byte{}, time.Time{}, err
// 	}
// 	defer stmt.Close()

// 	var sessions []byte
// 	var sessionsTimestamp time.Time
// 	err = stmt.QueryRow(username).Scan(&sessions, &sessionsTimestamp)
// 	if err != nil {
// 		return []byte{}, time.Time{}, err
// 	}
// 	return sessions, sessionsTimestamp, nil
// }

// func (UserDB *UserDB) StoreActiveSessions(username string, sessions []byte) error {
// 	tx, err := UserDB.connection.Begin()
// 	if err != nil {
// 		return err
// 	}

// 	stmt, err := tx.Prepare("UPDATE rooms SET sessions = ?, sessionsTimestamp = CURRENT_TIMESTAMP WHERE clientUsername = ? AND isBridge = 1")
// 	if err != nil {
// 		return err
// 	}

// 	defer stmt.Close()

// 	_, err = stmt.Exec(sessions, username)
// 	if err != nil {
// 		return err
// 	}

// 	err = tx.Commit()
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (UserDB *UserDB) RemoveActiveSessions(username string) error {
// 	tx, err := UserDB.connection.Begin()
// 	if err != nil {
// 		return err
// 	}

// 	stmt, err := tx.Prepare("update rooms set sessions = NULL, sessionsTimestamp = CURRENT_TIMESTAMP where clientUsername = ?")
// 	if err != nil {
// 		return err
// 	}

// 	defer stmt.Close()

// 	_, err = stmt.Exec(username)
// 	if err != nil {
// 		return err
// 	}

// 	err = tx.Commit()
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (b *Bridges) checkActiveSessions() (bool, error) {
// 	var UsersDB = users.UsersDB{
// 		Username: b.Client.UserID.Localpart(),
// 		Filepath: "db/" + b.Client.UserID.Localpart() + ".db",
// 	}

// 	if err := UsersDB.Init(); err != nil {
// 		return false, err
// 	}

// 	activeSessions, _, err := UsersDB.FetchActiveSessions(b.Client.UserID.Localpart())
// 	if err != nil {
// 		return false, err
// 	}

// 	if len(activeSessions) == 0 {
// 		return false, nil
// 	}

// 	return true, nil
// }
