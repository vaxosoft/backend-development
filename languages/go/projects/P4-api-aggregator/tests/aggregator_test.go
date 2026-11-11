package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	aggregator "webdev/projects/P4-api-aggregator/starter"
)

// fakeUpstream is a controllable stand-in for the real upstream API.
type fakeUpstream struct {
	srv *httptest.Server

	// behave decides the answer for one call; attempt counts calls per id from 1.
	behave func(w http.ResponseWriter, r *http.Request, id, attempt int)

	mu       sync.Mutex
	attempts map[int]int
	times    map[int][]time.Time

	inFlight atomic.Int64
	peak     atomic.Int64
	total    atomic.Int64
}

func newUpstream(t *testing.T, behave func(w http.ResponseWriter, r *http.Request, id, attempt int)) *fakeUpstream {
	t.Helper()
	u := &fakeUpstream{behave: behave, attempts: map[int]int{}, times: map[int][]time.Time{}}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr, ok := strings.CutPrefix(r.URL.Path, "/items/")
		id, err := strconv.Atoi(idStr)
		if !ok || err != nil || r.Method != http.MethodGet {
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusTeapot)
			return
		}
		u.total.Add(1)
		now := u.inFlight.Add(1)
		defer u.inFlight.Add(-1)
		for {
			p := u.peak.Load()
			if now <= p || u.peak.CompareAndSwap(p, now) {
				break
			}
		}
		u.mu.Lock()
		u.attempts[id]++
		attempt := u.attempts[id]
		u.times[id] = append(u.times[id], time.Now())
		u.mu.Unlock()
		u.behave(w, r, id, attempt)
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func (u *fakeUpstream) callsFor(id int) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.attempts[id]
}

func (u *fakeUpstream) timesFor(id int) []time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]time.Time(nil), u.times[id]...)
}

