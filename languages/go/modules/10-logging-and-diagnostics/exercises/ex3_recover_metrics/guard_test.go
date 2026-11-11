package guard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func serve(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
	return rec
}

func jsonLogger(buf *bytes.Buffer) *slog.Logger { return slog.New(slog.NewJSONHandler(buf, nil)) }

func TestRecoverTurnsPanicIntoEnvelope(t *testing.T) {
	var buf bytes.Buffer
	h := Recover(jsonLogger(&buf), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("secret internal detail")
	}))
	rec := serve(h, "/boom")

	if rec.Code != 500 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("code=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if want := `{"error":{"code":"internal","message":"internal server error"}}` + "\n"; rec.Body.String() != want {
		t.Errorf("body = %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Error("panic value leaked to the client")
	}

	var rec1 map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec1); err != nil {
		t.Fatalf("log is not one JSON record: %q", buf.String())
	}
	if rec1["level"] != "ERROR" || rec1["msg"] != "panic recovered" || rec1["panic"] != "secret internal detail" {
		t.Errorf("record = %v", rec1)
	}
	if stack, _ := rec1["stack"].(string); !strings.Contains(stack, "goroutine") {
		t.Errorf("stack = %q", stack)
	}
}

func TestRecoverRecoversRuntimePanicsAndErrorValues(t *testing.T) {
	var nilMap map[string]int
	for name, f := range map[string]func(){
		"nil map":    func() { nilMap["x"] = 1 },
		"error":      func() { panic(errors.New("bang")) },
		"integer":    func() { panic(42) },
		"index":      func() { empty := []int{}; i := 3; _ = empty[i] },
		"fmt.Errorf": func() { panic(fmt.Errorf("wrapped: %w", errors.New("x"))) },
	} {
		h := Recover(jsonLogger(&bytes.Buffer{}), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f() }))
		if rec := serve(h, "/"); rec.Code != 500 {
			t.Errorf("%s: code = %d", name, rec.Code)
		}
	}
}

func TestRecoverPassesThroughNormalResponses(t *testing.T) {
	h := Recover(jsonLogger(&bytes.Buffer{}), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		fmt.Fprint(w, "created")
	}))
	if rec := serve(h, "/"); rec.Code != 201 || rec.Body.String() != "created" {
		t.Errorf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestRecoverAfterResponseStarted(t *testing.T) {
	h := Recover(jsonLogger(&bytes.Buffer{}), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "partial")
		panic("late")
	}))
	rec := serve(h, "/")
	if rec.Code != 200 || rec.Body.String() != "partial" {
		t.Errorf("code=%d body=%q: an error body must not be appended to a started response", rec.Code, rec.Body.String())
	}
}

func TestAbortHandlerIsRepanickedAndNotLogged(t *testing.T) {
	var buf bytes.Buffer
	m := &Metrics{}
	h := Recover(jsonLogger(&buf), m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", v)
		}
		if buf.Len() != 0 {
			t.Errorf("an aborted handler must not be logged: %s", buf.String())
		}
		if m.panics.Load() != 0 {
			t.Error("an aborted handler must not be counted")
		}
	}()
	serve(h, "/")
}

func TestMetricsTextFormat(t *testing.T) {
	m := &Metrics{}
	h := m.Middleware(Recover(jsonLogger(&bytes.Buffer{}), m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/boom" {
			panic("x")
		}
	})))
	serve(h, "/a")
	serve(h, "/boom")
	serve(h, "/b")

	rec := serve(m.Handler(), "/metrics")
	if want := "requests_total 3\npanics_total 1\nin_flight 0\n"; rec.Body.String() != want {
		t.Errorf("metrics = %q, want %q", rec.Body.String(), want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("content type = %q", ct)
	}
}

func TestInFlightIsVisibleInsideAndResetAfterPanic(t *testing.T) {
	m := &Metrics{}
	var inside string
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inside = serve(m.Handler(), "/metrics").Body.String()
		panic("x")
	}))
	func() {
		defer func() { _ = recover() }() // the panic must propagate out of Middleware
		serve(h, "/")
	}()
	if !strings.Contains(inside, "in_flight 1\n") {
		t.Errorf("inside the handler: %q", inside)
	}
	if after := serve(m.Handler(), "/").Body.String(); !strings.Contains(after, "in_flight 0\n") {
		t.Errorf("after a panic: %q (use defer)", after)
	}
}

func TestCountersAreSafeUnderConcurrency(t *testing.T) {
	m := &Metrics{}
	h := m.Middleware(Recover(jsonLogger(&bytes.Buffer{}), m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("p") == "1" {
			panic("x")
		}
	})))
	var wg sync.WaitGroup
	for i := range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			target := "/"
			if i%4 == 0 {
				target = "/?p=1"
			}
			serve(h, target)
		}()
	}
	wg.Wait()
	if want := "requests_total 200\npanics_total 50\nin_flight 0\n"; serve(m.Handler(), "/").Body.String() != want {
		t.Errorf("metrics = %q", serve(m.Handler(), "/").Body.String())
	}
}
