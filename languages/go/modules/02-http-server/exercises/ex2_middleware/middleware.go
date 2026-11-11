// Package middleware practises the func(http.Handler) http.Handler pattern.
package middleware

import (
	"context"
	"net/http"
)

// Middleware wraps a handler with extra behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with the middleware so that the FIRST one listed is the
// OUTERMOST: Chain(h, A, B) runs A, then B, then h, then B and A on the way out.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	return h // TODO
}

// RequestID makes sure each request has an id. It uses the incoming
// X-Request-ID header when it is 1-64 bytes long, otherwise it calls gen.
// The id is stored in the request context (see IDFrom) and echoed in the
// X-Request-ID response header.
func RequestID(gen func() string) Middleware {
	return func(next http.Handler) http.Handler { return next } // TODO
}

// IDFrom returns the id stored by RequestID, or "" when there is none.
func IDFrom(ctx context.Context) string {
	return "" // TODO
}

// Recover turns a panic in the downstream handler into a 500 response with
// Content-Type application/json and this body (a trailing newline is fine):
//
//	{"error":{"code":"internal","message":"internal server error"}}
//
// The panic value must never be sent to the client. It is reported once via
// logf(format, args...) in a way that includes the panic value and the request id.
// A panic with http.ErrAbortHandler must be re-panicked unchanged.
func Recover(logf func(format string, args ...any)) Middleware {
	return func(next http.Handler) http.Handler { return next } // TODO
}

// AccessLog calls sink(method, path, status) after each request completes.
// The status is whatever the handler wrote with WriteHeader, or 200 when it
// wrote a body (or nothing) without calling WriteHeader.
func AccessLog(sink func(method, path string, status int)) Middleware {
	return func(next http.Handler) http.Handler { return next } // TODO
}
