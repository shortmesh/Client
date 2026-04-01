package contacts

import (
	"database/sql"
	"log/slog"
	"runtime/debug"
)

type contactDb struct {
	connection *sql.DB
	Filepath   string
	Username   string
}

func getContactDb(username string) (*contactDb, error) {
	contactDb := contactDb{
		Username: username,
		Filepath: "db/" + username + ".db",
	}

	err := contactDb.Init()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return &contactDb, err
}

func (c *contactDb) Init() error {
	db, err := sql.Open("sqlite3", c.Filepath)
	if err != nil {
		return err
	}

	c.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS contacts ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	name TEXT,
	username TEXT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, 
	UNIQUE(name, username)
	);
	`)

	if err != nil {
		return err
	}
	return err
}

func (c *contactDb) save(name, username string) error {
	_, err := c.connection.Exec(
		"INSERT INTO contacts (name, username) VALUES (?, ?)",
		name, username,
	)
	if err != nil {
		slog.Debug(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func (c *contactDb) find(contact string) (*string, error) {
	var username string
	err := c.connection.QueryRow(
		"select username from contacts where name = ?", contact,
	).Scan(&username)
	if err != nil {
		return nil, err // includes sql.ErrNoRows automatically
	}
	return &username, nil
}
