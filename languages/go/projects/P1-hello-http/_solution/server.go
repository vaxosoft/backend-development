package starter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NewServer returns an http.Server for h with every timeout set.
// WriteTimeout must be longer than MaxSlow or /slow responses get cut off.
func NewServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Run serves h on ln until ctx is cancelled, then shuts down gracefully:
// the listener closes at once, in-flight requests may finish for up to
// drain, and Run returns nil on a clean shutdown. If the drain time is
// exceeded the remaining connections are closed and the error wraps
// context.DeadlineExceeded.
func Run(ctx context.Context, ln net.Listener, h http.Handler, drain time.Duration) error {
	srv := NewServer(h)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return err // the listener failed before any shutdown was requested
	case <-ctx.Done():
	}

	// ctx is already cancelled, so the drain deadline needs a fresh context.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), drain)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
