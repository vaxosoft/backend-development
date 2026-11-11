// Package guard is exercise 3: panic recovery and basic request metrics.
package guard

import (
	"log/slog"
	"net/http"
	"sync/atomic"
)

// Metrics holds counters that are safe for concurrent use. The zero value is ready to use.
type Metrics struct {
	requests atomic.Int64
	panics   atomic.Int64
	inFlight atomic.Int64
}

// Middleware counts every request in requests_total when it starts and keeps in_flight accurate:
// +1 while a request is being served, -1 when it is done, even if the handler panics (the panic
// itself must keep propagating to whatever recovers it).
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	panic("TODO")
}

// Handler serves the counters as plain text, exactly these three lines in this order:
//
//	requests_total 3
//	panics_total 1
//	in_flight 0
//
// with Content-Type "text/plain; charset=utf-8".
func (m *Metrics) Handler() http.Handler {
	panic("TODO")
}

// Recover returns a middleware that turns a panic in the next handler into a 500 response.
//
//   - Log one record at Error level, message "panic recovered", with the attributes "panic"
//     (fmt.Sprint of the recovered value) and "stack" (the stack trace from runtime/debug).
//   - Respond with Content-Type application/json and the body
//     {"error":{"code":"internal","message":"internal server error"}} plus a newline. The panic value
//     must never appear in the response.
//   - If the handler had already started the response (written a header or body) the status cannot
//     change: do not append the error body.
//   - A panic with the value http.ErrAbortHandler is the documented way to abort a response: re-panic
//     with it, and neither log nor count it.
//   - Count each recovered panic in m.panics when m is not nil.
func Recover(logger *slog.Logger, m *Metrics) func(http.Handler) http.Handler {
	panic("TODO")
}
