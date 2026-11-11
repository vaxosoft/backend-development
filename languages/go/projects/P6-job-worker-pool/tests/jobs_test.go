package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"webdev/projects/P6-job-worker-pool/starter"
)

// fastConfig keeps retries and timeouts tiny so tests stay quick.
func fastConfig() starter.Config {
	cfg := starter.DefaultConfig()
	cfg.RetryDelay = time.Millisecond
	cfg.JobTimeout = 5 * time.Second
	cfg.DrainTimeout = 2 * time.Second
	return cfg
}

func newServer(t *testing.T, cfg starter.Config) (*httptest.Server, *starter.Service) {
	t.Helper()
	svc := starter.New(cfg)
	ts := httptest.NewServer(svc)
	t.Cleanup(func() {
		ts.Close()
		// An already-expired context cancels whatever is still running.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = svc.Shutdown(ctx)
	})
	return ts, svc
}

type response struct {
	Status int
	Header http.Header
	Body   map[string]any
}

func do(t *testing.T, method, url, body string) response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := response{Status: resp.StatusCode, Header: resp.Header}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &out.Body); err != nil {
			t.Fatalf("%s %s: body is not a JSON object: %q", method, url, raw)
		}
	}
	return out
}

func submit(t *testing.T, ts *httptest.Server, body string) int {
	t.Helper()
	r := do(t, "POST", ts.URL+"/jobs", body)
	if r.Status != http.StatusAccepted {
		t.Fatalf("POST /jobs %s: status = %d, want 202 (body %v)", body, r.Status, r.Body)
	}
	id, ok := r.Body["id"].(float64)
	if !ok {
		t.Fatalf("response has no numeric id: %v", r.Body)
	}
	return int(id)
}

func getJob(t *testing.T, ts *httptest.Server, id int) response {
	t.Helper()
	return do(t, "GET", ts.URL+"/jobs/"+strconv.Itoa(id), "")
}

