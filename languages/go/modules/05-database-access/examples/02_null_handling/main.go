// Mapping rows to structs: NULL handling, JSON-encoded tags and text timestamps.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

type Task struct {
	ID        int
	Title     string
	DueDate   *string // nullable column -> pointer (nil = NULL)
	OwnerID   sql.Null[int64]
	Tags      []string
	CreatedAt time.Time
}

func main() {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	_, err = db.ExecContext(ctx, `CREATE TABLE tasks (
		id         INTEGER PRIMARY KEY,
		title      TEXT NOT NULL,
		due_date   TEXT,
		owner_id   INTEGER,
		tags       TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatal(err)
	}

	created := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	due := "2030-01-31"
	tags, _ := json.Marshal([]string{"docs", "p1"})
	// A nil *string and a zero sql.Null[T] are both written as NULL.
	insert := `INSERT INTO tasks (id, title, due_date, owner_id, tags, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := db.ExecContext(ctx, insert, 1, "With everything", &due, sql.Null[int64]{V: 7, Valid: true}, string(tags), created.Format(time.RFC3339)); err != nil {
		log.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, insert, 2, "Mostly empty", nil, sql.Null[int64]{}, "[]", created.Format(time.RFC3339)); err != nil {
		log.Fatal(err)
	}

	// Scanning NULL into a plain string or int is an error, not a zero value.
	var s string
	err = db.QueryRowContext(ctx, `SELECT due_date FROM tasks WHERE id = 2`).Scan(&s)
	fmt.Println("scan NULL into string:", err)

	rows, err := db.QueryContext(ctx, `SELECT id, title, due_date, owner_id, tags, created_at FROM tasks ORDER BY id`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			t        Task
			tagsJSON string
			created  string
		)
		if err := rows.Scan(&t.ID, &t.Title, &t.DueDate, &t.OwnerID, &tagsJSON, &created); err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
			log.Fatal(err)
		}
		if t.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d %q due=%s owner=%s tags=%v at=%s\n", t.ID, t.Title, show(t.DueDate), showNull(t.OwnerID), t.Tags, t.CreatedAt.Format(time.RFC3339))
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// NULL never equals anything in SQL, not even NULL: use IS NULL.
	fmt.Println("owner_id = NULL:", count(ctx, db, `SELECT COUNT(*) FROM tasks WHERE owner_id = NULL`))
	fmt.Println("owner_id IS NULL:", count(ctx, db, `SELECT COUNT(*) FROM tasks WHERE owner_id IS NULL`))
}

func count(ctx context.Context, db *sql.DB, query string) int {
	var n int
	if err := db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		log.Fatal(err)
	}
	return n
}

func show(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

func showNull(n sql.Null[int64]) string {
	if !n.Valid {
		return "null"
	}
	return fmt.Sprint(n.V)
}
