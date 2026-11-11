package taskstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	_ "modernc.org/sqlite"
)

// testStore is the contract: every Store must pass it. newStore returns a fresh, empty store.
func testStore(t *testing.T, newStore func(t *testing.T) Store) {
	ctx := context.Background()

	t.Run("create assigns increasing ids and defaults", func(t *testing.T) {
		s := newStore(t)
		a, err := s.Create(ctx, Task{Title: "a"})
		if err != nil {
			t.Fatal(err)
		}
		b, _ := s.Create(ctx, Task{Title: "b", Status: "done", Tags: []string{"p1"}})
		if a.ID != 1 || b.ID != 2 || a.Status != "todo" || b.Status != "done" {
			t.Fatalf("a = %+v, b = %+v", a, b)
		}
	})

	t.Run("get returns what was stored", func(t *testing.T) {
		s := newStore(t)
		created, _ := s.Create(ctx, Task{Title: "Write docs", Tags: []string{"docs", "p1"}})
		got, err := s.Get(ctx, created.ID)
		if err != nil || !reflect.DeepEqual(got, created) {
			t.Fatalf("Get = %+v, %v; want %+v", got, err, created)
		}
	})

	t.Run("no tags round-trips as empty, not nil", func(t *testing.T) {
		s := newStore(t)
		created, _ := s.Create(ctx, Task{Title: "x"})
		got, _ := s.Get(ctx, created.ID)
		if got.Tags == nil || len(got.Tags) != 0 {
			t.Fatalf("Tags = %#v", got.Tags)
		}
	})

	t.Run("callers cannot change stored data", func(t *testing.T) {
		s := newStore(t)
		tags := []string{"a"}
		created, _ := s.Create(ctx, Task{Title: "x", Tags: tags})
		tags[0] = "mutated"
		got, _ := s.Get(ctx, created.ID)
		got.Tags[0] = "mutated too"
		again, _ := s.Get(ctx, created.ID)
		if !slices.Equal(again.Tags, []string{"a"}) {
			t.Fatalf("stored tags changed: %v", again.Tags)
		}
	})

	t.Run("missing ids report ErrNotFound", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Get(ctx, 99); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get: %v", err)
		}
		if err := s.Delete(ctx, 99); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete: %v", err)
		}
	})

	t.Run("list is ordered and filterable", func(t *testing.T) {
		s := newStore(t)
		for _, st := range []string{"todo", "done", "todo", "done"} {
			s.Create(ctx, Task{Title: "t", Status: st})
		}
		all, _ := s.List(ctx, "")
		done, _ := s.List(ctx, "done")
		none, err := s.List(ctx, "in_progress")
		ids := func(ts []Task) []int {
			out := []int{}
			for _, task := range ts {
				out = append(out, task.ID)
			}
			return out
		}
		if !slices.Equal(ids(all), []int{1, 2, 3, 4}) || !slices.Equal(ids(done), []int{2, 4}) {
			t.Errorf("all = %v, done = %v", ids(all), ids(done))
		}
		if err != nil || none == nil || len(none) != 0 {
			t.Errorf("no matches should be an empty non-nil slice: %#v, %v", none, err)
		}
	})

	t.Run("delete removes only that task", func(t *testing.T) {
		s := newStore(t)
		s.Create(ctx, Task{Title: "a"})
		b, _ := s.Create(ctx, Task{Title: "b"})
		if err := s.Delete(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("deleted task still readable: %v", err)
		}
		if got, err := s.Get(ctx, b.ID); err != nil || got.Title != "b" {
			t.Errorf("other task: %+v, %v", got, err)
		}
	})
}

func TestMemStore(t *testing.T) {
	testStore(t, func(*testing.T) Store { return NewMemStore() })
}

func TestSQLiteStore(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: skipped with -short")
	}
	testStore(t, func(t *testing.T) Store {
		// A file in t.TempDir is private to this subtest and deleted afterwards. (An in-memory
		// SQLite database would be a different, empty database on every pooled connection.)
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "tasks.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		s, err := NewSQLiteStore(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestSQLiteStorePersistsAcrossReopen(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: skipped with -short")
	}
	path := filepath.Join(t.TempDir(), "tasks.db")
	open := func() *SQLiteStore {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		s, err := NewSQLiteStore(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	first := open()
	created, _ := first.Create(context.Background(), Task{Title: "survives", Tags: []string{"x"}})
	second := open()
	got, err := second.Get(context.Background(), created.ID)
	if err != nil || got.Title != "survives" {
		t.Fatalf("after reopen: %+v, %v", got, err)
	}
}
