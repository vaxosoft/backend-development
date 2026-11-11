// Package starter is the P2 tasks CRUD API: JSON over HTTP backed by SQLite.
package starter

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// OpenDB opens a SQLite database. A single connection is used on purpose: with
// ":memory:" every connection would otherwise be its own empty database.
func OpenDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// New creates the schema and returns the fully routed API. It is complete:
// your work is in task.go, store.go and handlers.go.
func New(db *sql.DB) (http.Handler, error) {
	store := NewSQLiteStore(db)
	if err := store.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	api := &API{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", api.create)
	mux.HandleFunc("GET /tasks", api.list)
	mux.HandleFunc("GET /tasks/{id}", api.get)
	mux.HandleFunc("PUT /tasks/{id}", api.replace)
	mux.HandleFunc("PATCH /tasks/{id}", api.patch)
	mux.HandleFunc("DELETE /tasks/{id}", api.delete)
	return mux, nil
}
