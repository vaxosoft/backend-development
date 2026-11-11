// Package aggregator fans a single request out to an upstream API.
//
// GET /aggregate?ids=1,2,3 calls GET {UpstreamURL}/items/{id} for every id
// concurrently (bounded), retries transient failures and merges the answers
// in request order.
package aggregator

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// MaxIDs is the largest number of ids one request may ask for.
const MaxIDs = 50

// Config holds every tunable of the service. Tests shrink the durations.
type Config struct {
	UpstreamURL    string        // base URL, e.g. http://localhost:9000
	MaxConcurrency int           // upstream calls in flight at once
	CallTimeout    time.Duration // limit for ONE upstream attempt
	OverallTimeout time.Duration // limit for the whole /aggregate request
	MaxRetries     int           // retries after the first attempt
	BackoffBase    time.Duration // first retry waits BackoffBase, then doubles
}

// DefaultConfig returns production-like defaults for the given upstream.
func DefaultConfig(upstreamURL string) Config {
	return Config{
		UpstreamURL:    upstreamURL,
		MaxConcurrency: 4,
		CallTimeout:    2 * time.Second,
		OverallTimeout: 10 * time.Second,
		MaxRetries:     2,
		BackoffBase:    100 * time.Millisecond,
	}
}

// Result is one successful upstream answer; Data is the upstream JSON as-is.
type Result struct {
	ID   int             `json:"id"`
	Data json.RawMessage `json:"data"`
}

// Failure is one id that could not be fetched.
type Failure struct {
	ID    int    `json:"id"`
	Error string `json:"error"`
}

// Response is the body of GET /aggregate. Both slices keep request order.
type Response struct {
	Results []Result  `json:"results"`
	Errors  []Failure `json:"errors"`
}

// Service holds the shared state of the handlers.
type Service struct {
	cfg    Config
	client *http.Client // one client for all calls so connections are reused
}

// New builds the HTTP handler. Tests call it with httptest.NewServer.
func New(cfg Config) http.Handler {
	s := &Service{cfg: cfg, client: &http.Client{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /aggregate", s.handleAggregate)
	return mux
}

func (s *Service) handleAggregate(w http.ResponseWriter, r *http.Request) {
	ids, err := ParseIDs(r.URL.Query().Get("ids"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	// r.Context() is cancelled when the client disconnects; the timeout
	// adds an overall deadline on top.
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.OverallTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, s.Aggregate(ctx, ids))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
