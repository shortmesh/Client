package contacts

import (
	"database/sql"
	"log/slog"
	"runtime/debug"

	"maunium.net/go/mautrix"
)

type ContactsDB struct {
	connection *sql.DB
	Username   string
	Filepath   string
}

func (c *ContactsDB) init() error {
	db, err := sql.Open("sqlite3", c.Filepath)
	if err != nil {
		return err
	}

	c.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS contacts ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	name TEXT NOT NULL UNIQUE, 
	username TEXT NOT NULL, 
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(name, username)
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func getDb(client *mautrix.Client) (*ContactsDB, error) {
	contactsDb := ContactsDB{
		Username: client.UserID.String(),
		Filepath: "db/" + client.UserID.String() + ".db",
	}

	err := contactsDb.init()
	if err != nil {
		return nil, err
	}

	return &contactsDb, err
}

func (c *ContactsDB) insert(name, username string) error {
	_, err := c.connection.Exec(
		"INSERT OR IGNORE INTO contacts (name, username) VALUES (?, ?)",
		name, username,
	)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}
