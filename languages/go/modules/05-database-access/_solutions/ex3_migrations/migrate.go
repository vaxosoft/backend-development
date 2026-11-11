// Package migrations is exercise 3: a small, safe schema migration runner.
package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
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
	sorted := slices.SortedFunc(slices.Values(migrations), func(a, b Migration) int { return a.Version - b.Version })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Version == sorted[i-1].Version {
			return nil, fmt.Errorf("version %d: %w", sorted[i].Version, ErrDuplicateVersion)
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	var applied []int
	for _, m := range sorted {
		var done bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)`, m.Version).Scan(&done); err != nil {
			return applied, err
		}
		if done {
			continue
		}
		if err := apply(ctx, db, m); err != nil {
			return applied, fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
		applied = append(applied, m.Version)
	}
	return applied, nil
}

func apply(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, m.Version); err != nil {
		return err
	}
	return tx.Commit()
}

// Version returns the highest applied version, or 0 when none (or no schema_migrations table) exists.
func Version(ctx context.Context, db *sql.DB) (int, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations')`).Scan(&exists)
	if err != nil || !exists {
		return 0, err
	}
	var v int
	err = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}
