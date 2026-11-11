// Package transactions is exercise 2: make a multi-statement write atomic.
package transactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Schema is applied by the tests:
//
//	CREATE TABLE tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL CHECK (length(title) > 0));
//	CREATE TABLE task_tags (task_id INTEGER NOT NULL REFERENCES tasks(id), tag TEXT NOT NULL,
//	                        PRIMARY KEY (task_id, tag));

// ErrDuplicateTag is returned when the same tag appears twice in one call.
var ErrDuplicateTag = errors.New("duplicate tag")

// CreateWithTags inserts a task and one task_tags row per tag and returns the new id.
// It must be atomic: if any insert fails (empty title, duplicate tag, ...) nothing is stored.
// A duplicate tag must be reported as an error wrapping ErrDuplicateTag.
func CreateWithTags(ctx context.Context, db *sql.DB, title string, tags []string) (int64, error) {
	res, err := db.ExecContext(ctx, `INSERT INTO tasks (title) VALUES (?)`, title)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, tag := range tags {
		if seen[tag] {
			return 0, fmt.Errorf("tag %q: %w", tag, ErrDuplicateTag)
		}
		seen[tag] = true
		if _, err := db.ExecContext(ctx, `INSERT INTO task_tags (task_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return 0, fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}
	return id, nil
}
