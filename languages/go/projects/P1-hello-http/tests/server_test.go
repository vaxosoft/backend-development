package tests

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"webdev/projects/P1-hello-http/starter"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.TrimSpace(b.buf.String())
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// waitForLines polls because the log line is written after the response has
// been sent, so the client can see the response first.
func (b *syncBuffer) waitForLines(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		lines := b.Lines()
		if len(lines) >= n || time.Now().After(deadline) {
			return lines
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func newServer(t *testing.T) (*httptest.Server, *syncBuffer) {
	t.Helper()
	sink := &syncBuffer{}
	srv := httptest.NewServer(starter.New(starter.Config{LogSink: sink}))
	t.Cleanup(srv.Close)
	return srv, sink
}

func get(t *testing.T, url string, headers ...string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(body)
}

func TestHealthzReturnsStatusOKJSON(t *testing.T) {
	srv, _ := newServer(t)
	resp, body := get(t, srv.URL+"/healthz")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if strings.TrimSpace(body) != `{"status":"ok"}` {
		t.Errorf("body = %q", body)
	}
}

func TestHelloGreetsName(t *testing.T) {
	srv, _ := newServer(t)
	resp, body := get(t, srv.URL+"/hello/Gopher")
	if resp.StatusCode != 200 || body != "Hello, Gopher!" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHelloDecodesPercentEncodedName(t *testing.T) {
	srv, _ := newServer(t)
	_, body := get(t, srv.URL+"/hello/Ada%20Lovelace")
	if body != "Hello, Ada Lovelace!" {
		t.Errorf("body = %q", body)
	}
}

func TestHelloWithoutNameGreetsWorld(t *testing.T) {
	srv, _ := newServer(t)
	resp, body := get(t, srv.URL+"/hello")
	if resp.StatusCode != 200 || body != "Hello, world!" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
}

func TestWrongMethodIsRejectedWithAllowHeader(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Post(srv.URL+"/healthz", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Allow"), "GET") {
		t.Errorf("Allow = %q, want it to list GET", resp.Header.Get("Allow"))
	}
}

func TestSlowWaitsThenAnswersDone(t *testing.T) {
	srv, _ := newServer(t)
	start := time.Now()
	resp, body := get(t, srv.URL+"/slow?ms=40")
	if resp.StatusCode != 200 || body != "done" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("answered after %v, want at least 40ms", elapsed)
	}
}

func TestSlowDelayParsesAndClamps(t *testing.T) {
	tests := []struct {
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"150", 150 * time.Millisecond, false},
		{"2000", 2 * time.Second, false},
		{"2001", 2 * time.Second, false},
		{"999999", 2 * time.Second, false},
		{"-1", 0, true},
		{"abc", 0, true},
		{"1.5", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := starter.SlowDelay(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SlowDelay(%q) error = %v, wantErr %v", tc.raw, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("SlowDelay(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSlowRejectsInvalidMsWithErrorEnvelope(t *testing.T) {
	srv, _ := newServer(t)
	resp, body := get(t, srv.URL+"/slow?ms=abc")
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(body, `"code":"bad_request"`) {
		t.Errorf("body = %q, want a bad_request envelope", body)
	}
}

func TestPanicIsAnswered500WithEnvelopeAndNoLeak(t *testing.T) {
	srv, _ := newServer(t)
	resp, body := get(t, srv.URL+"/panic")
	if resp.StatusCode != 500 {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(body, `"code":"internal"`) {
		t.Errorf("body = %q, want an internal envelope", body)
	}
	if strings.Contains(body, "boom") {
		t.Errorf("body leaks the panic value: %q", body)
	}
}

func TestServerKeepsServingAfterPanic(t *testing.T) {
	srv, _ := newServer(t)
	get(t, srv.URL+"/panic")
	resp, body := get(t, srv.URL+"/hello/again")
	if resp.StatusCode != 200 || body != "Hello, again!" {
		t.Fatalf("got %d %q after a panic", resp.StatusCode, body)
	}
}

func TestRequestIDIsGeneratedWhenAbsent(t *testing.T) {
	srv, _ := newServer(t)
	resp, _ := get(t, srv.URL+"/healthz")
	if resp.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID response header")
	}
}

func TestRequestIDIsUniquePerRequest(t *testing.T) {
	srv, _ := newServer(t)
	r1, _ := get(t, srv.URL+"/healthz")
	r2, _ := get(t, srv.URL+"/healthz")
	if a, b := r1.Header.Get("X-Request-ID"), r2.Header.Get("X-Request-ID"); a == "" || a == b {
		t.Errorf("ids %q and %q should be non-empty and different", a, b)
	}
}

func TestRequestIDIsEchoed(t *testing.T) {
	srv, _ := newServer(t)
	resp, _ := get(t, srv.URL+"/healthz", "X-Request-ID", "abc-123")
	if got := resp.Header.Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("X-Request-ID = %q, want abc-123", got)
	}
}

func TestOverlongRequestIDIsReplaced(t *testing.T) {
	srv, _ := newServer(t)
	long := strings.Repeat("x", 65)
	resp, _ := get(t, srv.URL+"/healthz", "X-Request-ID", long)
	if got := resp.Header.Get("X-Request-ID"); got == "" || got == long {
		t.Errorf("X-Request-ID = %q, want a freshly generated id", got)
	}
}

func TestRequestIDIsPresentOnPanicResponses(t *testing.T) {
	srv, _ := newServer(t)
	resp, _ := get(t, srv.URL+"/panic", "X-Request-ID", "p-1")
	if got := resp.Header.Get("X-Request-ID"); got != "p-1" {
		t.Errorf("X-Request-ID = %q, want p-1", got)
	}
}

func TestLoggingWritesOneLinePerRequestWithFields(t *testing.T) {
	srv, sink := newServer(t)
	get(t, srv.URL+"/hello/log", "X-Request-ID", "log-1")
	lines := sink.waitForLines(t, 1)
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), lines)
	}
	for _, want := range []string{"method=GET", "path=/hello/log", "status=200", "duration=", "log-1"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("log line %q does not contain %q", lines[0], want)
		}
	}
}

func TestLoggingWritesALinePerRequest(t *testing.T) {
	srv, sink := newServer(t)
	for _, p := range []string{"/healthz", "/hello", "/hello/x"} {
		get(t, srv.URL+p)
	}
	if lines := sink.waitForLines(t, 3); len(lines) != 3 {
		t.Fatalf("got %d log lines, want 3: %q", len(lines), lines)
	}
}

func TestLoggingRecordsStatus500ForPanics(t *testing.T) {
	srv, sink := newServer(t)
	get(t, srv.URL+"/panic")
	lines := sink.waitForLines(t, 1)
	if len(lines) != 1 || !strings.Contains(lines[0], "status=500") || !strings.Contains(lines[0], "path=/panic") {
		t.Fatalf("log = %q, want one line with path=/panic status=500", lines)
	}
}

func TestServerTimeoutsAreConfigured(t *testing.T) {
	srv := starter.NewServer(http.NotFoundHandler())
	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Errorf("timeouts must all be set: header=%v read=%v write=%v idle=%v",
			srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
	if srv.WriteTimeout <= starter.MaxSlow {
		t.Errorf("WriteTimeout %v must exceed MaxSlow %v", srv.WriteTimeout, starter.MaxSlow)
	}
}

// runServer starts starter.Run on a loopback listener and returns its address,
// a function that triggers shutdown, and the channel carrying Run's result.
func runServer(t *testing.T, h http.Handler, drain time.Duration) (addr string, shutdown func(), done <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() { result <- starter.Run(ctx, ln, h, drain) }()
	return ln.Addr().String(), cancel, result
}

// blockingHandler signals when a request arrives and answers once released.
func blockingHandler(t *testing.T) (h http.Handler, started <-chan struct{}, release func()) {
	t.Helper()
	s, r := make(chan struct{}), make(chan struct{})
	var once sync.Once
	rel := func() { once.Do(func() { close(r) }) }
	t.Cleanup(rel)
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(s)
		<-r
		_, _ = io.WriteString(w, "finished")
	}), s, rel
}

