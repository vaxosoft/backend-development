package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"webdev/projects/P7-tested-service-docker-ci/starter"
	"webdev/projects/P7-tested-service-docker-ci/starter/tasks"
)

func newServer(t *testing.T, deps starter.Deps) (*httptest.Server, *starter.Service) {
	t.Helper()
	svc := starter.New(starter.DefaultConfig(), deps)
	ts := httptest.NewServer(svc)
	t.Cleanup(ts.Close)
	return ts, svc
}

func request(t *testing.T, method, url, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func decode(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", raw, err)
	}
	return m
}

func errorCode(t *testing.T, raw string) string {
	t.Helper()
	e, _ := decode(t, raw)["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestHealthzReportsOK(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	resp, body := request(t, "GET", ts.URL+"/healthz", "")
	if resp.StatusCode != http.StatusOK || decode(t, body)["status"] != "ok" {
		t.Errorf("got %d %s, want 200 {\"status\":\"ok\"}", resp.StatusCode, body)
	}
}

func TestReadyzIs503UntilServiceIsMarkedReady(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	resp, body := request(t, "GET", ts.URL+"/readyz", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 before SetReady", resp.StatusCode)
	}
	if code := errorCode(t, body); code != "unavailable" {
		t.Errorf("error code = %q, want unavailable", code)
	}
}

func TestReadyzIs200AfterSetReady(t *testing.T) {
	ts, svc := newServer(t, starter.Deps{})
	svc.SetReady(true)
	resp, body := request(t, "GET", ts.URL+"/readyz", "")
	if resp.StatusCode != http.StatusOK || decode(t, body)["status"] != "ready" {
		t.Errorf("got %d %s, want 200 {\"status\":\"ready\"}", resp.StatusCode, body)
	}
}

func TestReadyzGoesBackTo503WhenReadinessIsRevoked(t *testing.T) {
	ts, svc := newServer(t, starter.Deps{})
	svc.SetReady(true)
	svc.SetReady(false)
	if resp, _ := request(t, "GET", ts.URL+"/readyz", ""); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 after SetReady(false)", resp.StatusCode)
	}
}

