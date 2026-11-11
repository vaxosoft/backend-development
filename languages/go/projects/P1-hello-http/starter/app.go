// Package starter is the P1 hello-http server: routing, middleware and
// graceful shutdown.
package starter

import (
	"io"
	"net/http"
)

// Config holds everything New needs. The zero value is usable.
type Config struct {
	// LogSink receives one request-log line per request. nil discards logs.
	LogSink io.Writer
}

// New wires the routes and the middleware chain. It is complete: your work is
// in handlers.go, middleware.go and server.go.
func New(cfg Config) http.Handler {
	sink := cfg.LogSink
	if sink == nil {
		sink = io.Discard
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Healthz)
	mux.HandleFunc("GET /hello", Hello)
	mux.HandleFunc("GET /hello/{name}", Hello)
	mux.HandleFunc("GET /slow", Slow)
	mux.HandleFunc("GET /panic", Panic)

	// First element is the outermost: the request ID exists before logging,
	// and Recover sits inside Logging so a panic is logged as a 500.
	return Chain(mux, RequestID, Logging(sink), Recover)
}
