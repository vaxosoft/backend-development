package starter

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// NewServer returns an http.Server for h with every timeout set.
// WriteTimeout must be longer than MaxSlow or /slow responses get cut off.
func NewServer(h http.Handler) *http.Server {
	// TODO: set ReadHeaderTimeout, ReadTimeout, WriteTimeout and IdleTimeout.
	return &http.Server{Handler: h}
}

// Run serves h on ln until ctx is cancelled, then shuts down gracefully:
// the listener closes at once, in-flight requests may finish for up to
// drain, and Run returns nil on a clean shutdown. If the drain time is
// exceeded the remaining connections are closed and the error wraps
// context.DeadlineExceeded.
func Run(ctx context.Context, ln net.Listener, h http.Handler, drain time.Duration) error {
	// TODO: Serve in a goroutine, wait for ctx.Done(), call Shutdown with a
	// timeout of drain. http.ErrServerClosed is not an error.
	_, _, _, _ = ctx, ln, h, drain
	return errors.New("TODO: Run is not implemented")
}