func okItem(w http.ResponseWriter, id int) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"item":%d,"name":"item-%d"}`, id, id)
}

func serveOK(w http.ResponseWriter, _ *http.Request, id, _ int) { okItem(w, id) }

// start returns the base URL of an aggregator wired to the upstream.
func start(t *testing.T, up *fakeUpstream, tune func(*aggregator.Config)) string {
	t.Helper()
	cfg := aggregator.DefaultConfig(up.srv.URL)
	cfg.BackoffBase = time.Millisecond
	cfg.CallTimeout = 2 * time.Second
	cfg.OverallTimeout = 5 * time.Second
	if tune != nil {
		tune(&cfg)
	}
	srv := httptest.NewServer(aggregator.New(cfg))
	t.Cleanup(srv.Close)
	return srv.URL
}

type response struct {
	Results []struct {
		ID   int             `json:"id"`
		Data json.RawMessage `json:"data"`
	} `json:"results"`
	Errors []struct {
		ID    int    `json:"id"`
		Error string `json:"error"`
	} `json:"errors"`
}

func aggregate(t *testing.T, base, ids string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(base + "/aggregate?ids=" + ids)
	if err != nil {
		t.Fatalf("GET /aggregate: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, body
}

func decode(t *testing.T, body []byte) response {
	t.Helper()
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("response is not valid JSON: %v\n%s", err, body)
	}
	return r
}

func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

// --- request validation -----------------------------------------------------

func TestMissingIDsIs400WithErrorEnvelope(t *testing.T) {
	base := start(t, newUpstream(t, serveOK), nil)
	resp, body := aggregate(t, base, "")
	wantStatus(t, resp, http.StatusBadRequest)
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var env struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Code != "bad_request" || env.Error.Message == "" {
		t.Errorf("body = %s, want error envelope with code bad_request", body)
	}
}

func TestIDsParameterAbsentIs400(t *testing.T) {
	base := start(t, newUpstream(t, serveOK), nil)
	resp, err := http.Get(base + "/aggregate")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	wantStatus(t, resp, http.StatusBadRequest)
}

func TestInvalidIDsAre400AndNeverReachUpstream(t *testing.T) {
	up := newUpstream(t, serveOK)
	base := start(t, up, nil)
	for _, ids := range []string{"abc", "1,x,3", "1,,2", "1,2,", "-5", "1.5", ",", "9999999999999999999999"} {
		t.Run(ids, func(t *testing.T) {
			resp, _ := aggregate(t, base, ids)
			wantStatus(t, resp, http.StatusBadRequest)
		})
	}
	if n := up.total.Load(); n != 0 {
		t.Errorf("upstream received %d calls for invalid requests, want 0", n)
	}
}

func TestMoreThan50IDsIs400(t *testing.T) {
	up := newUpstream(t, serveOK)
	base := start(t, up, nil)
	resp, _ := aggregate(t, base, ids(51))
	wantStatus(t, resp, http.StatusBadRequest)
	if n := up.total.Load(); n != 0 {
		t.Errorf("upstream received %d calls, want 0", n)
	}
}

func TestExactly50IDsAreAccepted(t *testing.T) {
	base := start(t, newUpstream(t, serveOK), nil)
	resp, body := aggregate(t, base, ids(50))
	wantStatus(t, resp, http.StatusOK)
	if got := len(decode(t, body).Results); got != 50 {
		t.Errorf("results = %d, want 50", got)
	}
}

func ids(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = strconv.Itoa(i + 1)
	}
	return strings.Join(parts, ",")
}

// --- aggregation ---------------------------------------------------------------

func TestResultsKeepRequestOrderAndEmbedUpstreamJSON(t *testing.T) {
	// Lower ids answer later, so completion order is the reverse of request order.
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, id, _ int) {
		time.Sleep(time.Duration(5-id) * 15 * time.Millisecond)
		okItem(w, id)
	})
	base := start(t, up, func(c *aggregator.Config) { c.MaxConcurrency = 4 })

	resp, body := aggregate(t, base, "1,2,3,4")
	wantStatus(t, resp, http.StatusOK)
	got := decode(t, body)
	if len(got.Results) != 4 {
		t.Fatalf("results = %d, want 4: %s", len(got.Results), body)
	}
	for i, r := range got.Results {
		if r.ID != i+1 {
			t.Errorf("results[%d].id = %d, want %d", i, r.ID, i+1)
		}
		want := fmt.Sprintf(`{"item":%d,"name":"item-%d"}`, i+1, i+1)
		if string(r.Data) != want {
			t.Errorf("results[%d].data = %s, want %s (upstream JSON embedded as-is)", i, r.Data, want)
		}
	}
}

func TestEmptyErrorsEncodeAsArrayNotNull(t *testing.T) {
	base := start(t, newUpstream(t, serveOK), nil)
	_, body := aggregate(t, base, "1")
	if !strings.Contains(string(body), `"errors":[]`) {
		t.Errorf("body = %s, want \"errors\":[]", body)
	}
}

func TestPartialFailureIs200WithErrorsInRequestOrder(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, id, _ int) {
		if id == 3 || id == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		okItem(w, id)
	})
	base := start(t, up, nil)

	resp, body := aggregate(t, base, "1,2,3")
	wantStatus(t, resp, http.StatusOK)
	got := decode(t, body)
	if len(got.Results) != 1 || got.Results[0].ID != 2 {
		t.Errorf("results = %+v, want only id 2", got.Results)
	}
	if len(got.Errors) != 2 || got.Errors[0].ID != 1 || got.Errors[1].ID != 3 {
		t.Fatalf("errors = %+v, want ids 1 then 3", got.Errors)
	}
	if got.Errors[1].Error != "upstream status 500" {
		t.Errorf("error = %q, want %q", got.Errors[1].Error, "upstream status 500")
	}
}

func TestInvalidUpstreamJSONIsReportedAndNotRetried(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, _, _ int) {
		fmt.Fprint(w, "<html>not json</html>")
	})
	base := start(t, up, nil)

	resp, body := aggregate(t, base, "7")
	wantStatus(t, resp, http.StatusOK)
	got := decode(t, body)
	if len(got.Results) != 0 || len(got.Errors) != 1 || got.Errors[0].ID != 7 {
		t.Fatalf("body = %s, want one error for id 7", body)
	}
	if n := up.callsFor(7); n != 1 {
		t.Errorf("upstream calls = %d, want 1", n)
	}
}

// --- retries ------------------------------------------------------------------------

func TestServerErrorsAreRetriedTwiceThenReported(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, _, _ int) {
		http.Error(w, "boom", http.StatusBadGateway)
	})
	base := start(t, up, nil)

	_, body := aggregate(t, base, "5")
	if n := up.callsFor(5); n != 3 {
		t.Errorf("upstream calls = %d, want 3 (1 attempt + 2 retries)", n)
	}
	got := decode(t, body)
	if len(got.Errors) != 1 || got.Errors[0].Error != "upstream status 502" {
		t.Errorf("errors = %+v, want upstream status 502", got.Errors)
	}
}

func TestTransientServerErrorRecovers(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, id, attempt int) {
		if attempt < 3 {
			http.Error(w, "warming up", http.StatusServiceUnavailable)
			return
		}
		okItem(w, id)
	})
	base := start(t, up, nil)

	_, body := aggregate(t, base, "9")
	got := decode(t, body)
	if len(got.Results) != 1 || len(got.Errors) != 0 {
		t.Fatalf("body = %s, want one result and no errors", body)
	}
	if n := up.callsFor(9); n != 3 {
		t.Errorf("upstream calls = %d, want 3", n)
	}
}

func TestClientErrorsAreNeverRetried(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, _, _ int) {
		http.Error(w, "nope", http.StatusNotFound)
	})
	base := start(t, up, nil)

	_, body := aggregate(t, base, "4")
	if n := up.callsFor(4); n != 1 {
		t.Errorf("upstream calls = %d, want 1", n)
	}
	got := decode(t, body)
	if len(got.Errors) != 1 || got.Errors[0].Error != "upstream status 404" {
		t.Errorf("errors = %+v, want upstream status 404", got.Errors)
	}
}

func TestSlowCallTimesOutAndIsRetried(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request, id, attempt int) {
		if attempt == 1 {
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
			return
		}
		okItem(w, id)
	})
	base := start(t, up, func(c *aggregator.Config) { c.CallTimeout = 50 * time.Millisecond })

	_, body := aggregate(t, base, "2")
	got := decode(t, body)
	if len(got.Results) != 1 || len(got.Errors) != 0 {
		t.Fatalf("body = %s, want the retry to succeed", body)
	}
	if n := up.callsFor(2); n != 2 {
		t.Errorf("upstream calls = %d, want 2", n)
	}
}

func TestDroppedConnectionIsRetried(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, id, attempt int) {
		if attempt == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}
		okItem(w, id)
	})
	base := start(t, up, nil)

	_, body := aggregate(t, base, "6")
	if got := decode(t, body); len(got.Results) != 1 {
		t.Fatalf("body = %s, want the retry to succeed", body)
	}
	if n := up.callsFor(6); n != 2 {
		t.Errorf("upstream calls = %d, want 2", n)
	}
}

func TestBackoffDoublesBetweenRetries(t *testing.T) {
	const base = 30 * time.Millisecond
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, _, _ int) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	url := start(t, up, func(c *aggregator.Config) { c.BackoffBase = base })

	aggregate(t, url, "1")
	ts := up.timesFor(1)
	if len(ts) != 3 {
		t.Fatalf("upstream calls = %d, want 3", len(ts))
	}
	// Sleeping never returns early, so lower bounds are deterministic.
	if gap := ts[1].Sub(ts[0]); gap < base {
		t.Errorf("gap before retry 1 = %v, want >= %v", gap, base)
	}
	if gap := ts[2].Sub(ts[1]); gap < 2*base {
		t.Errorf("gap before retry 2 = %v, want >= %v", gap, 2*base)
	}
}

// --- concurrency -----------------------------------------------------------------------

func TestCallsRunConcurrently(t *testing.T) {
	// Every call waits until three calls are in flight at once. A sequential
	// implementation never gets there and every call times out instead.
	var arrived atomic.Int64
	release := make(chan struct{})
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, id, _ int) {
		if arrived.Add(1) == 3 {
			close(release)
		}
		select {
		case <-release:
			okItem(w, id)
		case <-time.After(2 * time.Second):
			http.Error(w, "not concurrent", http.StatusRequestTimeout)
		}
	})
	base := start(t, up, func(c *aggregator.Config) { c.MaxConcurrency = 3 })

	_, body := aggregate(t, base, "1,2,3")
	if got := decode(t, body); len(got.Results) != 3 {
		t.Fatalf("body = %s, want 3 results (calls were not concurrent)", body)
	}
}

func TestMaxConcurrencyBoundsCallsInFlight(t *testing.T) {
	up := newUpstream(t, func(w http.ResponseWriter, _ *http.Request, id, _ int) {
		time.Sleep(20 * time.Millisecond)
		okItem(w, id)
	})
	base := start(t, up, func(c *aggregator.Config) { c.MaxConcurrency = 2 })

	_, body := aggregate(t, base, ids(10))
	if got := decode(t, body); len(got.Results) != 10 {
		t.Fatalf("results = %d, want 10", len(got.Results))
	}
	if peak := up.peak.Load(); peak > 2 {
		t.Errorf("peak concurrent upstream calls = %d, want <= 2", peak)
	}
}

// --- cancellation ---------------------------------------------------------------------------

func TestClientDisconnectCancelsUpstreamCalls(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{})
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request, _, _ int) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
	})
	base := start(t, up, func(c *aggregator.Config) { c.MaxConcurrency = 1; c.MaxRetries = 0 })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/aggregate?ids=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}()

	waitFor(t, started, "upstream call to start")
	cancel()
	<-done
	waitFor(t, cancelled, "upstream call to be cancelled after the client disconnected")
}

func TestOverallDeadlineCancelsUpstreamAndReportsEveryID(t *testing.T) {
	var cancelled atomic.Int64
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request, _, _ int) {
		select {
		case <-r.Context().Done():
			cancelled.Add(1)
		case <-time.After(5 * time.Second):
		}
	})
	base := start(t, up, func(c *aggregator.Config) {
		c.MaxConcurrency = 2
		c.OverallTimeout = 100 * time.Millisecond
	})

	begin := time.Now()
	resp, body := aggregate(t, base, "1,2,3,4")
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Errorf("request took %v, want it to stop at the overall deadline", elapsed)
	}
	wantStatus(t, resp, http.StatusOK)
	got := decode(t, body)
	if len(got.Results) != 0 || len(got.Errors) != 4 {
		t.Fatalf("body = %s, want 4 errors and no results", body)
	}
	for i, e := range got.Errors {
		if e.ID != i+1 || e.Error != "cancelled" {
			t.Errorf("errors[%d] = %+v, want id %d with message %q", i, e, i+1, "cancelled")
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for cancelled.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n := cancelled.Load(); n < 2 {
		t.Errorf("cancelled upstream calls = %d, want the 2 in-flight calls cancelled", n)
	}
	if n := up.total.Load(); n > 2 {
		t.Errorf("upstream saw %d calls, want no new calls after the deadline", n)
	}
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}