func waitStarted(t *testing.T, started <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("Run returned early (%v) instead of serving", err)
	case <-time.After(3 * time.Second):
		t.Fatal("the request never reached the handler")
	}
}

func waitRefused(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err != nil {
			return
		}
		_ = c.Close()
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("new connections are still accepted after shutdown started")
}

func TestShutdownLetsInFlightRequestFinish(t *testing.T) {
	h, started, release := blockingHandler(t)
	addr, shutdown, done := runServer(t, h, 5*time.Second)

	type result struct {
		body string
		err  error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		got <- result{string(b), err}
	}()

	waitStarted(t, started, done)
	shutdown()
	waitRefused(t, addr)

	select {
	case err := <-done:
		t.Fatalf("Run returned (%v) while a request was still in flight", err)
	default:
	}

	release()
	r := <-got
	if r.err != nil || r.body != "finished" {
		t.Fatalf("in-flight request: body=%q err=%v, want finished", r.body, r.err)
	}
	if err := <-done; err != nil {
		t.Errorf("Run = %v, want nil after a clean drain", err)
	}
}

func TestShutdownRefusesNewConnections(t *testing.T) {
	addr, shutdown, done := runServer(t, http.NotFoundHandler(), time.Second)
	shutdown()
	if err := <-done; err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = c.Close()
		t.Fatal("connection accepted after Run returned")
	}
}

func TestShutdownGivesUpAfterDrainTimeout(t *testing.T) {
	h, started, _ := blockingHandler(t)
	addr, shutdown, done := runServer(t, h, 50*time.Millisecond)

	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Get("http://" + addr + "/"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	waitStarted(t, started, done)
	shutdown()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run = %v, want an error wrapping context.DeadlineExceeded", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after the drain timeout")
	}
}
