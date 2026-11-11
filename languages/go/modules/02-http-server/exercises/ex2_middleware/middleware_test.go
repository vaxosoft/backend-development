package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func serve(h http.Handler, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/x", nil)
	for _, m := range mutate {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestChainFirstListedIsOutermost(t *testing.T) {
	var events []string
	tag := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				events = append(events, name+" in")
				next.ServeHTTP(w, r)
				events = append(events, name+" out")
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		events = append(events, "handler")
	}), tag("A"), tag("B"), tag("C"))
	serve(h)

	want := []string{"A in", "B in", "C in", "handler", "C out", "B out", "A out"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %q\nwant     %q", events, want)
	}
}

func TestChainWithoutMiddlewareReturnsHandlerBehaviour(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") }))
	if got := serve(h).Body.String(); got != "ok" {
		t.Fatalf("body = %q", got)
	}
}

func TestRequestID(t *testing.T) {
	var seen string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = IDFrom(r.Context()) })
	h := RequestID(func() string { return "gen-1" })(next)

	withHeader := func(v string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("X-Request-ID", v) }
	}
	tests := []struct {
		name   string
		mutate []func(*http.Request)
		want   string
	}{
		{"generated when absent", nil, "gen-1"},
		{"client id is echoed", []func(*http.Request){withHeader("client-7")}, "client-7"},
		{"64 bytes accepted", []func(*http.Request){withHeader(strings.Repeat("a", 64))}, strings.Repeat("a", 64)},
		{"65 bytes replaced", []func(*http.Request){withHeader(strings.Repeat("a", 65))}, "gen-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen = ""
			rec := serve(h, tt.mutate...)
			if got := rec.Header().Get("X-Request-ID"); got != tt.want {
				t.Errorf("response header = %q, want %q", got, tt.want)
			}
			if seen != tt.want {
				t.Errorf("context id = %q, want %q", seen, tt.want)
			}
		})
	}
}

func TestIDFromWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if got := IDFrom(req.Context()); got != "" {
		t.Fatalf("IDFrom = %q, want empty", got)
	}
}

func TestRecover(t *testing.T) {
	var logs []string
	logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret boom detail") })
	h := Chain(boom, RequestID(func() string { return "req-9" }), Recover(logf))

	rec := serve(h)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if body.Error.Code != "internal" || body.Error.Message != "internal server error" {
		t.Errorf("envelope = %+v", body.Error)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Error("the panic value leaked to the client")
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "secret boom detail") || !strings.Contains(logs[0], "req-9") {
		t.Errorf("logs = %q, want one line with the panic value and request id", logs)
	}

	if rec := serve(h); rec.Code != http.StatusInternalServerError {
		t.Errorf("second request: status = %d", rec.Code)
	}
}

func TestRecoverLeavesNormalResponsesAlone(t *testing.T) {
	h := Recover(t.Logf)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		fmt.Fprint(w, "short and stout")
	}))
	rec := serve(h)
	if rec.Code != http.StatusTeapot || rec.Body.String() != "short and stout" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRecoverRepanicsErrAbortHandler(t *testing.T) {
	h := Recover(t.Logf)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler to propagate", v)
		}
	}()
	serve(h)
	t.Fatal("expected a panic to propagate")
}

func TestAccessLog(t *testing.T) {
	type entry struct {
		method, path string
		status       int
	}
	var got []entry
	sink := func(m, p string, s int) { got = append(got, entry{m, p, s}) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "body only") })
	mux.HandleFunc("GET /missing", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	mux.HandleFunc("GET /empty", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) { panic("x") })
	h := Chain(mux, AccessLog(sink), Recover(t.Logf))

	for _, path := range []string{"/x", "/missing", "/empty", "/panic"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	}
	want := []entry{{"GET", "/x", 200}, {"GET", "/missing", 404}, {"GET", "/empty", 200}, {"GET", "/panic", 500}}
	if !slices.Equal(got, want) {
		t.Fatalf("log = %v\nwant  %v", got, want)
	}
}
