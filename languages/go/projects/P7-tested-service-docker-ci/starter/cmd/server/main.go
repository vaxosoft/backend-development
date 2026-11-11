// Command server runs the Tasks service.
//
// With -healthcheck it instead probes /healthz on the local server and exits
// 0 or 1, which is how the Docker HEALTHCHECK works without curl in the image.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"webdev/projects/P7-tested-service-docker-ci/starter"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz and exit")
	flag.Parse()

	cfg, err := starter.LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(2)
	}
	addr := listenAddr(cfg, os.Getenv("ADDR"))

	if *healthcheck {
		os.Exit(probe(addr))
	}
	if err := run(cfg, addr); err != nil {
		fmt.Fprintln(os.Stderr, "server failed:", err)
		os.Exit(1)
	}
}

// listenAddr prefers ADDR (host:port) and falls back to loopback:PORT.
func listenAddr(cfg starter.Config, addr string) string {
	if addr != "" {
		return addr
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port))
}

func run(cfg starter.Config, addr string) error {
	opts := &slog.HandlerOptions{Level: cfg.SlogLevel()}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if cfg.Env == "production" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	log := slog.New(handler)

	svc := starter.New(cfg, starter.Deps{Logger: log})
	srv := &http.Server{
		Addr:              addr,
		Handler:           svc,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	svc.SetReady(true)
	log.Info("listening", "addr", ln.Addr().String(), "env", cfg.Env)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	svc.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// probe returns the process exit code for -healthcheck.
func probe(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
