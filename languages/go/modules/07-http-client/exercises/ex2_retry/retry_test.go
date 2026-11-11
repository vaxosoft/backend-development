package retry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

type step struct {
	status int
	err    error
}

type body struct {
	io.Reader
	closed bool
}

func (b *body) Close() error { b.closed = true; return nil }

// scriptedDoer replays steps in order and records what it saw.
type scriptedDoer struct {
	steps  []step
	calls  int
	bodies []string
	reqs   []*http.Request
	resps  []*body
}

func (d *scriptedDoer) Do(r *http.Request) (*http.Response, error) {
	s := d.steps[min(d.calls, len(d.steps)-1)]
	d.calls++
	d.reqs = append(d.reqs, r)
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		d.bodies = append(d.bodies, string(b))
	}
	if s.err != nil {
		return nil, s.err
	}
	b := &body{Reader: strings.NewReader("payload")}
	d.resps = append(d.resps, b)
	return &http.Response{StatusCode: s.status, Body: b}, nil
}

type sleeper struct{ got []time.Duration }

func (s *sleeper) Sleep(_ context.Context, d time.Duration) error {
	s.got = append(s.got, d)
	return nil
}

func policy(s *sleeper, retries int) Policy {
	return Policy{MaxRetries: retries, Base: 10 * time.Millisecond, Sleep: s.Sleep}
}

func TestRetriesServerErrorsThenSucceeds(t *testing.T) {
	d := &scriptedDoer{steps: []step{{status: 503}, {status: 500}, {status: 200}}}
	s := &sleeper{}
	resp, err := Do(context.Background(), d, policy(s, 3), http.MethodGet, "http://x.test/", nil)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	if d.calls != 3 {
		t.Errorf("calls = %d, want 3", d.calls)
	}
	if want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}; !slices.Equal(s.got, want) {
		t.Errorf("sleeps = %v, want %v", s.got, want)
	}
	if !d.resps[0].closed || !d.resps[1].closed {
		t.Error("retried responses must be closed")
	}
	if d.resps[2].closed {
		t.Error("the returned response belongs to the caller; do not close it")
	}
}

func TestRetriesTransportErrors(t *testing.T) {
	boom := errors.New("connection reset")
	d := &scriptedDoer{steps: []step{{err: boom}, {err: boom}, {status: 200}}}
	s := &sleeper{}
	resp, err := Do(context.Background(), d, policy(s, 2), http.MethodDelete, "http://x.test/", nil)
	if err != nil || resp.StatusCode != 200 || d.calls != 3 {
		t.Fatalf("resp=%v err=%v calls=%d", resp, err, d.calls)
	}
}

func TestExhaustedRetriesReturnLastResult(t *testing.T) {
	d := &scriptedDoer{steps: []step{{status: 503}}}
	s := &sleeper{}
	resp, err := Do(context.Background(), d, policy(s, 2), http.MethodGet, "http://x.test/", nil)
	if err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("resp=%v err=%v, want the final 503 response and nil error", resp, err)
	}
	if d.calls != 3 {
		t.Errorf("calls = %d, want 3", d.calls)
	}
	if len(s.got) != 2 {
		t.Errorf("sleeps = %v, want exactly 2 (none after the last attempt)", s.got)
	}

	boom := errors.New("down")
	d = &scriptedDoer{steps: []step{{err: boom}}}
	_, err = Do(context.Background(), d, policy(&sleeper{}, 1), http.MethodGet, "http://x.test/", nil)
	if !errors.Is(err, boom) || d.calls != 2 {
		t.Errorf("err=%v calls=%d, want boom after 2 calls", err, d.calls)
	}
}

func TestNeverRetriesClientErrorsOrRedirects(t *testing.T) {
	for _, code := range []int{200, 302, 400, 404, 429} {
		d := &scriptedDoer{steps: []step{{status: code}}}
		resp, err := Do(context.Background(), d, policy(&sleeper{}, 3), http.MethodGet, "http://x.test/", nil)
		if err != nil || resp.StatusCode != code || d.calls != 1 {
			t.Errorf("status %d: calls=%d err=%v", code, d.calls, err)
		}
	}
}

func TestNonIdempotentMethodsAreNotRetried(t *testing.T) {
	for _, m := range []string{http.MethodPost, http.MethodPatch} {
		d := &scriptedDoer{steps: []step{{status: 503}, {status: 200}}}
		resp, err := Do(context.Background(), d, policy(&sleeper{}, 3), m, "http://x.test/", []byte("{}"))
		if err != nil || resp.StatusCode != 503 || d.calls != 1 {
			t.Errorf("%s: calls=%d status=%v err=%v", m, d.calls, resp, err)
		}
	}
	boom := errors.New("reset")
	d := &scriptedDoer{steps: []step{{err: boom}, {status: 200}}}
	_, err := Do(context.Background(), d, policy(&sleeper{}, 3), http.MethodPost, "http://x.test/", nil)
	if !errors.Is(err, boom) || d.calls != 1 {
		t.Errorf("POST transport error: err=%v calls=%d", err, d.calls)
	}
}

func TestBodyIsResentOnEveryAttempt(t *testing.T) {
	d := &scriptedDoer{steps: []step{{status: 502}, {status: 502}, {status: 204}}}
	_, err := Do(context.Background(), d, policy(&sleeper{}, 3), http.MethodPut, "http://x.test/tasks/1", []byte(`{"title":"a"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.bodies) != 3 {
		t.Fatalf("bodies = %q", d.bodies)
	}
	for i, b := range d.bodies {
		if b != `{"title":"a"}` {
			t.Errorf("attempt %d sent body %q", i, b)
		}
	}
	if d.reqs[0].Method != http.MethodPut || d.reqs[0].URL.Path != "/tasks/1" {
		t.Errorf("request = %s %s", d.reqs[0].Method, d.reqs[0].URL)
	}
}

func TestContextEndsTheWait(t *testing.T) {
	d := &scriptedDoer{steps: []step{{status: 503}}}
	ctx, cancel := context.WithCancel(context.Background())
	p := Policy{MaxRetries: 5, Base: time.Hour, Sleep: func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}}
	_, err := Do(ctx, d, p, http.MethodGet, "http://x.test/", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if d.calls != 1 {
		t.Errorf("calls = %d, want 1", d.calls)
	}
}

func TestDefaultSleepIsContextAware(t *testing.T) {
	d := &scriptedDoer{steps: []step{{status: 503}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Do(ctx, d, Policy{MaxRetries: 3, Base: time.Hour}, http.MethodGet, "http://x.test/", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("waited far too long: the default sleep must stop when ctx is done")
	}
}
