// Task Tracker repository on SQLite: versioned migrations, a repository type and sentinel errors.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("task not found")

type Task struct {
	ID        int64
	Title     string
	Status    string
	DueDate   *string
	Tags      []string
	CreatedAt time.Time
}

type ListFilter struct {
	Status string
	Tag    string
	Limit  int
	Offset int
}

// TaskStore is what handlers depend on; defined here, at the consumer.
type TaskStore interface {
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, f ListFilter) ([]Task, error)
	SetStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

var _ TaskStore = (*Repo)(nil)

var migrations = []struct {
	version int
	sql     string
}{
	{1, `CREATE TABLE tasks (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		title      TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
		status     TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo','in_progress','done')),
		due_date   TEXT,
		tags       TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL
	)`},
	{2, `CREATE INDEX idx_tasks_status ON tasks (status)`},
}

// migrate applies every migration newer than the recorded version, each in its own transaction.
func migrate(ctx context.Context, db *sql.DB) (applied []int, err error) {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return nil, err
	}
	for _, m := range migrations {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)`, m.version).Scan(&exists); err != nil {
			return applied, err
		}
		if exists {
			continue
		}
		if err := applyMigration(ctx, db, m.version, m.sql); err != nil {
			return applied, err
		}
		applied = append(applied, m.version)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, ddl string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
		return err
	}
	return tx.Commit()
}

type Repo struct {
	db  *sql.DB
	now func() time.Time
}

type scanner interface{ Scan(dest ...any) error }

const taskColumns = `id, title, status, due_date, tags, created_at`

func scanTask(s scanner) (Task, error) {
	var (
		t       Task
		tags    string
		created string
	)
	if err := s.Scan(&t.ID, &t.Title, &t.Status, &t.DueDate, &tags, &created); err != nil {
		return Task{}, err
	}
	if err := json.Unmarshal([]byte(tags), &t.Tags); err != nil {
		return Task{}, fmt.Errorf("decode tags: %w", err)
	}
	var err error
	if t.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
		return Task{}, fmt.Errorf("decode created_at: %w", err)
	}
	return t, nil
}

func (r *Repo) Create(ctx context.Context, t Task) (Task, error) {
	if t.Status == "" {
		t.Status = "todo"
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	t.CreatedAt = r.now().UTC()
	tags, err := json.Marshal(t.Tags)
	if err != nil {
		return Task{}, err
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (title, status, due_date, tags, created_at) VALUES (?, ?, ?, ?, ?)`,
		t.Title, t.Status, t.DueDate, string(tags), t.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}
	if t.ID, err = res.LastInsertId(); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (r *Repo) Get(ctx context.Context, id int64) (Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

// List builds the WHERE clause from fixed fragments; user values only ever travel as arguments.
func (r *Repo) List(ctx context.Context, f ListFilter) ([]Task, error) {
	var (
		where []string
		args  []any
	)
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)")
		args = append(args, f.Tag)
	}
	q := `SELECT ` + taskColumns + ` FROM tasks`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q += " ORDER BY id LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repo) SetStatus(ctx context.Context, id int64, status string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE tasks SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	return mustAffect(res)
}

func (r *Repo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return mustAffect(res)
}

func mustAffect(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func main() {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	applied, err := migrate(ctx, db)
	fmt.Println("migrations applied:", applied, err)
	applied, err = migrate(ctx, db)
	fmt.Println("migrations applied:", applied, err)

	repo := &Repo{db: db, now: func() time.Time { return time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC) }}
	due := "2030-01-31"
	for _, t := range []Task{
		{Title: "Write docs", DueDate: &due, Tags: []string{"docs", "p1"}},
		{Title: "Fix bug", Tags: []string{"p1"}},
		{Title: "Ship it"},
	} {
		created, err := repo.Create(ctx, t)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("created %d %q\n", created.ID, created.Title)
	}

	got, err := repo.Get(ctx, 1)
	fmt.Printf("get 1: %q due=%s tags=%v at=%s %v\n", got.Title, *got.DueDate, got.Tags, got.CreatedAt.Format(time.RFC3339), err)
	_, err = repo.Get(ctx, 42)
	fmt.Println("get 42:", err, errors.Is(err, ErrNotFound))

	if err := repo.SetStatus(ctx, 2, "done"); err != nil {
		log.Fatal(err)
	}
	for _, f := range []ListFilter{{}, {Status: "done"}, {Tag: "p1"}, {Tag: "p1", Status: "todo"}, {Limit: 1, Offset: 2}} {
		tasks, err := repo.List(ctx, f)
		if err != nil {
			log.Fatal(err)
		}
		titles := []string{}
		for _, t := range tasks {
			titles = append(titles, t.Title)
		}
		fmt.Printf("list %+v -> %v\n", f, titles)
	}

	fmt.Println("delete 3:", repo.Delete(ctx, 3))
	fmt.Println("delete 3 again:", repo.Delete(ctx, 3))

	_, err = repo.Create(ctx, Task{Title: ""})
	fmt.Println("empty title:", err)
}
