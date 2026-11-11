// Package starter implements an in-process job worker pool behind a small
// JSON HTTP API.
package starter

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
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
	go func() { _ = srv.Serve(ln) }()

	<-ctx.Done()
	// TODO: shut down gracefully instead of cutting everything off. Stop the
	// HTTP server with srv.Shutdown, then drain the pool with svc.Shutdown,
	// both bounded by cfg.DrainTimeout.
	return srv.Close()
}

// createJob handles POST /jobs.
func (s *Service) createJob(w http.ResponseWriter, _ *http.Request) {
	// TODO: decode the body (400 bad_request on malformed JSON, 413 when it is
	// over 1 MiB), validate (422 with fields), Submit, then answer 202
	// {"id":N} with a Location header. Map ErrQueueFull to 429 rate_limited
	// with Retry-After and ErrClosed to 503 unavailable.
	notImplemented(w)
}

// getJob handles GET /jobs/{id}.
func (s *Service) getJob(w http.ResponseWriter, _ *http.Request) {
	// TODO: 200 with the JobView; 404 not_found for unknown or non-numeric ids.
	notImplemented(w)
}

// listJobs handles GET /jobs.
func (s *Service) listJobs(w http.ResponseWriter, _ *http.Request) {
	// TODO: ?limit (default 20, clamped to 100) and ?offset (default 0);
	// negative or non-numeric values are 400 bad_request.
	// Body: {"items":[...],"total":N,"limit":L,"offset":O}, newest first.
	notImplemented(w)
}

// cancelJob handles DELETE /jobs/{id}.
func (s *Service) cancelJob(w http.ResponseWriter, _ *http.Request) {
	// TODO: 204 on success, 404 for unknown ids, 409 conflict when finished.
	notImplemented(w)
}

func notImplemented(w http.ResponseWriter) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "TODO: handler not implemented", nil)
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
