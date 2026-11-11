package starter

import (
	"encoding/json"
	"net/http"
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
	// TODO: write the JSON body {"status":"ok"} with Content-Type application/json.
	WriteError(w, http.StatusNotImplemented, "todo", "healthz not implemented")
}

// Hello answers GET /hello and GET /hello/{name} with text/plain
// "Hello, {name}!" (name defaults to "world").
func Hello(w http.ResponseWriter, _ *http.Request) {
	// TODO: read r.PathValue("name"), default to "world", write text/plain.
	WriteError(w, http.StatusNotImplemented, "todo", "hello not implemented")
}

// SlowDelay parses the raw `ms` query value into a delay.
// An empty value means no delay; values above MaxSlow are clamped to MaxSlow;
// anything that is not a non-negative integer is an error.
func SlowDelay(_ string) (time.Duration, error) {
	// TODO: parse, validate and clamp.
	return 0, nil
}

// Slow sleeps for SlowDelay(ms) and then answers 200 "done". It must stop
// early (without writing) if the client goes away.
func Slow(w http.ResponseWriter, _ *http.Request) {
	// TODO: 400 bad_request envelope for an invalid ms, otherwise sleep with a
	// timer that also watches r.Context().Done().
	WriteError(w, http.StatusNotImplemented, "todo", "slow not implemented")
}

// Panic always panics; it exists to prove the Recover middleware works.
func Panic(_ http.ResponseWriter, _ *http.Request) {
	// TODO: panic with any value.
}
