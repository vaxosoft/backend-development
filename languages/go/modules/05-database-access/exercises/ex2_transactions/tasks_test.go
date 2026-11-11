package transactions

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
	CREATE TABLE tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL CHECK (length(title) > 0));
	CREATE TABLE task_tags (task_id INTEGER NOT NULL REFERENCES tasks(id), tag TEXT NOT NULL, PRIMARY KEY (task_id, tag));`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s (is a transaction left open?): %v", table, err)
	}
	return n
}

func TestCreateWithTagsHappyPath(t *testing.T) {
	db := newDB(t)
	id, err := CreateWithTags(context.Background(), db, "Write docs", []string{"docs", "p1"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			t.Fatal(err)
		}
		tags = append(tags, tag)
	}
	if !slices.Equal(tags, []string{"docs", "p1"}) {
		t.Fatalf("got %v", tags)
	}
}

func TestCreateWithTagsIsAtomic(t *testing.T) {
	tests := []struct {
		name  string
		title string
		tags  []string
		is    error
	}{
		{"duplicate tag", "Dup", []string{"a", "b", "a"}, ErrDuplicateTag},
		{"empty title", "", []string{"a"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newDB(t)
			if _, err := CreateWithTags(context.Background(), db, "Existing", []string{"keep"}); err != nil {
				t.Fatal(err)
			}
			_, err := CreateWithTags(context.Background(), db, tt.title, tt.tags)
			if err == nil {
				t.Fatal("expected an error")
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Fatalf("want %v, got %v", tt.is, err)
			}
			if n := count(t, db, "tasks"); n != 1 {
				t.Fatalf("a failed call left %d task row(s), want 1", n)
			}
			if n := count(t, db, "task_tags"); n != 1 {
				t.Fatalf("a failed call left %d tag row(s), want 1", n)
			}
		})
	}
}

func TestPoolIsUsableAfterFailure(t *testing.T) {
	db := newDB(t)
	for range 3 {
		if _, err := CreateWithTags(context.Background(), db, "x", []string{"a", "a"}); err == nil {
			t.Fatal("expected an error")
		}
	}
	if in := db.Stats().InUse; in != 0 {
		t.Fatalf("%d connection(s) still in use: the transaction was never finished", in)
	}
	if _, err := CreateWithTags(context.Background(), db, "ok", nil); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledContextStoresNothing(t *testing.T) {
	db := newDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CreateWithTags(ctx, db, "never", []string{"a"}); err == nil {
		t.Fatal("expected an error")
	}
	if n := count(t, db, "tasks"); n != 0 {
		t.Fatalf("got %d rows", n)
	}
}
