package shortener

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Sentinel errors the store returns for expected situations.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

// User is a registered account.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
}

// Link is a short link and its owner.
type Link struct {
	Slug    string `json:"slug"`
	URL     string `json:"url"`
	Hits    int64  `json:"hits"`
	OwnerID int64  `json:"-"`
}

// Store is the SQLite-backed persistence layer.
type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	email         TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS links (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	slug     TEXT NOT NULL UNIQUE,
	url      TEXT NOT NULL,
	owner_id INTEGER NOT NULL REFERENCES users(id),
	hits     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS links_owner ON links(owner_id, id);
`

func openStore(path string) (*Store, error) {
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// Every connection to ":memory:" is a separate, empty database, so the
	// pool is limited to one. Never hold rows open while running a second query.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// isUnique reports whether err is a UNIQUE constraint violation.
func isUnique(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
