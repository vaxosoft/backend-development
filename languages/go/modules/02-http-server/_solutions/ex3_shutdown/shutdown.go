// Package shutdown practises http.Server timeouts and graceful shutdown.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NewServer returns an http.Server for h with ALL of these timeouts set to a
// positive value: ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout.
func NewServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Run serves h on ln (using NewServer) until ctx is cancelled, then shuts down gracefully:
//
//   - it stops accepting new connections immediately,
//   - requests already in flight are allowed to finish,
//   - it waits at most grace for them, measured from the moment ctx is cancelled.
//
// Return values:
//   - nil after a clean shutdown (http.ErrServerClosed is not an error here),
//   - an error wrapping context.DeadlineExceeded if requests were still running
//     after grace (remaining connections are then closed forcibly),
//   - the serve error if the server fails before ctx is cancelled
//     (for example because ln was already closed).
//
// Run must not return before the server has stopped.
func Run(ctx context.Context, ln net.Listener, h http.Handler, grace time.Duration) error {
	srv := NewServer(h)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return err // Serve only returns a non-nil error
	case <-ctx.Done():
	}

	// ctx is already cancelled, so the drain deadline needs a fresh context.
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer cancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		_ = srv.Close()
		<-errc
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