// waitFor polls until the job has the wanted status.
func waitFor(t *testing.T, ts *httptest.Server, id int, want string) response {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		r := getJob(t, ts, id)
		if r.Status == http.StatusOK && r.Body["status"] == want {
			return r
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %d: status = %v, want %q after 3s", id, r.Body["status"], want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func errorCode(r response) string {
	e, _ := r.Body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func sleepJob(ms int) string {
	return `{"type":"sleep","payload":{"ms":` + strconv.Itoa(ms) + `}}`
}

func TestCreateJobReturns202WithIDAndLocation(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	r := do(t, "POST", ts.URL+"/jobs", `{"type":"reverse","payload":{"text":"abc"}}`)
	if r.Status != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", r.Status)
	}
	if r.Body["id"] != float64(1) {
		t.Errorf("first job id = %v, want 1", r.Body["id"])
	}
	if got := r.Header.Get("Location"); got != "/jobs/1" {
		t.Errorf("Location = %q, want /jobs/1", got)
	}
}

func TestReverseJobSucceedsWithReversedText(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	id := submit(t, ts, `{"type":"reverse","payload":{"text":"héllo, 世界"}}`)
	r := waitFor(t, ts, id, "succeeded")
	if r.Body["result"] != "界世 ,olléh" {
		t.Errorf("result = %v, want %q", r.Body["result"], "界世 ,olléh")
	}
	if r.Body["type"] != "reverse" || r.Body["attempts"] != float64(1) {
		t.Errorf("type/attempts = %v/%v, want reverse/1", r.Body["type"], r.Body["attempts"])
	}
	if v, present := r.Body["error"]; !present || v != nil {
		t.Errorf("error = %v (present=%v), want explicit null", v, present)
	}
}

func TestSleepJobSucceeds(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	id := submit(t, ts, sleepJob(10))
	r := waitFor(t, ts, id, "succeeded")
	if r.Body["result"] == nil {
		t.Error("sleep job has no result")
	}
}

func TestFailJobRetriesUntilMaxAttemptsThenFails(t *testing.T) {
	cfg := fastConfig()
	cfg.MaxAttempts = 3
	ts, _ := newServer(t, cfg)
	id := submit(t, ts, `{"type":"fail","payload":{}}`)
	r := waitFor(t, ts, id, "failed")
	if r.Body["attempts"] != float64(3) {
		t.Errorf("attempts = %v, want 3", r.Body["attempts"])
	}
	if s, _ := r.Body["error"].(string); s == "" {
		t.Errorf("error = %v, want a message", r.Body["error"])
	}
	if r.Body["result"] != nil {
		t.Errorf("result = %v, want null", r.Body["result"])
	}
}

func TestMaxAttemptsIsConfigurable(t *testing.T) {
	cfg := fastConfig()
	cfg.MaxAttempts = 1
	ts, _ := newServer(t, cfg)
	id := submit(t, ts, `{"type":"fail"}`)
	r := waitFor(t, ts, id, "failed")
	if r.Body["attempts"] != float64(1) {
		t.Errorf("attempts = %v, want 1", r.Body["attempts"])
	}
}

func TestJobTimeoutFailsTheAttempt(t *testing.T) {
	cfg := fastConfig()
	cfg.JobTimeout = 20 * time.Millisecond
	cfg.MaxAttempts = 2
	ts, _ := newServer(t, cfg)
	id := submit(t, ts, sleepJob(2000))
	r := waitFor(t, ts, id, "failed")
	if r.Body["attempts"] != float64(2) {
		t.Errorf("attempts = %v, want 2 (a timeout counts as a failed attempt)", r.Body["attempts"])
	}
}

func TestInvalidJobsAreRejectedWith422AndFieldErrors(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	cases := []struct{ name, body, field string }{
		{"unknown type", `{"type":"explode","payload":{}}`, "type"},
		{"missing type", `{"payload":{}}`, "type"},
		{"reverse without text", `{"type":"reverse","payload":{}}`, "payload.text"},
		{"reverse text not a string", `{"type":"reverse","payload":{"text":5}}`, "payload.text"},
		{"sleep without ms", `{"type":"sleep","payload":{}}`, "payload.ms"},
		{"sleep over the limit", sleepJob(5001), "payload.ms"},
		{"sleep negative", sleepJob(-1), "payload.ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := do(t, "POST", ts.URL+"/jobs", tc.body)
			if r.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", r.Status)
			}
			if code := errorCode(r); code != "validation_failed" {
				t.Errorf("code = %q, want validation_failed", code)
			}
			e, _ := r.Body["error"].(map[string]any)
			fields, _ := e["fields"].(map[string]any)
			if _, ok := fields[tc.field]; !ok {
				t.Errorf("fields = %v, want an entry for %q", fields, tc.field)
			}
		})
	}
}

func TestSleepAtTheLimitIsAccepted(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	submit(t, ts, sleepJob(5000))
}

func TestMalformedJSONIs400(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	r := do(t, "POST", ts.URL+"/jobs", `{"type":`)
	if r.Status != http.StatusBadRequest || errorCode(r) != "bad_request" {
		t.Errorf("got %d %q, want 400 bad_request", r.Status, errorCode(r))
	}
}

func TestUnknownJobIs404(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	for _, id := range []string{"999", "abc"} {
		r := do(t, "GET", ts.URL+"/jobs/"+id, "")
		if r.Status != http.StatusNotFound || errorCode(r) != "not_found" {
			t.Errorf("GET /jobs/%s: got %d %q, want 404 not_found", id, r.Status, errorCode(r))
		}
	}
}

