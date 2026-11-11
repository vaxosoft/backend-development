// Resources, verbs, status codes and one JSON error envelope for every failure.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type api struct {
	mu    sync.Mutex
	next  int
	tasks map[int]Task
}

func (a *api) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", a.create)
	mux.HandleFunc("GET /tasks/{id}", a.get)
	mux.HandleFunc("DELETE /tasks/{id}", a.remove)

	// A method-less pattern is less specific than "GET /tasks/{id}", so it only
	// receives the remaining methods: that is how to answer 405 in JSON.
	mux.HandleFunc("/tasks/{id}", methodNotAllowed("GET, DELETE"))
	mux.HandleFunc("/tasks", methodNotAllowed("POST"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such route", nil)
	})
	return mux
}

func (a *api) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "request body is not valid JSON", nil)
		return
	}
	if title := strings.TrimSpace(in.Title); title == "" {
		// Well-formed JSON with invalid content is 422, not 400.
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task",
			map[string]string{"title": "must be 1-200 characters"})
		return
	}
	a.mu.Lock()
	a.next++
	t := Task{ID: a.next, Title: strings.TrimSpace(in.Title)}
	a.tasks[t.ID] = t
	a.mu.Unlock()

	w.Header().Set("Location", "/tasks/"+strconv.Itoa(t.ID))
	writeJSON(w, http.StatusCreated, t)
}

func (a *api) lookup(w http.ResponseWriter, r *http.Request) (Task, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "id must be an integer", nil)
		return Task{}, false
	}
	t, ok := a.tasks[id]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("task %d not found", id), nil)
	}
	return t, ok
}

func (a *api) get(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ok := a.lookup(w, r); ok {
		writeJSON(w, http.StatusOK, t)
	}
}

func (a *api) remove(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ok := a.lookup(w, r); ok {
		delete(a.tasks, t.ID)
		w.WriteHeader(http.StatusNoContent) // no body on 204
	}
}

func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not supported here", nil)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	writeJSON(w, status, map[string]errorBody{"error": {Code: code, Message: msg, Fields: fields}})
}

func main() {
	log.SetFlags(0)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	a := &api{tasks: map[int]Task{}}
	srv := &http.Server{Addr: addr, Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("listening on %s", addr)

	select {
	case err := <-errc:
		log.Fatal(err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	log.Print("bye")
}
