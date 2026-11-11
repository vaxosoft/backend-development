package taskclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

type fakeServer struct {
	mu       sync.Mutex
	tasks    []Task
	requests []string // "METHOD path?query"
	headers  []http.Header
	bodies   []string
	failList bool
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var raw json.RawMessage
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&raw)
	}
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.headers = append(f.headers, r.Header.Clone())
	f.bodies = append(f.bodies, string(raw))

	reply := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	fail := func(code int, kind, msg string, fields map[string]string) {
		reply(code, map[string]any{"error": map[string]any{"code": kind, "message": msg, "fields": fields}})
	}

	if r.Header.Get("Authorization") != "Bearer tok" {
		fail(401, "unauthorized", "bad token", nil)
		return
	}
	find := func() (int, bool) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		for i, t := range f.tasks {
			if t.ID == id {
				return i, true
			}
		}
		return 0, false
	}

	switch r.Method + " " + r.URL.Path {
	case "POST /tasks":
		var nt NewTask
		_ = json.Unmarshal(raw, &nt)
		if nt.Title == "" {
			fail(422, "validation_failed", "invalid task", map[string]string{"title": "required"})
			return
		}
		t := Task{ID: len(f.tasks) + 1, Title: nt.Title, Status: "todo", Tags: nt.Tags}
		f.tasks = append(f.tasks, t)
		reply(201, t)
	case "GET /tasks":
		if f.failList {
			http.Error(w, "<html>bad gateway</html>", http.StatusBadGateway)
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		var all []Task
		for _, t := range f.tasks {
			if s := q.Get("status"); s == "" || s == t.Status {
				all = append(all, t)
			}
		}
		end := len(all)
		if limit > 0 {
			end = min(offset+limit, len(all))
		}
		page := []Task{}
		if offset < len(all) {
			page = all[offset:end]
		}
		reply(200, ListResult{Items: page, Total: len(all), Limit: limit, Offset: offset})
	default:
		r.SetPathValue("id", r.URL.Path[len("/tasks/"):])
		i, ok := find()
		if !ok {
			fail(404, "not_found", "task not found", nil)
			return
		}
		switch r.Method {
		case "GET":
			reply(200, f.tasks[i])
		case "PATCH":
			var p struct {
				Status string `json:"status"`
			}
			_ = json.Unmarshal(raw, &p)
			f.tasks[i].Status = p.Status
			reply(200, f.tasks[i])
		case "DELETE":
			f.tasks = append(f.tasks[:i], f.tasks[i+1:]...)
			w.WriteHeader(204)
		}
	}
}

func setup(t *testing.T, n int) (*Client, *fakeServer) {
	t.Helper()
	fs := &fakeServer{}
	for i := 1; i <= n; i++ {
		fs.tasks = append(fs.tasks, Task{ID: i, Title: fmt.Sprintf("task %d", i), Status: "todo"})
	}
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)
	return New(srv.URL, "tok", srv.Client()), fs
}

func TestCreateGetSetStatusDelete(t *testing.T) {
	c, fs := setup(t, 0)
	ctx := context.Background()

	created, err := c.Create(ctx, NewTask{Title: "Write docs", Tags: []string{"docs"}})
	if err != nil || created.ID != 1 || created.Title != "Write docs" {
		t.Fatalf("Create = %+v, %v", created, err)
	}
	got, err := c.Get(ctx, 1)
	if err != nil || got.Title != "Write docs" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	upd, err := c.SetStatus(ctx, 1, "done")
	if err != nil || upd.Status != "done" {
		t.Fatalf("SetStatus = %+v, %v", upd, err)
	}
	if fs.bodies[2] != `{"status":"done"}` {
		t.Errorf("PATCH body = %s", fs.bodies[2])
	}
	if err := c.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := c.Get(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete: %v, want ErrNotFound", err)
	}
}

func TestRequestHeaders(t *testing.T) {
	c, fs := setup(t, 1)
	ctx := context.Background()
	if _, err := c.Get(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Create(ctx, NewTask{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	for i, h := range fs.headers {
		if h.Get("Authorization") != "Bearer tok" || h.Get("Accept") != "application/json" || h.Get("User-Agent") != "task-client/1.0" {
			t.Errorf("request %d headers = %v", i, h)
		}
	}
	if ct := fs.headers[1].Get("Content-Type"); ct != "application/json" {
		t.Errorf("POST Content-Type = %q", ct)
	}
	if ct := fs.headers[0].Get("Content-Type"); ct != "" {
		t.Errorf("GET must not send Content-Type, got %q", ct)
	}
}

func TestErrorEnvelopeBecomesAPIError(t *testing.T) {
	c, _ := setup(t, 0)
	_, err := c.Create(context.Background(), NewTask{})
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if ae.Status != 422 || ae.Code != "validation_failed" || ae.Fields["title"] != "required" {
		t.Errorf("APIError = %+v", ae)
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("422 must not match ErrNotFound")
	}

	bad := New(c.BaseURL, "wrong", c.HTTP)
	_, err = bad.Get(context.Background(), 1)
	if !errors.As(err, &ae) || ae.Status != 401 {
		t.Errorf("wrong token: %v", err)
	}
}

func TestNonEnvelopeErrorBodyStillGivesAPIError(t *testing.T) {
	c, fs := setup(t, 1)
	fs.failList = true
	_, err := c.List(context.Background(), ListOptions{})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 502 || ae.Code != "" {
		t.Fatalf("err = %v, want APIError{Status:502} with empty code", err)
	}
}

func TestListEncodesQuery(t *testing.T) {
	c, fs := setup(t, 3)
	res, err := c.List(context.Background(), ListOptions{Status: "todo", Tag: "a&b c", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || res.Total != 3 {
		t.Errorf("result = %+v", res)
	}
	if want := "GET /tasks?limit=2&status=todo&tag=a%26b+c"; fs.requests[0] != want {
		t.Errorf("request = %q, want %q", fs.requests[0], want)
	}
	if _, err := c.List(context.Background(), ListOptions{}); err != nil {
		t.Fatal(err)
	}
	if fs.requests[1] != "GET /tasks" {
		t.Errorf("empty options must give no query string, got %q", fs.requests[1])
	}
}

func TestAllFollowsPages(t *testing.T) {
	c, fs := setup(t, 5)
	var ids []int
	for task, err := range c.All(context.Background(), "", 2) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}
	if fmt.Sprint(ids) != "[1 2 3 4 5]" {
		t.Errorf("ids = %v", ids)
	}
	if len(fs.requests) != 3 {
		t.Errorf("requests = %v, want 3 pages", fs.requests)
	}
}

func TestAllStopsEarlyAndReportsErrors(t *testing.T) {
	c, fs := setup(t, 6)
	for range c.All(context.Background(), "", 2) {
		break
	}
	if len(fs.requests) != 1 {
		t.Errorf("breaking out of the loop must stop fetching; requests = %v", fs.requests)
	}

	bad := New(c.BaseURL, "wrong", c.HTTP)
	n := 0
	for _, err := range bad.All(context.Background(), "", 2) {
		n++
		var ae *APIError
		if !errors.As(err, &ae) || ae.Status != 401 {
			t.Errorf("err = %v", err)
		}
	}
	if n != 1 {
		t.Errorf("yielded %d times, want exactly one error", n)
	}
}

func TestNewDefaultsClient(t *testing.T) {
	c := New("http://x.test", "t", nil)
	if c.HTTP == nil || c.HTTP.Timeout == 0 {
		t.Fatalf("New(nil) must install a client with a timeout, got %+v", c.HTTP)
	}
}
