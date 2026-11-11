// Package middleware practises the func(http.Handler) http.Handler pattern.
package middleware

import (
	"context"
	"fmt"
	"net/http"
)

// Middleware wraps a handler with extra behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with the middleware so that the FIRST one listed is the
// OUTERMOST: Chain(h, A, B) runs A, then B, then h, then B and A on the way out.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type idKey struct{}

// RequestID makes sure each request has an id. It uses the incoming
// X-Request-ID header when it is 1-64 bytes long, otherwise it calls gen.
// The id is stored in the request context (see IDFrom) and echoed in the
// X-Request-ID response header.
func RequestID(gen func() string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if n := len(id); n < 1 || n > 64 {
				id = gen()
			}
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), idKey{}, id)))
		})
	}
}

// IDFrom returns the id stored by RequestID, or "" when there is none.
func IDFrom(ctx context.Context) string {
	id, _ := ctx.Value(idKey{}).(string)
	return id
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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}
				logf("request %s panicked: %v", IDFrom(r.Context()), v)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, `{"error":{"code":"internal","message":"internal server error"}}`)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// AccessLog calls sink(method, path, status) after each request completes.
// The status is whatever the handler wrote with WriteHeader, or 200 when it
// wrote a body (or nothing) without calling WriteHeader.
func AccessLog(sink func(method, path string, status int)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			sink(r.Method, r.URL.Path, rec.status)
		})
	}
}
