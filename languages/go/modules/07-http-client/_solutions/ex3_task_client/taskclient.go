// Package taskclient is exercise 3: a typed client for the Task Tracker API.
package taskclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Task is the wire representation of a task.
type Task struct {
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
}

// NewTask is the body of POST /tasks.
type NewTask struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags,omitempty"`
}

// ListOptions are the query parameters of GET /tasks. Zero values are omitted from the URL.
type ListOptions struct {
	Status string
	Tag    string
	Limit  int
	Offset int
}

// ListResult is the envelope returned by GET /tasks.
type ListResult struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// ErrNotFound matches (errors.Is) any *APIError with HTTP status 404.
var ErrNotFound = errors.New("not found")

// APIError is a non-2xx answer. Code, Message and Fields come from the JSON error envelope
// {"error":{"code":...,"message":...,"fields":{...}}}; they stay empty when the body is not that envelope
// (for example an HTML page from a proxy).
type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d %s: %s", e.Status, e.Code, e.Message)
}

// Is makes errors.Is(err, ErrNotFound) work for 404 answers.
func (e *APIError) Is(target error) bool {
	return target == ErrNotFound && e.Status == http.StatusNotFound
}

// Client talks to one Task Tracker server.
type Client struct {
	BaseURL string // no trailing slash, e.g. http://127.0.0.1:8080
	Token   string
	HTTP    *http.Client
}

// New returns a client; a nil hc means a client with a 10 second timeout.
func New(baseURL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{BaseURL: baseURL, Token: token, HTTP: hc}
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
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "task-client/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &APIError{Status: resp.StatusCode}
		var env struct {
			Error struct {
				Code    string            `json:"code"`
				Message string            `json:"message"`
				Fields  map[string]string `json:"fields"`
			} `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&env) == nil {
			apiErr.Code, apiErr.Message, apiErr.Fields = env.Error.Code, env.Error.Message, env.Error.Fields
		}
		return apiErr
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Create sends POST /tasks and returns the created task (201).
func (c *Client) Create(ctx context.Context, nt NewTask) (Task, error) {
	var t Task
	err := c.do(ctx, http.MethodPost, "/tasks", nil, nt, &t)
	return t, err
}

// Get sends GET /tasks/{id}.
func (c *Client) Get(ctx context.Context, id int) (Task, error) {
	var t Task
	err := c.do(ctx, http.MethodGet, "/tasks/"+strconv.Itoa(id), nil, nil, &t)
	return t, err
}

// SetStatus sends PATCH /tasks/{id} with the body {"status": status}.
func (c *Client) SetStatus(ctx context.Context, id int, status string) (Task, error) {
	var t Task
	err := c.do(ctx, http.MethodPatch, "/tasks/"+strconv.Itoa(id), nil, map[string]string{"status": status}, &t)
	return t, err
}

// Delete sends DELETE /tasks/{id}; success is 204 with no body.
func (c *Client) Delete(ctx context.Context, id int) error {
	return c.do(ctx, http.MethodDelete, "/tasks/"+strconv.Itoa(id), nil, nil, nil)
}

// List sends GET /tasks with the options encoded as a query string (values must be URL-escaped).
func (c *Client) List(ctx context.Context, o ListOptions) (ListResult, error) {
	q := url.Values{}
	if o.Status != "" {
		q.Set("status", o.Status)
	}
	if o.Tag != "" {
		q.Set("tag", o.Tag)
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Offset > 0 {
		q.Set("offset", strconv.Itoa(o.Offset))
	}
	var res ListResult
	err := c.do(ctx, http.MethodGet, "/tasks", q, nil, &res)
	return res, err
}

// All iterates over every task with the given status ("" = any), requesting pages of pageSize
// until offset >= total. It stops at the first error (yielding it) and stops fetching when
// the consumer breaks out of the loop.
func (c *Client) All(ctx context.Context, status string, pageSize int) iter.Seq2[Task, error] {
	return func(yield func(Task, error) bool) {
		for offset := 0; ; {
			res, err := c.List(ctx, ListOptions{Status: status, Limit: pageSize, Offset: offset})
			if err != nil {
				yield(Task{}, err)
				return
			}
			for _, t := range res.Items {
				if !yield(t, nil) {
					return
				}
			}
			offset += len(res.Items)
			if len(res.Items) == 0 || offset >= res.Total {
				return
			}
		}
	}
}
