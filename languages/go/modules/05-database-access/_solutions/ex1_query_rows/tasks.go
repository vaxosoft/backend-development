// Package queryrows is exercise 1: read rows with database/sql, map NULLs and use parameters.
package queryrows

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned by Get when no row has the id.
var ErrNotFound = errors.New("task not found")

// Task mirrors the tasks table:
//
//	CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL,
//	                    due_date TEXT, owner_id INTEGER)
//
// due_date and owner_id are nullable.
type Task struct {
	ID      int64
	Title   string
	Status  string
	DueDate *string
	OwnerID *int64
}

const columns = `id, title, status, due_date, owner_id`

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Task, error) {
	var t Task
	err := s.Scan(&t.ID, &t.Title, &t.Status, &t.DueDate, &t.OwnerID)
	return t, err
}

// Get returns one task or ErrNotFound.
func Get(ctx context.Context, db *sql.DB, id int64) (Task, error) {
	t, err := scan(db.QueryRowContext(ctx, `SELECT `+columns+` FROM tasks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

// List returns tasks ordered by id. An empty status means "all". The result is never nil.
// Rows must be closed and rows.Err() checked.
func List(ctx context.Context, db *sql.DB, status string) ([]Task, error) {
	return query(ctx, db, `SELECT `+columns+` FROM tasks WHERE (? = '' OR status = ?) ORDER BY id`, status, status)
}

// SearchTitle returns tasks whose title contains term, ignoring ASCII case.
// term is user input: pass it as a query parameter, never splice it into the SQL.
func SearchTitle(ctx context.Context, db *sql.DB, term string) ([]Task, error) {
	return query(ctx, db, `SELECT `+columns+` FROM tasks WHERE title LIKE '%' || ? || '%' ORDER BY id`, term)
}

func query(ctx context.Context, db *sql.DB, q string, args ...any) ([]Task, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
