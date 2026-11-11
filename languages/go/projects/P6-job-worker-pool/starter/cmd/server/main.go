// Command server runs the job worker pool service.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"webdev/projects/P6-job-worker-pool/starter"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := starter.DefaultConfig()
	cfg.Logger = log
	cfg.Workers = envInt("WORKERS", cfg.Workers)
	cfg.QueueSize = envInt("QUEUE_SIZE", cfg.QueueSize)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Info("listening", "addr", ln.Addr().String(), "workers", cfg.Workers, "queue_size", cfg.QueueSize)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return starter.Run(ctx, ln, cfg)
}

func envInt(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
		return n
	}
	return def
}
