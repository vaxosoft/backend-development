package starter

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned by a Store when a task id does not exist.
var ErrNotFound = errors.New("task not found")

// ListQuery is a validated GET /tasks query.
type ListQuery struct {
	Status string // "" = any
	Tag    string // "" = any
	Sort   string // a key of orderBy
	Limit  int
	Offset int
}

// Store is the repository abstraction the handlers depend on.
type Store interface {
	Create(ctx context.Context, in TaskInput) (Task, error)
	Get(ctx context.Context, id int64) (Task, error)
	Replace(ctx context.Context, id int64, in TaskInput) (Task, error)
	Delete(ctx context.Context, id int64) error
	// List returns one page and the total number of matches.
	List(ctx context.Context, q ListQuery) ([]Task, int, error)
}

// orderBy maps the public sort values to SQL. Only these fixed strings are ever
// concatenated into a query; user input is never trusted as SQL. Tasks without
// a due date sort last in both directions; id breaks ties.
var orderBy = map[string]string{
	"created_at":  "created_at ASC, id ASC",
	"-created_at": "created_at DESC, id DESC",
	"due_date":    "due_date IS NULL, due_date ASC, id ASC",
	"-due_date":   "due_date IS NULL, due_date DESC, id DESC",
}

var errTodo = errors.New("TODO: not implemented")

// SQLiteStore implements Store on database/sql. Tags are stored as a JSON array
// in a TEXT column (filter them with SQLite's json_each).
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore returns a store on db. Call Migrate before using it.
func NewSQLiteStore(db *sql.DB) *SQLiteStore { return &SQLiteStore{db: db} }

// Migrate creates the schema if it does not exist.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	// TODO: CREATE TABLE IF NOT EXISTS tasks (...). created_at is stored as RFC 3339 text.
	return nil
}

// Create inserts a task with created_at = now (UTC, whole seconds).
func (s *SQLiteStore) Create(ctx context.Context, in TaskInput) (Task, error) {
	return Task{}, errTodo
}

// Get returns the task or ErrNotFound.
func (s *SQLiteStore) Get(ctx context.Context, id int64) (Task, error) {
	return Task{}, errTodo
}

// Replace overwrites every writable field (created_at is kept) or returns ErrNotFound.
func (s *SQLiteStore) Replace(ctx context.Context, id int64, in TaskInput) (Task, error) {
	return Task{}, errTodo
}

// Delete removes the task or returns ErrNotFound.
func (s *SQLiteStore) Delete(ctx context.Context, id int64) error {
	return errTodo
}

// List filters by status and tag, orders by q.Sort (a key of orderBy), applies
// limit/offset and also returns the total match count. Never leave rows open:
// with a single connection an open cursor blocks the next query.
func (s *SQLiteStore) List(ctx context.Context, q ListQuery) ([]Task, int, error) {
	return nil, 0, errTodo
}
