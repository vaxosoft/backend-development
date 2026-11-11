// Versioning and idempotency: a /v1 prefix, a deprecated /v0, and an
// Idempotency-Key middleware that makes POST safe to retry.
// Runs in-process with httptest, so no port is needed.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

type store struct {
	mu    sync.Mutex
	next  int
	tasks map[int]string
}

func (s *store) v1() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Title string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		s.mu.Lock()
		s.next++
		id := s.next
		s.tasks[id] = in.Title
		s.mu.Unlock()
		w.Header().Set("Location", fmt.Sprintf("/v1/tasks/%d", id))
		jsonReply(w, http.StatusCreated, fmt.Sprintf(`{"id":%d,"title":%q}`, id, in.Title))
	})
	// PUT is idempotent by definition: repeating it leaves the same state.
	mux.HandleFunc("PUT /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		var in struct{ Title string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		s.mu.Lock()
		s.tasks[id] = in.Title
		s.mu.Unlock()
		jsonReply(w, http.StatusOK, fmt.Sprintf(`{"id":%d,"title":%q}`, id, in.Title))
	})
	// DELETE is idempotent in effect; the second status may differ (404).
	mux.HandleFunc("DELETE /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		s.mu.Lock()
		_, ok := s.tasks[id]
		delete(s.tasks, id)
		s.mu.Unlock()
		if !ok {
			jsonReply(w, http.StatusNotFound, `{"error":{"code":"not_found","message":"task not found"}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func jsonReply(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintln(w, body)
}

// capture records a response so it can be stored and replayed.
type capture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *capture) Header() http.Header         { return c.header }
func (c *capture) Write(b []byte) (int, error) { return c.body.Write(b) }
func (c *capture) WriteHeader(code int)        { c.status = code }

type saved struct {
	fingerprint [32]byte
	res         *capture
}

// Idempotent replays the stored response when a POST repeats an Idempotency-Key.
// A real implementation also needs expiry and per-user key scoping.
func Idempotent(next http.Handler) http.Handler {
	var mu sync.Mutex
	seen := map[string]saved{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if r.Method != http.MethodPost || key == "" {
			next.ServeHTTP(w, r)
			return
		}
		body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		r.Body = io.NopCloser(bytes.NewReader(body)) // the handler still needs the body
		fp := sha256.Sum256(body)

		mu.Lock()
		defer mu.Unlock() // serialises POSTs: simple, and a retry never races the original
		if prev, ok := seen[key]; ok {
			if prev.fingerprint != fp {
				jsonReply(w, http.StatusUnprocessableEntity,
					`{"error":{"code":"idempotency_key_reused","message":"key was used with a different request body"}}`)
				return
			}
			w.Header().Set("Idempotent-Replayed", "true") // before the body: headers freeze on first write
			write(w, prev.res)
			return
		}
		c := &capture{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(c, r)
		seen[key] = saved{fp, c}
		write(w, c)
	})
}

func write(w http.ResponseWriter, c *capture) {
	for k, v := range c.header {
		w.Header()[k] = v
	}
	w.WriteHeader(c.status)
	_, _ = w.Write(c.body.Bytes())
}

func newHandler() http.Handler {
	s := &store{tasks: map[int]string{}}
	root := http.NewServeMux()
	root.Handle("/v1/", http.StripPrefix("/v1", Idempotent(s.v1())))
	root.HandleFunc("/v0/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Link", `</v1/tasks>; rel="successor-version"`)
		jsonReply(w, http.StatusGone, `{"error":{"code":"gone","message":"v0 was removed, use /v1"}}`)
	})
	return root
}

func main() {
	h := newHandler()
	do := func(method, path, key, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		extra := ""
		for _, name := range []string{"Location", "Idempotent-Replayed", "Deprecation", "Link"} {
			if v := rec.Header().Get(name); v != "" {
				extra += " " + name + "=" + v
			}
		}
		fmt.Printf("%-6s %-14s key=%-4s -> %d %s%s\n", method, path, key, rec.Code, strings.TrimSpace(rec.Body.String()), extra)
	}

	do("POST", "/v1/tasks", "", `{"title":"A"}`)
	do("POST", "/v1/tasks", "", `{"title":"A"}`) // no key: a duplicate is created
	do("POST", "/v1/tasks", "k1", `{"title":"B"}`)
	do("POST", "/v1/tasks", "k1", `{"title":"B"}`) // retry: same response, nothing created
	do("POST", "/v1/tasks", "k1", `{"title":"C"}`) // same key, different body
	do("POST", "/v1/tasks", "k2", `{"title":"D"}`)
	do("PUT", "/v1/tasks/1", "", `{"title":"A2"}`)
	do("PUT", "/v1/tasks/1", "", `{"title":"A2"}`)
	do("DELETE", "/v1/tasks/1", "", "")
	do("DELETE", "/v1/tasks/1", "", "")
	do("GET", "/v0/tasks", "", "")
}
