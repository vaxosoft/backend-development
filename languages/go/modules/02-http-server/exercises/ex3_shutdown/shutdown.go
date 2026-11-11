// Package shutdown practises http.Server timeouts and graceful shutdown.
package shutdown

import (
	"context"
	"net"
	"net/http"
	"time"
)

// NewServer returns an http.Server for h with ALL of these timeouts set to a
// positive value: ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout.
func NewServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h} // TODO: a zero-value server has no timeouts
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
	panic("TODO")
}
