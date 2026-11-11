// PostgreSQL is a driver swap. This example compiles everywhere but only talks to a
// database when DATABASE_URL is set, e.g. postgres://user:pass@localhost:5432/tasks?sslmode=disable
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

// Differences from SQLite: $1 placeholders, RETURNING instead of LastInsertId,
// identity columns, real timestamptz / date / text[] / jsonb types.
const (
	createTable = `CREATE TEMP TABLE tasks (
		id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		title      TEXT NOT NULL,
		due_date   DATE,
		tags       TEXT[] NOT NULL DEFAULT '{}',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`
	insertTask = `INSERT INTO tasks (title, due_date) VALUES ($1, $2) RETURNING id`
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Println("DATABASE_URL not set; skipping PostgreSQL demo")
		return
	}
	if err := run(url); err != nil {
		log.Fatal(err)
	}
}

func run(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	// A TEMP table lives on one connection, so pin one with db.Conn.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, createTable); err != nil {
		return err
	}
	var id int64
	if err := conn.QueryRowContext(ctx, insertTask, "Write docs", "2030-01-31").Scan(&id); err != nil {
		return err
	}
	var title string
	if err := conn.QueryRowContext(ctx, `SELECT title FROM tasks WHERE id = $1`, id).Scan(&title); err != nil {
		return err
	}
	fmt.Println("postgres round trip:", id, title)
	return nil
}
