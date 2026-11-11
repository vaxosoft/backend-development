// Example 05: a small typed client for the Task Tracker API, tested against an in-process fake.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Task struct {
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
}

type NewTask struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags,omitempty"`
}

type ListResult struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// APIError is the decoded {"error":{...}} envelope.
type APIError struct {
	Status  int
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message) }

var ErrNotFound = errors.New("not found")

func (e *APIError) Is(target error) bool {
	return target == ErrNotFound && e.Status == http.StatusNotFound
}

type Client struct {
	base  string
	token string
	http  *http.Client
}

func NewClient(base, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{base: base, token: token, http: hc}
}

func (c *Client) Create(ctx context.Context, nt NewTask) (Task, error) {
	var t Task
	return t, c.do(ctx, http.MethodPost, "/tasks", nil, nt, &t)
}

func (c *Client) Get(ctx context.Context, id int) (Task, error) {
	var t Task
	return t, c.do(ctx, http.MethodGet, "/tasks/"+strconv.Itoa(id), nil, nil, &t)
}

func (c *Client) List(ctx context.Context, status string, limit int) (ListResult, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var res ListResult
	return res, c.do(ctx, http.MethodGet, "/tasks", q, nil, &res)
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var env struct {
			Error APIError `json:"error"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&env); err != nil {
			return fmt.Errorf("%s %s: status %d with unreadable error body", method, path, resp.StatusCode)
		}
		env.Error.Status = resp.StatusCode
		return &env.Error
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func main() {
	srv := httptest.NewServer(fakeAPI())
	defer srv.Close()

	ctx := context.Background()
	c := NewClient(srv.URL, "s3cret", srv.Client())

	t, err := c.Create(ctx, NewTask{Title: "Write docs", Tags: []string{"docs"}})
	fmt.Printf("create: %+v err=%v\n", t, err)

	_, err = c.Create(ctx, NewTask{Title: ""})
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		fmt.Printf("create invalid: status=%d code=%s fields=%v\n", apiErr.Status, apiErr.Code, apiErr.Fields)
	}

	_, err = c.Get(ctx, 42)
	fmt.Println("get 42:", err, "| is not found:", errors.Is(err, ErrNotFound))

	res, err := c.List(ctx, "todo", 10)
	fmt.Printf("list: total=%d limit=%d first=%q err=%v\n", res.Total, res.Limit, res.Items[0].Title, err)

	_, err = NewClient(srv.URL, "wrong", srv.Client()).Get(ctx, 1)
	fmt.Println("bad token:", err)
}

// fakeAPI stands in for the real service so the example is offline and deterministic.
func fakeAPI() http.Handler {
	var mu sync.Mutex
	tasks := map[int]Task{}
	writeJSON := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(v)
	}
	apiErr := func(w http.ResponseWriter, code int, kind, msg string, fields map[string]string) {
		writeJSON(w, code, map[string]any{"error": map[string]any{"code": kind, "message": msg, "fields": fields}})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var nt NewTask
		if err := json.NewDecoder(r.Body).Decode(&nt); err != nil {
			apiErr(w, 400, "bad_request", "malformed JSON", nil)
			return
		}
		if nt.Title == "" {
			apiErr(w, 422, "validation_failed", "invalid task", map[string]string{"title": "must be 1-200 characters"})
			return
		}
		t := Task{ID: len(tasks) + 1, Title: nt.Title, Status: "todo", Tags: nt.Tags}
		tasks[t.ID] = t
		writeJSON(w, 201, t)
	})
	mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		id, _ := strconv.Atoi(r.PathValue("id"))
		t, ok := tasks[id]
		if !ok {
			apiErr(w, 404, "not_found", "task not found", nil)
			return
		}
		writeJSON(w, 200, t)
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items := []Task{}
		for id := 1; id <= len(tasks); id++ {
			if s := r.URL.Query().Get("status"); s == "" || tasks[id].Status == s {
				items = append(items, tasks[id])
			}
		}
		writeJSON(w, 200, ListResult{Items: items, Total: len(items), Limit: limit})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer s3cret" {
			apiErr(w, 401, "unauthorized", "missing or invalid credentials", nil)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