func TestMetricsCountsEveryRequestIncludingItself(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	for range 3 {
		request(t, "GET", ts.URL+"/healthz", "")
	}
	resp, body := request(t, "GET", ts.URL+"/metrics", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if body != "requests_total 4\n" {
		t.Errorf("body = %q, want %q", body, "requests_total 4\n")
	}
}

func TestMetricsCountsUnknownRoutesAndErrors(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	request(t, "GET", ts.URL+"/nope", "")
	request(t, "GET", ts.URL+"/tasks/99", "")
	_, body := request(t, "GET", ts.URL+"/metrics", "")
	if body != "requests_total 3\n" {
		t.Errorf("body = %q, want %q", body, "requests_total 3\n")
	}
}

func TestCreateTaskThenFetchIt(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	resp, body := request(t, "POST", ts.URL+"/tasks", `{"title":"  Write docs  "}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", resp.StatusCode, body)
	}
	if loc := resp.Header.Get("Location"); loc != "/tasks/1" {
		t.Errorf("Location = %q, want /tasks/1", loc)
	}
	created := decode(t, body)
	if created["title"] != "Write docs" || created["status"] != "todo" || created["id"] != float64(1) {
		t.Errorf("created = %v, want trimmed title, default status todo, id 1", created)
	}

	resp, body = request(t, "GET", ts.URL+"/tasks/1", "")
	if resp.StatusCode != http.StatusOK || decode(t, body)["title"] != "Write docs" {
		t.Errorf("GET /tasks/1 = %d %s", resp.StatusCode, body)
	}
}

func TestCreateTaskValidationReturns422WithFields(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	for name, tc := range map[string]struct{ body, field string }{
		"blank title":    {`{"title":"   "}`, "title"},
		"long title":     {`{"title":"` + strings.Repeat("x", 201) + `"}`, "title"},
		"unknown status": {`{"title":"ok","status":"blocked"}`, "status"},
	} {
		t.Run(name, func(t *testing.T) {
			resp, body := request(t, "POST", ts.URL+"/tasks", tc.body)
			if resp.StatusCode != http.StatusUnprocessableEntity || errorCode(t, body) != "validation_failed" {
				t.Fatalf("got %d %s, want 422 validation_failed", resp.StatusCode, body)
			}
			e, _ := decode(t, body)["error"].(map[string]any)
			if fields, _ := e["fields"].(map[string]any); fields[tc.field] == nil {
				t.Errorf("fields = %v, want an entry for %q", fields, tc.field)
			}
		})
	}
}

func TestCreateTaskWithMalformedJSONReturns400(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	resp, body := request(t, "POST", ts.URL+"/tasks", `{"title":`)
	if resp.StatusCode != http.StatusBadRequest || errorCode(t, body) != "bad_request" {
		t.Errorf("got %d %s, want 400 bad_request", resp.StatusCode, body)
	}
}

func TestUnknownTaskReturns404(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	for _, id := range []string{"42", "abc"} {
		resp, body := request(t, "GET", ts.URL+"/tasks/"+id, "")
		if resp.StatusCode != http.StatusNotFound || errorCode(t, body) != "not_found" {
			t.Errorf("GET /tasks/%s = %d %s, want 404 not_found", id, resp.StatusCode, body)
		}
	}
}

func TestListTasksReturnsItemsInCreationOrder(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{})
	for _, title := range []string{"first", "second"} {
		request(t, "POST", ts.URL+"/tasks", `{"title":"`+title+`"}`)
	}
	_, body := request(t, "GET", ts.URL+"/tasks", "")
	m := decode(t, body)
	items, _ := m["items"].([]any)
	if len(items) != 2 || m["total"] != float64(2) {
		t.Fatalf("body = %s, want 2 items", body)
	}
	if items[0].(map[string]any)["title"] != "first" || items[1].(map[string]any)["title"] != "second" {
		t.Errorf("items = %v, want creation order", items)
	}
}

type brokenStore struct{ tasks.Store }

var errDisk = errors.New("disk exploded: secret-detail")

func (brokenStore) List(context.Context) ([]tasks.Task, error)   { return nil, errDisk }
func (brokenStore) Get(context.Context, int) (tasks.Task, error) { return tasks.Task{}, errDisk }
func (brokenStore) Create(context.Context, tasks.Task) (tasks.Task, error) {
	return tasks.Task{}, errDisk
}

func TestStoreFailureReturns500WithoutLeakingDetails(t *testing.T) {
	ts, _ := newServer(t, starter.Deps{Store: brokenStore{}})
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/tasks", ""},
		{"GET", "/tasks/1", ""},
		{"POST", "/tasks", `{"title":"x"}`},
	} {
		resp, body := request(t, tc.method, ts.URL+tc.path, tc.body)
		if resp.StatusCode != http.StatusInternalServerError || errorCode(t, body) != "internal" {
			t.Errorf("%s %s = %d %s, want 500 internal", tc.method, tc.path, resp.StatusCode, body)
		}
		if strings.Contains(body, "secret-detail") {
			t.Errorf("%s %s leaked the internal error: %s", tc.method, tc.path, body)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestEachRequestIsLoggedWithMethodPathAndStatus(t *testing.T) {
	var out syncBuffer
	log := slog.New(slog.NewJSONHandler(&out, nil))
	ts, _ := newServer(t, starter.Deps{Logger: log})
	request(t, "GET", ts.URL+"/tasks/7", "")

	// The line is written after the response, so give the handler a moment.
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(out.String(), `"msg":"request"`) {
		if time.Now().After(deadline) {
			t.Fatalf("no request log line within 2s; log output: %q", out.String())
		}
		time.Sleep(2 * time.Millisecond)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(out.String(), "\n", 2)[0]), &line); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if line["method"] != "GET" || line["path"] != "/tasks/7" || line["status"] != float64(404) {
		t.Errorf("log line = %v, want method GET, path /tasks/7, status 404", line)
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Errorf("log line = %v, want a duration_ms attribute", line)
	}
}
