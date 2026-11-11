// Open a database, run parameterised statements and iterate over rows.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// Every connection to ":memory:" is its own empty database. Pin the pool to one.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE tasks (
		id     INTEGER PRIMARY KEY AUTOINCREMENT,
		title  TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'todo'
	)`)
	if err != nil {
		log.Fatal(err)
	}

	for _, title := range []string{"Write docs", "Fix bug", "Ship it"} {
		res, err := db.ExecContext(ctx, `INSERT INTO tasks (title) VALUES (?)`, title)
		if err != nil {
			log.Fatal(err)
		}
		id, _ := res.LastInsertId()
		fmt.Println("inserted", id)
	}

	res, err := db.ExecContext(ctx, `UPDATE tasks SET status = ? WHERE title = ?`, "done", "Fix bug")
	if err != nil {
		log.Fatal(err)
	}
	n, _ := res.RowsAffected()
	fmt.Println("updated", n)

	// One row: QueryRowContext defers errors to Scan; no rows is sql.ErrNoRows.
	var title string
	err = db.QueryRowContext(ctx, `SELECT title FROM tasks WHERE id = ?`, 2).Scan(&title)
	fmt.Println(title, err)
	err = db.QueryRowContext(ctx, `SELECT title FROM tasks WHERE id = ?`, 99).Scan(&title)
	fmt.Println(err == sql.ErrNoRows)

	// Many rows: always Close, and always check rows.Err() after the loop.
	rows, err := db.QueryContext(ctx, `SELECT id, title, status FROM tasks ORDER BY id`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id            int
			title, status string
		)
		if err := rows.Scan(&id, &title, &status); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d %-10s %s\n", id, title, status)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Never build SQL with fmt.Sprintf. A hostile value is just data when it is a parameter.
	hostile := "x'; DROP TABLE tasks; --"
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE title = ?`, hostile).Scan(&count); err != nil {
		log.Fatal(err)
	}
	fmt.Println("matches:", count)

	// A prepared statement is parsed once and reused; the pool handles it across connections.
	stmt, err := db.PrepareContext(ctx, `SELECT COUNT(*) FROM tasks WHERE status = ?`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	for _, s := range []string{"todo", "done"} {
		if err := stmt.QueryRowContext(ctx, s).Scan(&count); err != nil {
			log.Fatal(err)
		}
		fmt.Println(s, count)
	}

	stats := db.Stats()
	fmt.Println("open connections:", stats.OpenConnections, "max:", stats.MaxOpenConnections)
}
