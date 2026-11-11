// Package migrations is exercise 3: a small, safe schema migration runner.
package migrations

import (
	"context"
	"database/sql"
	"errors"
)

// ErrDuplicateVersion is returned when two migrations share a version.
var ErrDuplicateVersion = errors.New("duplicate migration version")

// Migration is one numbered schema change. SQL may contain several statements.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrate applies the migrations that are not yet recorded in the schema_migrations table
// (version INTEGER PRIMARY KEY) and returns the versions it applied, in order.
//
//   - Migrations run in ascending Version order, whatever order they are passed in.
//   - Duplicate versions fail with ErrDuplicateVersion before anything runs.
//   - Each migration and its schema_migrations row are one transaction: if its SQL fails
//     nothing from it remains, earlier migrations stay applied, later ones do not run,
//     and the error names the failing version.
//   - Calling Migrate again with the same list applies nothing.
func Migrate(ctx context.Context, db *sql.DB, migrations []Migration) ([]int, error) {
	panic("TODO")
}

// Version returns the highest applied version, or 0 when none (or no schema_migrations table) exists.
func Version(ctx context.Context, db *sql.DB) (int, error) {
	panic("TODO")
}
