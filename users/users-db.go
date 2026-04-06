package users

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"

	"github.com/shortmesh/core/configs"
)

type ClientDB struct {
	connection *sql.DB
	Filepath   string
}

type UserDB struct {
	connection *sql.DB
	Username   string
	Filepath   string
}

// https://github.com/mattn/go-sql/blob/v1.14.28/_example/simple/simple.go
func (UserDB *UserDB) Init() error {
	conf, err := configs.GetConf()
	if err != nil {
		return err
	}

	key := conf.DATABASE_KEY
	dbname := fmt.Sprintf("%s?_pragma_key=x'%s'&_pragma_cipher_page_size=4096", UserDB.Filepath, key)
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return err
	}

	UserDB.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS user ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	username TEXT NOT NULL UNIQUE, 
	access_token TEXT NOT NULL, 
	device_id TEXT NOT NULL, 
	recovery_key TEXT NOT NULL, 
	pickle_key BLOB NOT NULL, 
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(username)
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func (c *ClientDB) Init() error {
	conf, err := configs.GetConf()
	if err != nil {
		return err
	}

	key := conf.DATABASE_KEY
	dbname := fmt.Sprintf("%s?_pragma_key=x'%s'&_pragma_cipher_page_size=4096", c.Filepath, key)
	db, err := sql.Open("sqlite3", dbname)

	c.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS clients ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	username TEXT NOT NULL UNIQUE, 
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(username)
	);
	`)

	if err != nil {
		return err
	}

	return err
}

func (u *UserDB) FetchUser(username string) (string, string, string, []byte, error) {
	stmt, err := u.connection.Prepare("select id, username, access_token, device_id, pickle_key from user where username = ?")
	if err != nil {
		return "", "", "", nil, err
	}

	defer stmt.Close()

	var id int
	var _username string
	var accessToken string
	var deviceId string
	var pickleKey []byte
	err = stmt.QueryRow(username).Scan(&id, &_username, &accessToken, &deviceId, &pickleKey)
	if err != nil {
		return "", "", "", nil, err
	}

	return _username, accessToken, deviceId, pickleKey, nil
}

func (UserDB *UserDB) AuthenticateAccessToken(username string, accessToken string) (bool, error) {
	query := `SELECT COUNT(*) FROM user WHERE username = ? AND access_token = ?`

	var count int
	err := UserDB.connection.QueryRow(query, username, accessToken).Scan(&count)
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

func (UserDB *UserDB) Authenticate(username string, password string) (bool, error) {
	query := `SELECT COUNT(*) FROM user WHERE username = ? AND password = ?`

	var count int
	err := UserDB.connection.QueryRow(query, username, password).Scan(&count)
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

func (UserDB *UserDB) Store(
	username,
	accessToken,
	deviceId,
	recoveryKey string,
	pickleKey []byte,
) error {
	tx, err := UserDB.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO user (username, access_token, recovery_key, device_id, pickle_key, timestamp) 
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(username, accessToken, recoveryKey, deviceId, pickleKey)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientDB) Store(username string) error {
	tx, err := c.connection.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO clients (username, timestamp) VALUES (?, CURRENT_TIMESTAMP)
	`)
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

func (c *ClientDB) DeleteUser(username string) error {
	tx, err := c.connection.Begin()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	stmt, err := tx.Prepare(`DELETE FROM clients WHERE username = ?`)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(username)
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

func (c *ClientDB) FetchUsers() ([]string, error) {
	stmt, err := c.connection.Prepare("select username from clients")
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	var users []string

	rows, err := stmt.Query()
	for rows.Next() {
		var username string
		err = rows.Scan(&username)
		if err != nil {
			return nil, err
		}
		users = append(users, username)
	}
	return users, nil
}

func (UserDB *UserDB) Close() {
	defer UserDB.connection.Close()
}

func (UserDB *UserDB) FetchDeviceBridgeContact(
	deviceId,
	bridgeName,
	contact string,
) (*string, error) {
	slog.Debug("Fetching bridge", "deviceId", deviceId, "bridgeNam", bridgeName, "contact", contact)
	stmt, err := UserDB.connection.Prepare(
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

func (UserDB *UserDB) FetchDeviceBridgeContactOnly(
	bridgeName,
	contact string,
) (string, error) {
	slog.Debug("Fetching bridge", "bridgeNam", bridgeName, "contact", contact)
	stmt, err := UserDB.connection.Prepare(
		"select room_id from rooms where bridge_name = ? AND contact_name = ? AND is_bridge_bot = ?",
	)
	if err != nil {
		return "", err
	}

	defer stmt.Close()

	rows, err := stmt.Query(bridgeName, contact, false)
	if err != nil {
		return "", err
	}

	defer rows.Close()

	if rows.Next() {
		var roomId string

		err = rows.Scan(&roomId)
		if err != nil {
			return "", err
		}
		return roomId, nil
	}

	if err = rows.Err(); err != nil {
		return "", err
	}

	return "", sql.ErrNoRows
}
