package shutdown

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

type running struct {
	url    string
	cancel context.CancelFunc
	done   chan error
}

func start(t *testing.T, h http.Handler, grace time.Duration) *running {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := &running{url: "http://" + ln.Addr().String(), cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- Run(ctx, ln, h, grace) }()
	return r
}

func (r *running) waitDone(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return in time")
		return nil
	}
}

func client() *http.Client {
	return &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
}

func get(t *testing.T, url string) (int, string, error) {
	t.Helper()
	resp, err := client().Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

func TestNewServerSetsAllTimeouts(t *testing.T) {
	srv := NewServer(http.NotFoundHandler())
	checks := map[string]time.Duration{
		"ReadHeaderTimeout": srv.ReadHeaderTimeout,
		"ReadTimeout":       srv.ReadTimeout,
		"WriteTimeout":      srv.WriteTimeout,
		"IdleTimeout":       srv.IdleTimeout,
	}
	for name, d := range checks {
		if d <= 0 {
			t.Errorf("%s is not set", name)
		}
	}
	if srv.Handler == nil {
		t.Error("Handler is nil")
	}
}

func TestServesThenStopsCleanly(t *testing.T) {
	r := start(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "pong") }), time.Second)

	if code, body, err := get(t, r.url); err != nil || code != 200 || body != "pong" {
		t.Fatalf("GET = %d %q %v", code, body, err)
	}
	r.cancel()
	if err := r.waitDone(t); err != nil {
		t.Fatalf("Run = %v, want nil after a clean shutdown", err)
	}
	if _, _, err := get(t, r.url); err == nil {
		t.Fatal("server still accepts connections after Run returned")
	}
}

func TestInFlightRequestFinishesDuringShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		io.WriteString(w, "finished")
	})
	r := start(t, h, 3*time.Second)

	type result struct {
		code int
		body string
		err  error
	}
	res := make(chan result, 1)
	go func() {
		code, body, err := get(t, r.url)
		res <- result{code, body, err}
	}()
	<-started
	r.cancel()

	// New connections are refused soon after shutdown begins.
	deadline := time.Now().Add(2 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", r.url[len("http://"):], 200*time.Millisecond)
		if err != nil {
			break
		}
		c.Close()
		if time.Now().After(deadline) {
			t.Fatal("listener still accepting connections after cancel")
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case err := <-r.done:
		t.Fatalf("Run returned (%v) while a request was still in flight", err)
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	got := <-res
	if got.err != nil || got.code != 200 || got.body != "finished" {
		t.Fatalf("in-flight request = %+v, want it to complete normally", got)
	}
	if err := r.waitDone(t); err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
}

func TestGraceExpiryReturnsDeadlineExceeded(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	run := start(t, h, 50*time.Millisecond)

	clientErr := make(chan error, 1)
	go func() {
		_, _, err := get(t, run.url)
		clientErr <- err
	}()
	<-started
	run.cancel()

	err := run.waitDone(t)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run = %v, want an error wrapping context.DeadlineExceeded", err)
	}
	select {
	case err := <-clientErr:
		if err == nil {
			t.Error("the stuck request should have been cut off")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connections were not closed after the grace period")
	}
}

func TestServeFailureBeforeCancelIsReturned(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()

	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), ln, http.NotFoundHandler(), time.Second) }()
	select {
	case err := <-done:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run = %v, want the listener failure", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return although the listener is closed")
	}
}
