package reqlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type harness struct {
	buf     bytes.Buffer
	handler http.Handler
	seen    string // request ID seen by the wrapped handler
	ids     int
}

func newHarness(inner http.HandlerFunc) *harness {
	h := &harness{}
	logger := slog.New(slog.NewJSONHandler(&h.buf, nil))
	newID := func() string { h.ids++; return fmt.Sprintf("gen-%d", h.ids) }
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h.seen = RequestID(r.Context())
		inner(w, r)
	})
	h.handler = Middleware(logger, newID)(mux)
	return h
}

func (h *harness) do(method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range hdr {
		req.Header[k] = []string{v} // bypass validation so tests can send hostile values
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) records(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(h.buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad log line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func ok(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "hello") }

func TestOneRecordWithAllFields(t *testing.T) {
	h := newHarness(ok)
	h.do("GET", "/tasks/1", map[string]string{"X-Request-Id": "abc-123"})
	recs := h.records(t)
	if len(recs) != 1 {
		t.Fatalf("records = %v", recs)
	}
	m := recs[0]
	want := map[string]any{"msg": "request", "level": "INFO", "method": "GET", "path": "/tasks/1", "status": float64(200), "bytes": float64(5), "request_id": "abc-123"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	if d, ok := m["duration_ms"].(float64); !ok || d < 0 {
		t.Errorf("duration_ms = %v", m["duration_ms"])
	}
}

func TestRequestIDHeaderContextAndGeneration(t *testing.T) {
	h := newHarness(ok)
	rec := h.do("GET", "/", map[string]string{"X-Request-Id": "client-id"})
	if rec.Header().Get("X-Request-ID") != "client-id" || h.seen != "client-id" {
		t.Errorf("header=%q ctx=%q", rec.Header().Get("X-Request-ID"), h.seen)
	}
	rec = h.do("GET", "/", nil)
	if rec.Header().Get("X-Request-ID") != "gen-1" || h.seen != "gen-1" {
		t.Errorf("header=%q ctx=%q", rec.Header().Get("X-Request-ID"), h.seen)
	}
	if got := h.records(t)[1]["request_id"]; got != "gen-1" {
		t.Errorf("logged id = %v", got)
	}
	if RequestID(httptest.NewRequest("GET", "/", nil).Context()) != "" {
		t.Error("RequestID without middleware must be empty")
	}
}

func TestUnacceptableIncomingIDsAreReplaced(t *testing.T) {
	bad := []string{"has space", "line\nbreak", strings.Repeat("a", 65), "tab\there", "naïve"}
	for _, id := range bad {
		h := newHarness(ok)
		rec := h.do("GET", "/", map[string]string{"X-Request-Id": id})
		if got := rec.Header().Get("X-Request-ID"); got != "gen-1" {
			t.Errorf("incoming %q: response ID = %q, want a generated one", id, got)
		}
		if strings.Contains(h.buf.String(), "break") && strings.Contains(id, "break") {
			t.Errorf("hostile ID reached the log: %s", h.buf.String())
		}
	}
	h := newHarness(ok)
	if rec := h.do("GET", "/", map[string]string{"X-Request-Id": strings.Repeat("a", 64)}); rec.Header().Get("X-Request-ID") != strings.Repeat("a", 64) {
		t.Error("a 64-byte ID is acceptable")
	}
}

func TestStatusLevelsAndImplicit200(t *testing.T) {
	tests := []struct {
		name   string
		inner  http.HandlerFunc
		status float64
		level  string
	}{
		{"implicit 200 without writing", func(w http.ResponseWriter, r *http.Request) {}, 200, "INFO"},
		{"201", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) }, 201, "INFO"},
		{"404", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, 404, "WARN"},
		{"422", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(422) }, 422, "WARN"},
		{"500", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }, 500, "ERROR"},
		{"first WriteHeader wins", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202); w.WriteHeader(500) }, 202, "INFO"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(tc.inner)
			h.do("GET", "/x", nil)
			m := h.records(t)[0]
			if m["status"] != tc.status || m["level"] != tc.level {
				t.Errorf("status=%v level=%v, want %v %v", m["status"], m["level"], tc.status, tc.level)
			}
		})
	}
}

func TestBytesCountsAllWrites(t *testing.T) {
	h := newHarness(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "12345")
		fmt.Fprint(w, "678")
	})
	h.do("GET", "/", nil)
	if got := h.records(t)[0]["bytes"]; got != float64(8) {
		t.Errorf("bytes = %v", got)
	}
}

func TestQueryStringIsNeverLogged(t *testing.T) {
	h := newHarness(ok)
	h.do("GET", "/tasks?token=s3cret&limit=5", nil)
	if strings.Contains(h.buf.String(), "s3cret") || h.records(t)[0]["path"] != "/tasks" {
		t.Errorf("log = %s", h.buf.String())
	}
}

func TestWrapperDoesNotHideFlush(t *testing.T) {
	var flushErr error
	h := newHarness(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "chunk")
		flushErr = http.NewResponseController(w).Flush()
	})
	rec := h.do("GET", "/", nil)
	if flushErr != nil || !rec.Flushed {
		t.Fatalf("Flush through the middleware: err=%v flushed=%v (implement Unwrap on your ResponseWriter wrapper)", flushErr, rec.Flushed)
	}
}
