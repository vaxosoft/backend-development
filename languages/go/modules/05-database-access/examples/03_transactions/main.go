// Transactions: commit, rollback on error, isolation options and the "used db instead of tx" trap.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// withTx runs fn inside a transaction: commit on nil, roll back on error or panic.
func withTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // no-op after a successful Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func mustExec(ctx context.Context, x interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, q string, args ...any) {
	if _, err := x.ExecContext(ctx, q, args...); err != nil {
		log.Fatal(err)
	}
}

func owners(ctx context.Context, db *sql.DB) string {
	rows, err := db.QueryContext(ctx, `SELECT id, owner_id FROM tasks ORDER BY id`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	out := ""
	for rows.Next() {
		var id, owner int
		if err := rows.Scan(&id, &owner); err != nil {
			log.Fatal(err)
		}
		out += fmt.Sprintf("task%d->user%d ", id, owner)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	return out
}

func main() {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	mustExec(ctx, db, `CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE)`)
	mustExec(ctx, db, `CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL,
		owner_id INTEGER NOT NULL REFERENCES users(id))`)
	mustExec(ctx, db, `INSERT INTO users (id, email) VALUES (1, 'ann@example.com'), (2, 'bob@example.com')`)
	mustExec(ctx, db, `INSERT INTO tasks (id, title, owner_id) VALUES (1, 'a', 1), (2, 'b', 1), (3, 'c', 2)`)
	fmt.Println("start:   ", owners(ctx, db))

	// Both statements succeed or neither does.
	err = withTx(ctx, db, nil, func(tx *sql.Tx) error {
		mustExec(ctx, tx, `UPDATE tasks SET owner_id = 2 WHERE owner_id = 1`)
		_, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = 1`)
		return err
	})
	fmt.Println("commit:  ", err, owners(ctx, db))

	// The DELETE succeeds, then the function fails: the deferred Rollback restores the rows.
	err = withTx(ctx, db, nil, func(tx *sql.Tx) error {
		mustExec(ctx, tx, `DELETE FROM tasks`)
		return errors.New("simulated failure after the first statement")
	})
	fmt.Println("rollback:", err, owners(ctx, db))

	// Second INSERT violates UNIQUE(email), so the first INSERT is undone as well.
	err = withTx(ctx, db, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, email) VALUES (3, 'cy@example.com')`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO users (id, email) VALUES (4, 'cy@example.com')`)
		return err
	})
	var n int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	fmt.Println("unique:  ", err != nil, "users:", n)

	// Foreign keys are enforced because the DSN enabled the pragma.
	_, err = db.ExecContext(ctx, `INSERT INTO tasks (id, title, owner_id) VALUES (9, 'orphan', 42)`)
	fmt.Println("fk:      ", err)

	// Isolation level and read-only are requests passed to the driver. SQLite serializes
	// writers anyway; PostgreSQL honours the level (default read committed).
	err = withTx(ctx, db, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true}, func(tx *sql.Tx) error { return nil })
	fmt.Println("serializable read-only:", err)

	// Trap: inside a transaction, use tx, not db. With a pool of one this waits forever.
	tctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	err = withTx(tctx, db, nil, func(tx *sql.Tx) error {
		return db.QueryRowContext(tctx, `SELECT COUNT(*) FROM tasks`).Scan(&n)
	})
	fmt.Println("db inside tx:", err)
}
