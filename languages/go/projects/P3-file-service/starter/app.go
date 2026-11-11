// Package starter is the P3 file service: streamed uploads and downloads.
package starter

import (
	"fmt"
	"net/http"
)

// DefaultMaxSize is the upload limit used when Config.MaxSize is zero (5 MiB).
const DefaultMaxSize = 5 << 20

// Config configures the service.
type Config struct {
	Dir     string // storage directory, created if missing
	MaxSize int64  // largest accepted file in bytes; 0 = DefaultMaxSize
}

// New returns the routed service. It is complete: your work is in storage.go,
// sanitize.go and handlers.go.
func New(cfg Config) (http.Handler, error) {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = DefaultMaxSize
	}
	store, err := NewStorage(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}

	api := &API{store: store, maxSize: cfg.MaxSize}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", api.upload)
	mux.HandleFunc("GET /files/{id}", api.download)
	mux.HandleFunc("DELETE /files/{id}", api.remove)
	return mux, nil
}
