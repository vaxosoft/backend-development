// Package reqlog is exercise 2: request-ID and access-log middleware.
package reqlog

import (
	"context"
	"log/slog"
	"net/http"
)

// RequestID returns the request ID stored in ctx by Middleware, or "" when there is none.
func RequestID(ctx context.Context) string {
	panic("TODO")
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
	panic("TODO")
}
