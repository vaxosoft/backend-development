// Package starter implements an in-process job worker pool behind a small
// JSON HTTP API.
package starter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Config controls the pool and the HTTP service.
type Config struct {
	Workers      int           // goroutines pulling from the queue
	QueueSize    int           // jobs waiting for a worker
	MaxAttempts  int           // total attempts per job (first run + retries)
	RetryDelay   time.Duration // pause between attempts
	JobTimeout   time.Duration // limit for one attempt
	DrainTimeout time.Duration // how long Run waits for jobs on shutdown
	Logger       *slog.Logger  // nil discards logs
}

// DefaultConfig returns production-like defaults.
func DefaultConfig() Config {
	return Config{
		Workers:      4,
		QueueSize:    64,
		MaxAttempts:  3,
		RetryDelay:   500 * time.Millisecond,
		JobTimeout:   30 * time.Second,
		DrainTimeout: 10 * time.Second,
	}
}

// Service is the HTTP front end of a Pool.
type Service struct {
	pool *Pool
	mux  *http.ServeMux
}

// New starts the worker pool and returns the HTTP handler. Call Shutdown when
// done to stop the workers.
func New(cfg Config) *Service {
	cfg.Workers = max(cfg.Workers, 1)
	cfg.MaxAttempts = max(cfg.MaxAttempts, 1)
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Service{pool: NewPool(cfg), mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /jobs", s.createJob)
	s.mux.HandleFunc("GET /jobs", s.listJobs)
	s.mux.HandleFunc("GET /jobs/{id}", s.getJob)
	s.mux.HandleFunc("DELETE /jobs/{id}", s.cancelJob)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Shutdown stops accepting jobs and drains the pool (see Pool.Shutdown).
func (s *Service) Shutdown(ctx context.Context) error { return s.pool.Shutdown(ctx) }

// Run serves on ln until ctx is cancelled, then shuts down gracefully: the
// HTTP server stops accepting, and queued/running jobs get cfg.DrainTimeout
// to finish.
func Run(ctx context.Context, ln net.Listener, cfg Config) error {
	svc := New(cfg)
	srv := &http.Server{
		Handler:           svc,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return errors.Join(err, svc.Shutdown(context.Background()))
	case <-ctx.Done():
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), cfg.DrainTimeout)
	defer cancel()
	return errors.Join(srv.Shutdown(drainCtx), svc.Shutdown(drainCtx))
}

type createRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (s *Service) createJob(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large", nil)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body", nil)
		return
	}
	if fields := validate(req.Type, req.Payload); len(fields) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid job", fields)
		return
	}
	id, err := s.pool.Submit(req.Type, req.Payload)
	switch {
	case errors.Is(err, ErrQueueFull):
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "job queue is full", nil)
	case errors.Is(err, ErrClosed):
		writeError(w, http.StatusServiceUnavailable, "unavailable", "shutting down", nil)
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", "internal error", nil)
	default:
		w.Header().Set("Location", "/jobs/"+strconv.Itoa(id))
		writeJSON(w, http.StatusAccepted, map[string]int{"id": id})
	}
}

func (s *Service) getJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "job not found", nil)
		return
	}
	view, ok := s.pool.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "job not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Service) listJobs(w http.ResponseWriter, r *http.Request) {
	limit, err1 := intParam(r, "limit", 20)
	offset, err2 := intParam(r, "offset", 0)
	if err := errors.Join(err1, err2); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "limit and offset must be non-negative integers", nil)
		return
	}
	limit = min(limit, 100)
	items, total := s.pool.List(limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (s *Service) cancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "job not found", nil)
		return
	}
	switch err := s.pool.Cancel(id); {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "job not found", nil)
	case errors.Is(err, ErrFinished):
		writeError(w, http.StatusConflict, "conflict", "job already finished", nil)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// intParam reads a non-negative integer query parameter.
func intParam(r *http.Request, name string, def int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errors.New("invalid " + name)
	}
	return n, nil
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