func TestListJobsIsNewestFirst(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	for _, text := range []string{"a", "b", "c"} {
		submit(t, ts, `{"type":"reverse","payload":{"text":"`+text+`"}}`)
	}
	r := do(t, "GET", ts.URL+"/jobs", "")
	items, _ := r.Body["items"].([]any)
	if r.Status != http.StatusOK || len(items) != 3 {
		t.Fatalf("status %d with %d items, want 200 with 3", r.Status, len(items))
	}
	var ids []float64
	for _, it := range items {
		ids = append(ids, it.(map[string]any)["id"].(float64))
	}
	if ids[0] != 3 || ids[1] != 2 || ids[2] != 1 {
		t.Errorf("ids = %v, want [3 2 1]", ids)
	}
	if r.Body["total"] != float64(3) || r.Body["limit"] != float64(20) || r.Body["offset"] != float64(0) {
		t.Errorf("envelope = %v, want total 3, limit 20, offset 0", r.Body)
	}
}

func TestListJobsPaginatesAndClampsLimit(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	for range 5 {
		submit(t, ts, `{"type":"reverse","payload":{"text":"x"}}`)
	}
	r := do(t, "GET", ts.URL+"/jobs?limit=2&offset=1", "")
	items, _ := r.Body["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != float64(4) || items[1].(map[string]any)["id"] != float64(3) {
		t.Errorf("items = %v, want ids 4 and 3", items)
	}
	r = do(t, "GET", ts.URL+"/jobs?limit=500", "")
	if r.Body["limit"] != float64(100) {
		t.Errorf("limit = %v, want clamped to 100", r.Body["limit"])
	}
	r = do(t, "GET", ts.URL+"/jobs?offset=99", "")
	if items, _ := r.Body["items"].([]any); r.Status != http.StatusOK || items == nil || len(items) != 0 {
		t.Errorf("offset past the end: status %d items %v, want 200 and []", r.Status, r.Body["items"])
	}
}

func TestListJobsRejectsBadPagingWith400(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	for _, q := range []string{"limit=abc", "limit=-1", "offset=-5"} {
		if r := do(t, "GET", ts.URL+"/jobs?"+q, ""); r.Status != http.StatusBadRequest {
			t.Errorf("GET /jobs?%s: status = %d, want 400", q, r.Status)
		}
	}
}

// saturate fills a one-worker pool: job 1 runs, jobs 2..n queue up.
func saturate(t *testing.T, ts *httptest.Server, queued int) []int {
	t.Helper()
	ids := []int{submit(t, ts, sleepJob(3000))}
	waitFor(t, ts, ids[0], "running")
	for range queued {
		ids = append(ids, submit(t, ts, sleepJob(3000)))
	}
	return ids
}

func TestFullQueueReturns429WithRetryAfter(t *testing.T) {
	cfg := fastConfig()
	cfg.Workers, cfg.QueueSize = 1, 1
	ts, _ := newServer(t, cfg)
	saturate(t, ts, 1)

	r := do(t, "POST", ts.URL+"/jobs", sleepJob(10))
	if r.Status != http.StatusTooManyRequests || errorCode(r) != "rate_limited" {
		t.Fatalf("got %d %q, want 429 rate_limited", r.Status, errorCode(r))
	}
	if r.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
	if got := do(t, "GET", ts.URL+"/jobs", "").Body["total"]; got != float64(2) {
		t.Errorf("total = %v, want 2 (the rejected job must not be stored)", got)
	}
}

func TestOnlyConfiguredWorkersRunAtOnce(t *testing.T) {
	cfg := fastConfig()
	cfg.Workers, cfg.QueueSize = 2, 5
	ts, _ := newServer(t, cfg)
	ids := []int{}
	for range 4 {
		ids = append(ids, submit(t, ts, sleepJob(3000)))
	}
	waitFor(t, ts, ids[0], "running")
	waitFor(t, ts, ids[1], "running")
	time.Sleep(20 * time.Millisecond)
	for _, id := range ids[2:] {
		if got := getJob(t, ts, id).Body["status"]; got != "queued" {
			t.Errorf("job %d status = %v, want queued while both workers are busy", id, got)
		}
	}
}

