package users

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shortmesh/core/configs"
)

type UsersDB struct {
	connection *sql.DB
	Username   string
	Filepath   string
}

func (u *UsersDB) CreateUser(username string, accessToken string) error {
	tx, err := u.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO users (username, accessToken) values(?,?)`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(username, accessToken)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (u *UsersDB) FetchUser(username string) (*configs.UsersConfig, error) {
	stmt, err := u.connection.Prepare("select id, username, accessToken from users where username = ?")
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	var id int
	var _username string
	var _accessToken string
	err = stmt.QueryRow(username).Scan(&id, &_username, &_accessToken)
	if err != nil {
		return nil, err
	}

	return &configs.UsersConfig{Username: _username, AccessToken: _accessToken}, nil
}

// func (ks *Keystore) FetchAllUsers() ([]Users, error) {
// stmt, err := ks.connection.Prepare("select id, username, accessToken from users")
// if err != nil {
// 	return []Users{}, err
// }

// defer stmt.Close()

// rows, err := stmt.Query()
// if err != nil {
// 	return []Users{}, err
// }

// defer rows.Close()

// var users []Users
// for rows.Next() {
// 	var id int
// 	var _username string
// 	var _accessToken string

// 	err = rows.Scan(&id, &_username, &_accessToken)
// 	if err != nil {
// 		return []Users{}, err
// 	}

// 	users = append(users, Users{ID: id, Username: _username, AccessToken: _accessToken})
// }

// return users, nil
// }

// https://github.com/mattn/go-sqlite3/blob/v1.14.28/_example/simple/simple.go
func (UsersDB *UsersDB) Init() error {
	db, err := sql.Open("sqlite3", UsersDB.Filepath)
	if err != nil {
		return err
	}

	UsersDB.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS user ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	username TEXT NOT NULL UNIQUE, 
	password TEXT NOT NULL,
	access_token TEXT NOT NULL, 
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func (UsersDB *UsersDB) AuthenticateAccessToken(username string, accessToken string) (bool, error) {
	query := `SELECT COUNT(*) FROM clients WHERE username = ? AND accessToken = ?`

	var count int
	err := UsersDB.connection.QueryRow(query, username, accessToken).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("authentication query failed: %w", err)
	}

	if count == 0 {
		log.Printf("[-] Authentication failed for user: %s", username)
		return false, nil
	}

	log.Printf("[+] Authentication successful for user: %s", username)
	return true, nil
}

func (UsersDB *UsersDB) Authenticate(username string, password string) (bool, error) {
	query := `SELECT COUNT(*) FROM clients WHERE username = ? AND password = ?`

	var count int
	err := UsersDB.connection.QueryRow(query, username, password).Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[-] Authentication failed for user: %s", username)
			return false, nil
		}
		return false, fmt.Errorf("authentication query failed: %w", err)
	}

	if count == 0 {
		log.Printf("[-] Authentication failed for user: %s", username)
		return false, nil
	}

	log.Printf("[+] Authentication successful for user: %s", username)
	return true, nil
}

func (UsersDB *UsersDB) Store(accessToken string, password string) error {
	tx, err := UsersDB.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO clients (username, accessToken, password, timestamp) 
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(UsersDB.Username, accessToken, password)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (UsersDB *UsersDB) Fetch() (string, error) {
	fmt.Println("- Fetching username:", UsersDB.Filepath)
	stmt, err := UsersDB.connection.Prepare("select accessToken from clients where username = ?")
	var accessToken string

	if err != nil {
		return accessToken, err
	}

	defer stmt.Close()

	err = stmt.QueryRow(UsersDB.Username).Scan(&accessToken)
	if err != nil {
		if err == sql.ErrNoRows {
			return accessToken, nil
		}
		return accessToken, err
	}
	return accessToken, err
}

func (UsersDB *UsersDB) Close() {
	defer UsersDB.connection.Close()
}

func (UsersDB *UsersDB) FetchActiveSessions(username string) ([]byte, time.Time, error) {
	stmt, err := UsersDB.connection.Prepare("select sessions, sessionsTimestamp from rooms where clientUsername = ? and isBridge = 1")
	if err != nil {
		return []byte{}, time.Time{}, err
	}
	defer stmt.Close()

	var sessions []byte
	var sessionsTimestamp time.Time
	err = stmt.QueryRow(username).Scan(&sessions, &sessionsTimestamp)
	if err != nil {
		return []byte{}, time.Time{}, err
	}
	return sessions, sessionsTimestamp, nil
}

func (UsersDB *UsersDB) StoreActiveSessions(username string, sessions []byte) error {
	tx, err := UsersDB.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("UPDATE rooms SET sessions = ?, sessionsTimestamp = CURRENT_TIMESTAMP WHERE clientUsername = ? AND isBridge = 1")
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(sessions, username)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (UsersDB *UsersDB) RemoveActiveSessions(username string) error {
	tx, err := UsersDB.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("update rooms set sessions = NULL, sessionsTimestamp = CURRENT_TIMESTAMP where clientUsername = ?")
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(username)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (UsersDB *UsersDB) FetchDeviceBridgeContact(deviceId, bridgeName, contact string) (*string, error) {
	stmt, err := UsersDB.connection.Prepare(
		"select room_id from rooms where device_id = ? AND bridge_name = ? AND contact_name = ? AND is_bridge_bot = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(deviceId, bridgeName, contact, false)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var roomId string

		err = rows.Scan(&roomId)
		if err != nil {
			return nil, err
		}
		return &roomId, nil
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return nil, sql.ErrNoRows
}
