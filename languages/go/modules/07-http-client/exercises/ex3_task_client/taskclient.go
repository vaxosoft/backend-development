// Package taskclient is exercise 3: a typed client for the Task Tracker API.
package taskclient

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
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
	panic("TODO")
}

// Every request carries "Authorization: Bearer <token>", "Accept: application/json" and
// "User-Agent: task-client/1.0"; requests with a body also carry "Content-Type: application/json".

// Create sends POST /tasks and returns the created task (201).
func (c *Client) Create(ctx context.Context, nt NewTask) (Task, error) {
	panic("TODO")
}

// Get sends GET /tasks/{id}.
func (c *Client) Get(ctx context.Context, id int) (Task, error) {
	panic("TODO")
}

// SetStatus sends PATCH /tasks/{id} with the body {"status": status}.
func (c *Client) SetStatus(ctx context.Context, id int, status string) (Task, error) {
	panic("TODO")
}

// Delete sends DELETE /tasks/{id}; success is 204 with no body.
func (c *Client) Delete(ctx context.Context, id int) error {
	panic("TODO")
}

// List sends GET /tasks with the options encoded as a query string (values must be URL-escaped).
func (c *Client) List(ctx context.Context, o ListOptions) (ListResult, error) {
	panic("TODO")
}

// All iterates over every task with the given status ("" = any), requesting pages of pageSize
// until offset >= total. It stops at the first error (yielding it) and stops fetching when
// the consumer breaks out of the loop.
func (c *Client) All(ctx context.Context, status string, pageSize int) iter.Seq2[Task, error] {
	panic("TODO")
}
