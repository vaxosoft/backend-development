// Package starter is a small, fully tested Tasks service ready to be packaged.
package starter

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"webdev/projects/P7-tested-service-docker-ci/starter/tasks"
)

// Deps are the collaborators of the service; nil fields get defaults.
type Deps struct {
	Store  tasks.Store
	Logger *slog.Logger
}

// Service is the HTTP handler of the Tasks service.
type Service struct {
	handler  http.Handler
	store    tasks.Store
	log      *slog.Logger
	ready    atomic.Bool
	requests atomic.Int64
}

// New builds the service. It starts not ready: call SetReady(true) once
// start-up work is done.
func New(cfg Config, deps Deps) *Service {
	s := &Service{store: deps.Store, log: deps.Logger}
	if s.store == nil {
		s.store = tasks.NewMemoryStore()
	}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	s.log = s.log.With("env", cfg.Env)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /tasks", s.createTask)
	mux.HandleFunc("GET /tasks", s.listTasks)
	mux.HandleFunc("GET /tasks/{id}", s.getTask)
	// TODO: register GET /readyz and GET /metrics.
	s.handler = mux
	return s
}

// ServeHTTP implements http.Handler.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// SetReady flips the readiness reported by /readyz.
func (s *Service) SetReady(ready bool) { s.ready.Store(ready) }

func (s *Service) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// TODO: readyz answers 200 {"status":"ready"} once SetReady(true) was called
// and 503 with the error envelope (code "unavailable") before that.

// TODO: metrics answers 200 text/plain with the body "requests_total <n>\n",
// where n counts every request served so far, including this one.

// TODO: add middleware around the mux that (1) counts requests in s.requests
// and (2) logs one "request" line per request with method, path, status and
// duration_ms through s.log.

type createRequest struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

func (s *Service) createTask(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body", nil)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Status == "" {
		req.Status = tasks.StatusTodo
	}
	fields := map[string]string{}
	if n := utf8.RuneCountInString(req.Title); n < 1 || n > 200 {
		fields["title"] = "must be 1-200 characters"
	}
	switch req.Status {
	case tasks.StatusTodo, tasks.StatusInProgress, tasks.StatusDone:
	default:
		fields["status"] = "must be todo, in_progress or done"
	}
	if len(fields) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", fields)
		return
	}
	task, err := s.store.Create(r.Context(), tasks.Task{Title: req.Title, Status: req.Status})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.Header().Set("Location", "/tasks/"+strconv.Itoa(task.ID))
	writeJSON(w, http.StatusCreated, task)
}

func (s *Service) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "task not found", nil)
		return
	}
	task, err := s.store.Get(r.Context(), id)
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "task not found", nil)
	case err != nil:
		s.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, task)
	}
}

func (s *Service) listTasks(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.List(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// internalError logs the real cause and answers with a generic envelope.
func (s *Service) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.ErrorContext(r.Context(), "store failure", "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal error", nil)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	body := map[string]any{"code": code, "message": message}
	if fields != nil {
		body["fields"] = fields
	}
	writeJSON(w, status, map[string]any{"error": body})
}
