package db

import (
	"database/sql"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type StatusStore struct {
	db   *sql.DB
	lock sync.Mutex
}

func NewStatusStore(dbPath string) (*StatusStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS status_message (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT,
		channel_id TEXT,
		view_type TEXT
	)`)
	if err != nil {
		return nil, err
	}
	return &StatusStore{db: db}, nil
}

func (s *StatusStore) SetStatus(messageID, channelID, viewType string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Check if an entry with the same message_id and channel_id exists
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM status_message WHERE message_id = ? AND channel_id = ?)`, messageID, channelID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Update the existing entry
		_, err = s.db.Exec(`UPDATE status_message SET view_type = ? WHERE message_id = ? AND channel_id = ?`, viewType, messageID, channelID)
	} else {
		// Insert a new entry
		_, err = s.db.Exec(`INSERT INTO status_message (message_id, channel_id, view_type) VALUES (?, ?, ?)`, messageID, channelID, viewType)
	}
	return err
}

func (s *StatusStore) GetStatus() ([]struct{ MessageID, ChannelID, ViewType string }, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	rows, err := s.db.Query(`SELECT message_id, channel_id, view_type FROM status_message`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []struct{ MessageID, ChannelID, ViewType string }
	for rows.Next() {
		var msgID, chID, vType string
		if err := rows.Scan(&msgID, &chID, &vType); err != nil {
			return nil, err
		}
		entries = append(entries, struct{ MessageID, ChannelID, ViewType string }{msgID, chID, vType})
	}
	return entries, nil
}

func (s *StatusStore) DeleteStatus(messageID, channelID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.db.Exec(`DELETE FROM status_message WHERE message_id = ? AND channel_id = ?`, messageID, channelID)
	return err
}
