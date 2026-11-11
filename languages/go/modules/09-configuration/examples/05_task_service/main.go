// Example 05: Task Tracker service wired by hand: config -> dependencies -> handler (the composition root is main).
//
//	ADDR=127.0.0.1:8080 LOG_LEVEL=info API_TOKEN=s3cret go run ./modules/09-configuration/examples/05_task_service
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Secret string

func (Secret) String() string                 { return "[redacted]" }
func (s Secret) MarshalText() ([]byte, error) { return []byte("[redacted]"), nil }
func (s Secret) LogValue() slog.Value         { return slog.StringValue("[redacted]") }

type Config struct {
	Addr     string `json:"addr"`
	LogLevel string `json:"log_level"`
	APIToken Secret `json:"api_token"`
}

type Lookup func(string) (string, bool)

// LoadConfig reads the environment exactly once, here; nothing else in the program calls os.Getenv.
func LoadConfig(lookup Lookup) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && v != "" {
			return v
		}
		return def
	}
	cfg := Config{
		Addr:     get("ADDR", "127.0.0.1:8080"),
		LogLevel: get("LOG_LEVEL", "info"),
		APIToken: Secret(get("API_TOKEN", "")),
	}
	var errs []error
	if _, err := parseLevel(cfg.LogLevel); err != nil {
		errs = append(errs, err)
	}
	if cfg.APIToken == "" {
		errs = append(errs, errors.New("API_TOKEN is required"))
	}
	return cfg, errors.Join(errs...)
}

func parseLevel(s string) (slog.Level, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL %q: want debug|info|warn|error", s)
	}
	return l, nil
}

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// Store is declared where it is used; any implementation (memory, SQL) satisfies it.
type Store interface {
	Add(title string) Task
	All() []Task
}

type memStore struct {
	mu    sync.Mutex
	tasks []Task
}

func (m *memStore) Add(title string) Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := Task{ID: len(m.tasks) + 1, Title: title}
	m.tasks = append(m.tasks, t)
	return t
}

func (m *memStore) All() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Task{}, m.tasks...)
}

// NewHandler receives everything it needs as arguments: no globals, trivial to test.
func NewHandler(cfg Config, store Store, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(v)
	}

	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, cfg) // the Secret type redacts itself
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"items": store.All()})
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
		t := store.Add(in.Title)
		logger.Info("task created", "id", t.ID)
		w.Header().Set("Location", fmt.Sprintf("/tasks/%d", t.ID))
		writeJSON(w, http.StatusCreated, t)
	})

	// Authentication wraps only the routes that need it.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/tasks") && r.Header.Get("Authorization") != "Bearer "+string(cfg.APIToken) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{
				"code": "unauthorized", "message": "missing or invalid credentials"}})
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func main() {
	cfg, err := LoadConfig(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error:\n%v\n", err)
		os.Exit(2)
	}
	level, _ := parseLevel(cfg.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	logger.Info("starting", "addr", cfg.Addr, "log_level", cfg.LogLevel, "api_token", cfg.APIToken)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           NewHandler(cfg, &memStore{}, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case err := <-errc:
		logger.Error("server failed", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("stopped")
}
