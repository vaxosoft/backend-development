package starter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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

// SQLiteStore implements Store on database/sql. Tags are stored as a JSON array
// in a TEXT column (filter them with SQLite's json_each).
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore returns a store on db. Call Migrate before using it.
func NewSQLiteStore(db *sql.DB) *SQLiteStore { return &SQLiteStore{db: db} }

const columns = "id, title, status, due_date, tags, owner_id, created_at"

// Migrate creates the schema if it does not exist.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			title      TEXT NOT NULL,
			status     TEXT NOT NULL,
			due_date   TEXT,
			tags       TEXT NOT NULL DEFAULT '[]',
			owner_id   INTEGER,
			created_at TEXT NOT NULL
		)`)
	return err
}

// Create inserts a task with created_at = now (UTC, whole seconds).
func (s *SQLiteStore) Create(ctx context.Context, in TaskInput) (Task, error) {
	tags, err := json.Marshal(in.Tags)
	if err != nil {
		return Task{}, err
	}
	created := time.Now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (title, status, due_date, tags, owner_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		in.Title, in.Status, in.DueDate, string(tags), in.OwnerID, created.Format(time.RFC3339))
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Task{}, err
	}
	return s.Get(ctx, id)
}

// Get returns the task or ErrNotFound.
func (s *SQLiteStore) Get(ctx context.Context, id int64) (Task, error) {
	t, err := scanTask(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM tasks WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

// Replace overwrites every writable field (created_at is kept) or returns ErrNotFound.
func (s *SQLiteStore) Replace(ctx context.Context, id int64, in TaskInput) (Task, error) {
	tags, err := json.Marshal(in.Tags)
	if err != nil {
		return Task{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET title = ?, status = ?, due_date = ?, tags = ?, owner_id = ? WHERE id = ?`,
		in.Title, in.Status, in.DueDate, string(tags), in.OwnerID, id)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return Task{}, err
	} else if n == 0 {
		return Task{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes the task or returns ErrNotFound.
func (s *SQLiteStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

// List filters by status and tag, orders by q.Sort (a key of orderBy), applies
// limit/offset and also returns the total match count. Never leave rows open:
// with a single connection an open cursor blocks the next query.
func (s *SQLiteStore) List(ctx context.Context, q ListQuery) ([]Task, int, error) {
	var where []string
	var args []any
	if q.Status != "" {
		where = append(where, "status = ?")
		args = append(args, q.Status)
	}
	if q.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)")
		args = append(args, q.Tag)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	order, ok := orderBy[q.Sort]
	if !ok {
		order = orderBy["-created_at"]
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+columns+" FROM tasks"+clause+" ORDER BY "+order+" LIMIT ? OFFSET ?",
		append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

func scanTask(sc scanner) (Task, error) {
	var (
		t       Task
		due     sql.NullString
		owner   sql.NullInt64
		tags    string
		created string
	)
	if err := sc.Scan(&t.ID, &t.Title, &t.Status, &due, &tags, &owner, &created); err != nil {
		return Task{}, err
	}
	if due.Valid {
		t.DueDate = &due.String
	}
	if owner.Valid {
		t.OwnerID = &owner.Int64
	}
	if err := json.Unmarshal([]byte(tags), &t.Tags); err != nil {
		return Task{}, fmt.Errorf("decode tags: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	var err error
	if t.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
		return Task{}, fmt.Errorf("decode created_at: %w", err)
	}
	return t, nil
}
