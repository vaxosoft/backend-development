package starter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// MaxSlow is the longest delay GET /slow will honour.
const MaxSlow = 2 * time.Second

// WriteError writes the shared JSON error envelope:
// {"error":{"code":"...","message":"..."}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// Healthz answers GET /healthz with 200 {"status":"ok"}.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}

// Hello answers GET /hello and GET /hello/{name} with text/plain
// "Hello, {name}!" (name defaults to "world").
func Hello(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		name = "world"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Hello, %s!", name)
}

// SlowDelay parses the raw `ms` query value into a delay.
// An empty value means no delay; values above MaxSlow are clamped to MaxSlow;
// anything that is not a non-negative integer is an error.
func SlowDelay(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 0 {
		return 0, errors.New("ms must be a non-negative integer")
	}
	return min(time.Duration(ms)*time.Millisecond, MaxSlow), nil
}

// Slow sleeps for SlowDelay(ms) and then answers 200 "done". It must stop
// early (without writing) if the client goes away.
func Slow(w http.ResponseWriter, r *http.Request) {
	d, err := SlowDelay(r.URL.Query().Get("ms"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "done")
	case <-r.Context().Done():
	}
}

// Panic always panics; it exists to prove the Recover middleware works.
func Panic(_ http.ResponseWriter, _ *http.Request) {
	panic("boom: this is expected")
}
