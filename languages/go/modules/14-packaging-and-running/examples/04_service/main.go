// A deployable Task Tracker service: embedded seed data, health/readiness probes, build info,
// timeouts and graceful shutdown. See Dockerfile and ci.yml next to this file.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// Set with -ldflags "-X main.version=...".
var (
	version = "dev"
	commit  = "none"
)

//go:embed seed.json
var seedJSON []byte

type Task struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	DueDate   *string  `json:"due_date"`
	Tags      []string `json:"tags"`
	OwnerID   *int     `json:"owner_id"`
	CreatedAt string   `json:"created_at"`
}

type taskList struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type server struct {
	tasks []Task
	ready atomic.Bool
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s.ready.Load() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": version, "commit": commit})
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, taskList{Items: s.tasks, Total: len(s.tasks), Limit: 20})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func run(ctx context.Context, addr string) error {
	s := &server{}
	if err := json.Unmarshal(seedJSON, &s.tasks); err != nil {
		return fmt.Errorf("load seed data: %w", err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	s.ready.Store(true)
	slog.Info("listening", "addr", ln.Addr().String(), "version", version)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	// Fail readiness first so the orchestrator stops routing traffic, then drain.
	s.ready.Store(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	slog.Info("shutting down")
	return srv.Shutdown(shutdownCtx)
}

// probe is the container HEALTHCHECK: a distroless image has no curl or shell.
func probe(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz returned %d", resp.StatusCode)
	}
	return nil
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz on $ADDR and exit 0 (healthy) or 1")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	switch {
	case *showVersion:
		fmt.Println("tasks", version, commit)
	case *healthcheck:
		if err := probe(addr); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			os.Exit(1)
		}
	default:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := run(ctx, addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}
}
