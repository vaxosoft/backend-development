package starter

import (
	"context"
	"io"
	"net/http"
)

// Middleware wraps a handler with cross-cutting behaviour.
type Middleware func(http.Handler) http.Handler

// Chain applies mws to h; the first middleware is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestIDHeader is the header used to carry the request ID.
const RequestIDHeader = "X-Request-ID"

type ctxKey struct{}

// RequestIDFrom returns the request ID stored by the RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// RequestID echoes a valid incoming X-Request-ID (at most 64 printable ASCII
// characters) or generates a new random one, sets it on the response and
// stores it in the request context.
func RequestID(next http.Handler) http.Handler {
	// TODO: pick the ID, set the response header, add it to the context with
	// context.WithValue(ctx, ctxKey{}, id).
	return next
}

// Logging writes one line per request to sink after the handler returns:
// request_id, method, path, status and duration. Use log/slog.
func Logging(sink io.Writer) Middleware {
	_ = sink
	return func(next http.Handler) http.Handler {
		// TODO: wrap w in a *statusRecorder, time the call, log the line.
		return next
	}
}

// Recover turns a panic in a handler into a 500 "internal" error envelope and
// keeps the server alive. http.ErrAbortHandler must be re-panicked.
func Recover(next http.Handler) http.Handler {
	// TODO: defer a recover() that writes the envelope (never leak the panic value).
	return next
}

// statusRecorder remembers the status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the real ResponseWriter.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