func TestCancelQueuedJobNeverRuns(t *testing.T) {
	cfg := fastConfig()
	cfg.Workers, cfg.QueueSize = 1, 2
	ts, _ := newServer(t, cfg)
	ids := saturate(t, ts, 1)
	queued := ids[1]

	if r := do(t, "DELETE", ts.URL+"/jobs/"+strconv.Itoa(queued), ""); r.Status != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", r.Status)
	}
	r := getJob(t, ts, queued)
	if r.Body["status"] != "cancelled" || r.Body["attempts"] != float64(0) {
		t.Errorf("status/attempts = %v/%v, want cancelled/0", r.Body["status"], r.Body["attempts"])
	}
}

func TestCancelRunningJobStopsIt(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	id := submit(t, ts, sleepJob(3000))
	waitFor(t, ts, id, "running")
	if r := do(t, "DELETE", ts.URL+"/jobs/"+strconv.Itoa(id), ""); r.Status != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", r.Status)
	}
	waitFor(t, ts, id, "cancelled")
}

func TestCancelledRunningJobFreesItsWorker(t *testing.T) {
	cfg := fastConfig()
	cfg.Workers = 1
	ts, _ := newServer(t, cfg)
	first := submit(t, ts, sleepJob(3000))
	waitFor(t, ts, first, "running")
	second := submit(t, ts, `{"type":"reverse","payload":{"text":"ok"}}`)
	do(t, "DELETE", ts.URL+"/jobs/"+strconv.Itoa(first), "")
	waitFor(t, ts, second, "succeeded")
}

func TestCancelFinishedOrUnknownJob(t *testing.T) {
	ts, _ := newServer(t, fastConfig())
	id := submit(t, ts, `{"type":"reverse","payload":{"text":"x"}}`)
	waitFor(t, ts, id, "succeeded")

	r := do(t, "DELETE", ts.URL+"/jobs/"+strconv.Itoa(id), "")
	if r.Status != http.StatusConflict || errorCode(r) != "conflict" {
		t.Errorf("finished job: got %d %q, want 409 conflict", r.Status, errorCode(r))
	}
	if r := do(t, "DELETE", ts.URL+"/jobs/999", ""); r.Status != http.StatusNotFound {
		t.Errorf("unknown job: status = %d, want 404", r.Status)
	}
}

func TestShutdownDrainsQueuedJobs(t *testing.T) {
	cfg := fastConfig()
	cfg.Workers, cfg.QueueSize = 1, 5
	ts, svc := newServer(t, cfg)
	var ids []int
	for range 3 {
		ids = append(ids, submit(t, ts, sleepJob(20)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	for _, id := range ids {
		if got := getJob(t, ts, id).Body["status"]; got != "succeeded" {
			t.Errorf("job %d status = %v after Shutdown, want succeeded", id, got)
		}
	}
}

func TestShutdownRejectsNewJobsWith503(t *testing.T) {
	ts, svc := newServer(t, fastConfig())
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	r := do(t, "POST", ts.URL+"/jobs", `{"type":"reverse","payload":{"text":"x"}}`)
	if r.Status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", r.Status)
	}
}

func TestShutdownDrainTimeoutCancelsRunningJobs(t *testing.T) {
	ts, svc := newServer(t, fastConfig())
	id := submit(t, ts, sleepJob(3000))
	waitFor(t, ts, id, "running")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := svc.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown error = %v, want context.DeadlineExceeded", err)
	}
	if got := getJob(t, ts, id).Body["status"]; got != "cancelled" {
		t.Errorf("status = %v, want cancelled", got)
	}
}

func TestRunDrainsJobsThenStopsListening(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { done <- starter.Run(ctx, ln, fastConfig()) }()

	base := "http://" + addr
	begin := time.Now()
	id := submit(t, &httptest.Server{URL: base}, sleepJob(60))
	waitFor(t, &httptest.Server{URL: base}, id, "running")
	stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return within 3s of cancellation")
	}
	if elapsed := time.Since(begin); elapsed < 60*time.Millisecond {
		t.Errorf("Run returned after %v, before the running job could finish", elapsed)
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Error("server still accepts connections after Run returned")
	}
}
