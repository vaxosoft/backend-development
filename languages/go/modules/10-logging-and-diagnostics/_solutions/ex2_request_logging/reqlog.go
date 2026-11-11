// Package reqlog is exercise 2: request-ID and access-log middleware.
package reqlog

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type ctxKey struct{}

// RequestID returns the request ID stored in ctx by Middleware, or "" when there is none.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func acceptable(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] <= ' ' || id[i] > '~' {
			return false
		}
	}
	return true
}

// Middleware returns a middleware that, for every request:
//
//  1. Takes the request ID from the X-Request-ID header, or calls newID() when the header is missing
//     or unacceptable. Acceptable means 1-64 bytes of visible ASCII (no spaces or control characters):
//     anything else could forge log lines.
//  2. Sets the X-Request-ID response header (before the handler runs) and stores the ID in the request
//     context for RequestID.
//  3. After the handler returns, writes exactly one record with message "request" and the attributes
//     method (string), path (string, r.URL.Path only: never the query string), status (int, 200 when the
//     handler never wrote a header), bytes (int, response body bytes written), duration_ms (int64, >= 0)
//     and request_id (string).
//     The record level is Error for status >= 500, Warn for status >= 400 and Info otherwise.
//
// The ResponseWriter handed to the next handler must keep working with http.NewResponseController
// (for example Flush).
func Middleware(logger *slog.Logger, newID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if !acceptable(id) {
				id = newID()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), ctxKey{}, id)

			start := time.Now()
			rec := &recorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}
			logger.LogAttrs(ctx, level, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", rec.bytes),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("request_id", id),
			)
		})
	}
}
