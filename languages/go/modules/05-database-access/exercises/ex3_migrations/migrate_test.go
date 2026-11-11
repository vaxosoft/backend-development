package migrations

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
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
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

var all = []Migration{
	{1, "create tasks", `CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`},
	{2, "add status", `ALTER TABLE tasks ADD COLUMN status TEXT NOT NULL DEFAULT 'todo'`},
	{3, "create users", `CREATE TABLE users (id INTEGER PRIMARY KEY); INSERT INTO users (id) VALUES (1)`},
}

func TestMigrateAppliesInOrderAndIsIdempotent(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	got, err := Migrate(ctx, db, all)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("applied %v", got)
	}
	if v, err := Version(ctx, db); err != nil || v != 3 {
		t.Fatalf("Version = %d, %v", v, err)
	}
	if _, err := db.Exec(`INSERT INTO tasks (title, status) VALUES ('x', 'done')`); err != nil {
		t.Fatalf("schema not as expected: %v", err)
	}

	got, err = Migrate(ctx, db, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("second run applied %v", got)
	}
}

func TestMigrateOnlyAppliesNewOnes(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, db, all[:2]); err != nil {
		t.Fatal(err)
	}
	got, err := Migrate(ctx, db, all)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []int{3}) {
		t.Fatalf("applied %v", got)
	}
}

func TestMigrateSortsByVersion(t *testing.T) {
	db := newDB(t)
	got, err := Migrate(context.Background(), db, []Migration{all[1], all[0]})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("applied %v", got)
	}
}

func TestMigrateRejectsDuplicateVersionsBeforeRunning(t *testing.T) {
	db := newDB(t)
	dup := []Migration{all[0], {1, "again", `CREATE TABLE other (id INTEGER)`}}
	_, err := Migrate(context.Background(), db, dup)
	if !errors.Is(err, ErrDuplicateVersion) {
		t.Fatalf("want ErrDuplicateVersion, got %v", err)
	}
	if tableExists(t, db, "tasks") {
		t.Fatal("nothing should have run")
	}
}

func TestFailingMigrationRollsBackOnlyItself(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	broken := []Migration{
		all[0],
		{2, "half done", `CREATE TABLE half (id INTEGER); INSERT INTO missing_table VALUES (1)`},
		{3, "never runs", `CREATE TABLE never (id INTEGER)`},
	}
	got, err := Migrate(ctx, db, broken)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error should mention version 2: %v", err)
	}
	if !slices.Equal(got, []int{1}) {
		t.Fatalf("applied %v, want [1]", got)
	}
	if !tableExists(t, db, "tasks") {
		t.Fatal("migration 1 must stay applied")
	}
	if tableExists(t, db, "half") {
		t.Fatal("migration 2 must be rolled back completely")
	}
	if tableExists(t, db, "never") {
		t.Fatal("migration 3 must not run")
	}
	if v, err := Version(ctx, db); err != nil || v != 1 {
		t.Fatalf("Version = %d, %v", v, err)
	}
	if in := db.Stats().InUse; in != 0 {
		t.Fatalf("%d connection(s) leaked", in)
	}

	// Fixing migration 2 lets the run continue from where it stopped.
	fixed := []Migration{all[0], {2, "fixed", `CREATE TABLE half (id INTEGER)`}, broken[2]}
	got, err = Migrate(ctx, db, fixed)
	if err != nil || !slices.Equal(got, []int{2, 3}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestVersionOnEmptyDatabase(t *testing.T) {
	v, err := Version(context.Background(), newDB(t))
	if err != nil || v != 0 {
		t.Fatalf("Version = %d, %v", v, err)
	}
}
