package limiter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock { return &clock{now: time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestBurstThenRefill(t *testing.T) {
	clk := newClock()
	l := New(3, 1, clk.Now)
	for i := range 3 {
		if ok, _ := l.Allow("ada"); !ok {
			t.Fatalf("request %d inside the burst was rejected", i+1)
		}
	}
	ok, wait := l.Allow("ada")
	if ok || wait != time.Second {
		t.Fatalf("4th request: ok=%t wait=%v, want rejected with 1s", ok, wait)
	}
	clk.Advance(time.Second)
	if ok, _ := l.Allow("ada"); !ok {
		t.Fatal("one token should have refilled after 1s")
	}
	if ok, _ := l.Allow("ada"); ok {
		t.Fatal("only one token should have refilled")
	}
}

func TestRejectionsDoNotConsumeTokens(t *testing.T) {
	clk := newClock()
	l := New(1, 1, clk.Now)
	l.Allow("ada")
	for range 10 {
		ok, wait := l.Allow("ada")
		if ok || wait != time.Second {
			t.Fatalf("ok=%t wait=%v: rejected calls must not push the wait time out", ok, wait)
		}
	}
	clk.Advance(time.Second)
	if ok, _ := l.Allow("ada"); !ok {
		t.Fatal("after 1s the next request must pass despite the rejected attempts")
	}
}

func TestPartialRefillReportsRemainingWait(t *testing.T) {
	clk := newClock()
	l := New(1, 2, clk.Now) // one token every 500ms
	l.Allow("ada")
	clk.Advance(200 * time.Millisecond)
	ok, wait := l.Allow("ada")
	if ok || wait != 300*time.Millisecond {
		t.Fatalf("ok=%t wait=%v, want rejected with 300ms", ok, wait)
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, 1, newClock().Now)
	if ok, _ := l.Allow("ada"); !ok {
		t.Fatal("ada's first request rejected")
	}
	if ok, _ := l.Allow("bob"); !ok {
		t.Fatal("bob must have his own bucket")
	}
	if ok, _ := l.Allow("ada"); ok {
		t.Fatal("ada's second request should be rejected")
	}
}

func TestBucketNeverExceedsBurst(t *testing.T) {
	clk := newClock()
	l := New(2, 1, clk.Now)
	l.Allow("ada")
	clk.Advance(time.Hour)
	allowed := 0
	for range 5 {
		if ok, _ := l.Allow("ada"); ok {
			allowed++
		}
	}
	if allowed != 2 {
		t.Fatalf("allowed %d requests after a long idle period, want the burst of 2", allowed)
	}
}

func TestConcurrentCallsShareOneBucket(t *testing.T) {
	l := New(10, 1, newClock().Now)
	var mu sync.Mutex
	allowed := 0
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if ok, _ := l.Allow("shared"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if allowed != 10 {
		t.Fatalf("allowed %d, want exactly the burst of 10", allowed)
	}
}

func TestMiddleware(t *testing.T) {
	clk := newClock()
	l := New(1, 0.5, clk.Now) // one token every 2s
	reached := 0
	h := Middleware(l, func(r *http.Request) string { return r.Header.Get("X-User") })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached++
			w.WriteHeader(http.StatusNoContent)
		}))

	call := func(user string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/tasks", nil)
		req.Header.Set("X-User", user)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := call("ada"); rec.Code != http.StatusNoContent {
		t.Fatalf("first call: %d", rec.Code)
	}
	clk.Advance(500 * time.Millisecond)
	rec := call("ada")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second call: %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "2" { // 1.5s rounded up
		t.Errorf("Retry-After = %q, want 2", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	var body struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "rate_limited" || body.Error.Message == "" {
		t.Errorf("body = %q (err %v), want the rate_limited envelope", rec.Body.String(), err)
	}
	if rec := call("bob"); rec.Code != http.StatusNoContent {
		t.Fatalf("other user: %d", rec.Code)
	}
	if reached != 2 {
		t.Fatalf("handler reached %d times, want 2 (rejected requests must not reach it)", reached)
	}
}
