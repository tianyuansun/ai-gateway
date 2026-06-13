package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using a SQLite file for durable storage.
type SQLiteStore struct {
	db  *sql.DB
	ttl time.Duration
	mu  sync.RWMutex
}

// NewSQLiteStore creates a new SQLiteStore.
func NewSQLiteStore(path string, ttl time.Duration) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		data BLOB,
		created_at INTEGER,
		last_access INTEGER
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite create table: %w", err)
	}

	store := &SQLiteStore{db: db, ttl: ttl}
	go store.pruneLoop()
	return store, nil
}

func (s *SQLiteStore) Get(sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	var lastAccess int64
	err := s.db.QueryRow("SELECT data, last_access FROM sessions WHERE id = ?", sessionID).Scan(&data, &lastAccess)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if time.Since(time.Unix(lastAccess, 0)) > s.ttl {
		return nil, nil
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	sess.LastAccess = time.Now()
	return &sess, nil
}

func (s *SQLiteStore) Set(sessionID string, sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess.TTL == 0 {
		sess.TTL = s.ttl
	}
	sess.LastAccess = time.Now()

	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO sessions (id, data, created_at, last_access) VALUES (?, ?, ?, ?)",
		sessionID, data, now, now,
	)
	return err
}

func (s *SQLiteStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

func (s *SQLiteStore) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-s.ttl).Unix()
	s.db.Exec("DELETE FROM sessions WHERE last_access < ?", cutoff)
}

func (s *SQLiteStore) pruneLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.Prune()
	}
}
