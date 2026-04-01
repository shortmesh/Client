package rooms

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
)

type roomsDb struct {
	connection *sql.DB
	Username   string
	Filepath   string
}

func (r *roomsDb) Init() error {
	db, err := sql.Open("sqlite3", r.Filepath)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
	}

	r.connection = db

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS rooms ( 
	id INTEGER PRIMARY KEY AUTOINCREMENT, 
	room_id TEXT NOT NULL,
	name TEXT NULL,
	username TEXT NULL,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, 
	UNIQUE(room_id, name, username)
	);
	`)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
	}
	return err
}

func (r *roomsDb) FetchRoomByRoomId(roomId string) (string, error) {
	stmt, err := r.connection.Prepare(
		"select bridge_name from rooms where room_id = ?",
	)
	if err != nil {
		return "", err
	}

	defer stmt.Close()

	rows, err := stmt.Query(roomId)
	if err != nil {
		return "", err
	}

	defer rows.Close()

	if rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", err
		}

		return name, err
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	return "", sql.ErrNoRows
}

func (r *roomsDb) FetchRoomByName(name string, isBridgeBot bool) (*[]string, error) {
	stmt, err := r.connection.Prepare(
		"select room_id from rooms where bridge_name = ? AND is_bridge_bot = ?",
	)
	if err != nil {
		return nil, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(name, isBridgeBot)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var roomIds []string
	for rows.Next() {
		var roomId string

		err = rows.Scan(&roomId)
		if err != nil {
			return nil, err
		}

		roomIds = append(roomIds, roomId)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &roomIds, nil
}

func (r *roomsDb) Clear(bridgeName string, isBridgeBot bool) error {
	tx, err := r.connection.Begin()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	stmt, err := tx.Prepare(`DELETE FROM rooms WHERE bridge_name = ? AND is_bridge_bot = ?`)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec(bridgeName, isBridgeBot)
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

func (r *roomsDb) Delete(roomId string) error {
	tx, err := r.connection.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM rooms WHERE room_id = ?`, roomId)
	if err != nil {
		return fmt.Errorf("deleting room: %w", err)
	}

	return tx.Commit()
}

func (r *roomsDb) Save(roomId, username string) error {
	_, err := r.connection.Exec(
		"INSERT INTO rooms (room_id, username) VALUES (?, ?)",
		roomId, username,
	)
	if err != nil {
		slog.Debug(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func (r *roomsDb) findUsernames(usernames []string) ([]string, error) {
	slog.Debug("Finding user rooms", "usernames", usernames)
	if len(usernames) == 0 {
		return nil, errors.New("no usernames provided")
	}

	placeholders := strings.Repeat("?,", len(usernames))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
        SELECT room_id
        FROM rooms
        WHERE username IN (%s)
        GROUP BY room_id
        HAVING COUNT(DISTINCT username) = ?
    `, placeholders)

	args := make([]any, len(usernames)+1)
	for i, c := range usernames {
		args[i] = c
	}
	args[len(usernames)] = len(usernames)

	rows, err := r.connection.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomIds []string
	for rows.Next() {
		var roomId string
		if err := rows.Scan(&roomId); err != nil {
			return nil, err
		}
		roomIds = append(roomIds, roomId)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(roomIds) == 0 {
		return nil, sql.ErrNoRows
	}
	return roomIds, nil
}
