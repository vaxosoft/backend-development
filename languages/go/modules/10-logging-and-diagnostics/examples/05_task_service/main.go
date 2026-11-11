// Example 05: Task Tracker service with structured JSON logs, request IDs, panic recovery, metrics and
// a pprof admin listener that is separate from the public one.
//
//	ADDR=127.0.0.1:8080 ADMIN_ADDR=127.0.0.1:6060 go run ./modules/10-logging-and-diagnostics/examples/05_task_service
//
// LOG_STABLE=1 removes timestamps and durations from the log lines (useful for docs and tests).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type ctxKey struct{}

func requestID(ctx context.Context) string { id, _ := ctx.Value(ctxKey{}).(string); return id }

type idHandler struct{ slog.Handler }

func (h idHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := requestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}
func (h idHandler) WithAttrs(a []slog.Attr) slog.Handler { return idHandler{h.Handler.WithAttrs(a)} }
func (h idHandler) WithGroup(n string) slog.Handler      { return idHandler{h.Handler.WithGroup(n)} }

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

type app struct {
	logger   *slog.Logger
	requests atomic.Int64
	panics   atomic.Int64
	ready    atomic.Bool
	mu       sync.Mutex
	tasks    []map[string]any
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// middleware stack, outermost first: request ID -> access log -> recover -> handler.
func (a *app) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKey{}, id)
		r = r.WithContext(ctx)
		a.requests.Add(1)

		start := time.Now()
		rec := &recorder{ResponseWriter: w}
		defer func() {
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			a.logger.InfoContext(ctx, "request", "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				a.panics.Add(1)
				a.logger.ErrorContext(ctx, "panic recovered", "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
				writeJSON(rec, http.StatusInternalServerError, map[string]any{
					"error": map[string]any{"code": "internal", "message": "internal server error"}})
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !a.ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ready")
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "requests_total %d\npanics_total %d\n", a.requests.Load(), a.panics.Load())
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"items": append([]map[string]any{}, a.tasks...)})
	})
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil || in.Title == "" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{
				"code": "validation_failed", "message": "invalid task", "fields": map[string]string{"title": "required"}}})
			return
		}
		a.mu.Lock()
		t := map[string]any{"id": len(a.tasks) + 1, "title": in.Title}
		a.tasks = append(a.tasks, t)
		a.mu.Unlock()
		a.logger.InfoContext(r.Context(), "task created", "id", t["id"]) // request_id is added by the handler
		writeJSON(w, http.StatusCreated, t)
	})
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) { panic("demo panic") })
	return a.wrap(mux)
}

// adminMux is served on a different, loopback-only address: profiles expose internals and cost CPU.
func adminMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/vars", expvar.Handler())
	return mux
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	opts := &slog.HandlerOptions{}
	if os.Getenv("LOG_STABLE") == "1" {
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey, "duration_ms", "stack":
				return slog.Attr{}
			}
			return a
		}
	}
	logger := slog.New(idHandler{slog.NewJSONHandler(os.Stdout, opts)})

	a := &app{logger: logger}
	public := &http.Server{
		Addr:              env("ADDR", "127.0.0.1:8080"),
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	admin := &http.Server{
		Addr:              env("ADMIN_ADDR", "127.0.0.1:6060"),
		Handler:           adminMux(),
		ReadHeaderTimeout: 5 * time.Second,
		// no WriteTimeout: CPU profiles take 30 s by default
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 2)
	for _, s := range []*http.Server{public, admin} {
		go func() { errc <- s.ListenAndServe() }()
	}
	a.ready.Store(true)
	logger.Info("listening", "addr", public.Addr, "admin_addr", admin.Addr)

	select {
	case err := <-errc:
		logger.Error("server failed", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	a.ready.Store(false)
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range []*http.Server{public, admin} {
		if err := s.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("shutdown", "err", err)
		}
	}
	logger.Info("stopped")
}
