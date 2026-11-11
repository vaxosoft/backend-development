package queryrows

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"

	_ "modernc.org/sqlite"
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL,
		due_date TEXT, owner_id INTEGER);
	INSERT INTO tasks VALUES
		(1, 'Write docs', 'todo', '2030-01-31', 7),
		(2, 'Fix bug', 'done', NULL, NULL),
		(3, 'Ship it', 'todo', NULL, 7),
		(4, 'Update DOCS site', 'in_progress', '2030-02-01', NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func ids(tasks []Task) []int64 {
	out := []int64{}
	for _, t := range tasks {
		out = append(out, t.ID)
	}
	return out
}

func TestGetMapsNulls(t *testing.T) {
	db := newDB(t)
	full, err := Get(context.Background(), db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if full.Title != "Write docs" || full.Status != "todo" || full.DueDate == nil || *full.DueDate != "2030-01-31" ||
		full.OwnerID == nil || *full.OwnerID != 7 {
		t.Fatalf("unexpected task %+v", full)
	}
	empty, err := Get(context.Background(), db, 2)
	if err != nil {
		t.Fatal(err)
	}
	if empty.DueDate != nil || empty.OwnerID != nil || empty.Title != "Fix bug" {
		t.Fatalf("NULL columns must map to nil pointers, got %+v", empty)
	}
}

func TestGetNotFound(t *testing.T) {
	_, err := Get(context.Background(), newDB(t), 99)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	db := newDB(t)
	tests := []struct {
		status string
		want   []int64
	}{
		{"", []int64{1, 2, 3, 4}},
		{"todo", []int64{1, 3}},
		{"done", []int64{2}},
		{"nope", []int64{}},
	}
	for _, tt := range tests {
		t.Run("status="+tt.status, func(t *testing.T) {
			got, err := List(context.Background(), db, tt.status)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("result must not be nil")
			}
			if !slices.Equal(ids(got), tt.want) {
				t.Fatalf("got %v, want %v", ids(got), tt.want)
			}
		})
	}
}

func TestListHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := List(ctx, newDB(t), ""); err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
}

func TestSearchTitle(t *testing.T) {
	db := newDB(t)
	got, err := SearchTitle(context.Background(), db, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), []int64{1, 4}) {
		t.Fatalf("got %v", ids(got))
	}
}

func TestSearchTitleIsInjectionSafe(t *testing.T) {
	db := newDB(t)
	got, err := SearchTitle(context.Background(), db, "x'; DROP TABLE tasks; --")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("tasks table damaged: n=%d err=%v", n, err)
	}
}

func TestConnectionsAreReleased(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	for range 3 {
		if _, err := List(ctx, db, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := Get(ctx, db, 99); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err := SearchTitle(ctx, db, "a"); err != nil {
			t.Fatal(err)
		}
	}
	if in := db.Stats().InUse; in != 0 {
		t.Fatalf("%d connection(s) still in use: rows were not closed", in)
	}
}
