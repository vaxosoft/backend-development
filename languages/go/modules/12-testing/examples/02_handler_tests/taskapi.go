// Package taskapi is example 02: a tiny Task Tracker handler that is tested without opening a port
// (httptest.NewRecorder) and once over real loopback HTTP (httptest.NewServer).
//
//	go test -v ./modules/12-testing/examples/02_handler_tests/
package taskapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Task struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type API struct {
	mu    sync.Mutex
	tasks []Task
}

// New returns the handler for POST /tasks, GET /tasks/{id} and GET /tasks?status=.
func New() http.Handler {
	a := &API{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", a.create)
	mux.HandleFunc("GET /tasks/{id}", a.get)
	mux.HandleFunc("GET /tasks", a.list)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	e := map[string]any{"code": code, "message": msg}
	if fields != nil {
		e["fields"] = fields
	}
	writeJSON(w, status, map[string]any{"error": e})
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body", nil)
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Status == "" {
		in.Status = "todo"
	}
	fields := map[string]string{}
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		fields["title"] = "must be 1-200 characters"
	}
	if in.Status != "todo" && in.Status != "in_progress" && in.Status != "done" {
		fields["status"] = "must be todo, in_progress or done"
	}
	if len(fields) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", fields)
		return
	}
	a.mu.Lock()
	t := Task{ID: len(a.tasks) + 1, Title: in.Title, Status: in.Status}
	a.tasks = append(a.tasks, t)
	a.mu.Unlock()
	w.Header().Set("Location", fmt.Sprintf("/tasks/%d", t.ID))
	writeJSON(w, http.StatusCreated, t)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	if id, err := strconv.Atoi(r.PathValue("id")); err == nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		if id >= 1 && id <= len(a.tasks) {
			writeJSON(w, http.StatusOK, a.tasks[id-1])
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "task not found", nil)
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	a.mu.Lock()
	items := []Task{}
	for _, t := range a.tasks {
		if status == "" || t.Status == status {
			items = append(items, t)
		}
	}
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}
