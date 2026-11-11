// Example 04: health, readiness and basic metrics (expvar + a plain-text /metrics endpoint).
package main

import (
	"expvar"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
)

type metrics struct {
	vars     expvar.Map // a private map; expvar.NewMap would publish into the global /debug/vars
	inFlight atomic.Int64
}

func newMetrics() *metrics {
	m := &metrics{}
	m.vars.Init()
	return m
}

// Instrument counts every request and tracks how many are in flight.
func (m *metrics) Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Add(1)
		defer m.inFlight.Add(-1)
		m.vars.Add("requests_total", 1)
		next.ServeHTTP(w, r)
	})
}

func (m *metrics) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "requests_total %s\n", m.vars.Get("requests_total"))
	fmt.Fprintf(w, "in_flight %d\n", m.inFlight.Load())
}

func main() {
	var ready atomic.Bool // false until startup work is done, false again while draining
	m := newMetrics()

	mux := http.NewServeMux()
	// Liveness: "the process is running". Never check dependencies here, or a DB outage restarts every instance.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	// Readiness: "send me traffic".
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ready")
	})
	mux.HandleFunc("GET /metrics", m.Handler)
	mux.HandleFunc("GET /work", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "done") })

	srv := httptest.NewServer(m.Instrument(mux))
	defer srv.Close()

	get := func(path string) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("GET %-9s -> %d %s", path, resp.StatusCode, body)
	}

	get("/healthz")
	get("/readyz") // still starting
	ready.Store(true)
	get("/readyz")
	get("/work")
	get("/work")
	get("/metrics")

	ready.Store(false) // what a shutdown hook does before srv.Shutdown: stop receiving new traffic first
	get("/readyz")

	fmt.Println("expvar JSON:", m.vars.String())
}
