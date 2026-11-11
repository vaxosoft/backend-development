package taskstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// SQLiteStore persists tasks with database/sql. Tags are stored as a JSON array.
type SQLiteStore struct{ db *sql.DB }

// NewSQLiteStore creates the schema if needed. The caller owns db and closes it.
func NewSQLiteStore(ctx context.Context, db *sql.DB) (*SQLiteStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tasks (
		id     INTEGER PRIMARY KEY AUTOINCREMENT,
		title  TEXT NOT NULL,
		status TEXT NOT NULL,
		tags   TEXT NOT NULL)`)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Create(ctx context.Context, t Task) (Task, error) {
	if t.Status == "" {
		t.Status = "todo"
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	tags, err := json.Marshal(t.Tags)
	if err != nil {
		return Task{}, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO tasks (title, status, tags) VALUES (?, ?, ?)`, t.Title, t.Status, string(tags))
	if err != nil {
		return Task{}, fmt.Errorf("insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Task{}, err
	}
	t.ID = int(id)
	return t, nil
}

func scan(row interface{ Scan(...any) error }) (Task, error) {
	var t Task
	var tags string
	if err := row.Scan(&t.ID, &t.Title, &t.Status, &tags); err != nil {
		return Task{}, err
	}
	if err := json.Unmarshal([]byte(tags), &t.Tags); err != nil {
		return Task{}, fmt.Errorf("decode tags: %w", err)
	}
	return t, nil
}

func (s *SQLiteStore) Get(ctx context.Context, id int) (Task, error) {
	t, err := scan(s.db.QueryRowContext(ctx, `SELECT id, title, status, tags FROM tasks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *SQLiteStore) List(ctx context.Context, status string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, status, tags FROM tasks WHERE ?1 = '' OR status = ?1 ORDER BY id`, status)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Delete(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
