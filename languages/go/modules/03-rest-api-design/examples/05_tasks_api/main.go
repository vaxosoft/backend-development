// Task Tracker REST API: CRUD, validation, error envelope, pagination, sorting,
// /v1 prefix and an embedded OpenAPI document.
package main

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

//go:embed openapi.json
var openAPI []byte

type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	DueDate   *string   `json:"due_date"`
	Tags      []string  `json:"tags"`
	OwnerID   *int      `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// taskInput is the body of POST and PUT: the writable fields of a task.
type taskInput struct {
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	DueDate *string  `json:"due_date"`
	Tags    []string `json:"tags"`
	OwnerID *int     `json:"owner_id"`
}

// optional distinguishes "absent" (Set=false) from "null" (Set=true, Value=nil).
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	return json.Unmarshal(b, &o.Value)
}

// taskPatch is the body of PATCH: only these fields may change.
type taskPatch struct {
	Title   *string          `json:"title"`
	Status  *string          `json:"status"`
	DueDate optional[string] `json:"due_date"`
	Tags    *[]string        `json:"tags"`
}

type app struct {
	mu    sync.Mutex
	next  int
	tasks map[int]Task
	now   func() time.Time
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/tasks", a.list)
	mux.HandleFunc("POST /v1/tasks", a.create)
	mux.HandleFunc("GET /v1/tasks/{id}", a.get)
	mux.HandleFunc("PUT /v1/tasks/{id}", a.replace)
	mux.HandleFunc("PATCH /v1/tasks/{id}", a.patch)
	mux.HandleFunc("DELETE /v1/tasks/{id}", a.remove)
	mux.HandleFunc("GET /v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openAPI)
	})

	mux.HandleFunc("/v1/tasks", methodNotAllowed("GET, POST"))
	mux.HandleFunc("/v1/tasks/{id}", methodNotAllowed("GET, PUT, PATCH, DELETE"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such route", nil)
	})
	return mux
}

func (a *app) list(w http.ResponseWriter, r *http.Request) {
	q, problem := parseQuery(r.URL.Query())
	if problem != "" {
		writeError(w, http.StatusBadRequest, "bad_request", problem, nil)
		return
	}
	a.mu.Lock()
	all := make([]Task, 0, len(a.tasks))
	for _, t := range a.tasks {
		all = append(all, t)
	}
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, page(all, q))
}

func (a *app) create(w http.ResponseWriter, r *http.Request) {
	var in taskInput
	if !decode(w, r, &in) {
		return
	}
	if fields := validate(&in); len(fields) > 0 {
		writeInvalid(w, fields)
		return
	}
	a.mu.Lock()
	a.next++
	t := Task{ID: a.next, Title: in.Title, Status: in.Status, DueDate: in.DueDate, Tags: in.Tags, OwnerID: in.OwnerID, CreatedAt: a.now()}
	a.tasks[t.ID] = t
	a.mu.Unlock()

	w.Header().Set("Location", "/v1/tasks/"+strconv.Itoa(t.ID))
	writeJSON(w, http.StatusCreated, t)
}

func (a *app) get(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ok := a.find(w, r); ok {
		writeJSON(w, http.StatusOK, t)
	}
}

func (a *app) replace(w http.ResponseWriter, r *http.Request) {
	var in taskInput
	if !decode(w, r, &in) {
		return
	}
	if fields := validate(&in); len(fields) > 0 {
		writeInvalid(w, fields)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.find(w, r)
	if !ok {
		return
	}
	t.Title, t.Status, t.DueDate, t.Tags, t.OwnerID = in.Title, in.Status, in.DueDate, in.Tags, in.OwnerID
	a.tasks[t.ID] = t
	writeJSON(w, http.StatusOK, t)
}

func (a *app) patch(w http.ResponseWriter, r *http.Request) {
	var p taskPatch
	if !decode(w, r, &p) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.find(w, r)
	if !ok {
		return
	}
	// Merge into the stored task, then validate the result as a whole.
	in := taskInput{Title: t.Title, Status: t.Status, DueDate: t.DueDate, Tags: t.Tags, OwnerID: t.OwnerID}
	if p.Title != nil {
		in.Title = *p.Title
	}
	if p.Status != nil {
		in.Status = *p.Status
	}
	if p.DueDate.Set {
		in.DueDate = p.DueDate.Value // nil clears the date
	}
	if p.Tags != nil {
		in.Tags = *p.Tags
	}
	if fields := validate(&in); len(fields) > 0 {
		writeInvalid(w, fields)
		return
	}
	t.Title, t.Status, t.DueDate, t.Tags = in.Title, in.Status, in.DueDate, in.Tags
	a.tasks[t.ID] = t
	writeJSON(w, http.StatusOK, t)
}

func (a *app) remove(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ok := a.find(w, r); ok {
		delete(a.tasks, t.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// find must be called with a.mu held. It writes the error response itself.
func (a *app) find(w http.ResponseWriter, r *http.Request) (Task, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "id must be an integer", nil)
		return Task{}, false
	}
	t, ok := a.tasks[id]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("task %d not found", id), nil)
	}
	return t, ok
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil {
		if _, extra := dec.Token(); !errors.Is(extra, io.EOF) {
			err = errors.New("trailing data")
		}
	}
	var tooBig *http.MaxBytesError
	switch {
	case err == nil:
		return true
	case errors.As(err, &tooBig):
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large", nil)
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON request body", nil)
	}
	return false
}

// validate normalises the input (trim, default status, non-nil tags) and returns field problems.
func validate(t *taskInput) map[string]string {
	f := map[string]string{}
	t.Title = strings.TrimSpace(t.Title)
	if n := utf8.RuneCountInString(t.Title); n < 1 || n > 200 {
		f["title"] = "must be 1-200 characters"
	}
	switch t.Status {
	case "":
		t.Status = "todo"
	case "todo", "in_progress", "done":
	default:
		f["status"] = "must be one of todo, in_progress, done"
	}
	if t.DueDate != nil {
		if _, err := time.Parse("2006-01-02", *t.DueDate); err != nil {
			f["due_date"] = "must be a date in YYYY-MM-DD format"
		}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	seen := map[string]bool{}
	for _, tag := range t.Tags {
		if n := len(tag); n < 1 || n > 30 || tag != strings.ToLower(tag) || seen[tag] {
			f["tags"] = "each tag must be 1-30 lowercase characters and unique"
			break
		}
		seen[tag] = true
	}
	if len(t.Tags) > 10 {
		f["tags"] = "at most 10 tags"
	}
	return f
}

type query struct {
	status, tag, sort string
	limit, offset     int
}

func parseQuery(v url.Values) (query, string) {
	q := query{status: v.Get("status"), tag: v.Get("tag"), sort: "-created_at", limit: 20}
	switch q.status {
	case "", "todo", "in_progress", "done":
	default:
		return q, "status must be todo, in_progress or done"
	}
	if s := v.Get("sort"); s != "" {
		if !slices.Contains([]string{"created_at", "-created_at", "due_date", "-due_date"}, s) {
			return q, "sort must be created_at, -created_at, due_date or -due_date"
		}
		q.sort = s
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return q, "limit must be a positive integer"
		}
		q.limit = min(n, 100)
	}
	if s := v.Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return q, "offset must be a non-negative integer"
		}
		q.offset = n
	}
	return q, ""
}

type pageBody struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func page(all []Task, q query) pageBody {
	items := []Task{}
	for _, t := range all {
		if (q.status == "" || t.Status == q.status) && (q.tag == "" || slices.Contains(t.Tags, q.tag)) {
			items = append(items, t)
		}
	}
	desc := strings.HasPrefix(q.sort, "-")
	slices.SortStableFunc(items, func(a, b Task) int {
		var c int
		if strings.HasSuffix(q.sort, "created_at") {
			c = a.CreatedAt.Compare(b.CreatedAt)
		} else {
			switch {
			case a.DueDate == nil && b.DueDate == nil:
			case a.DueDate == nil:
				return 1 // no due date: always last
			case b.DueDate == nil:
				return -1
			default:
				c = cmp.Compare(*a.DueDate, *b.DueDate)
			}
		}
		if desc {
			c = -c
		}
		return cmp.Or(c, cmp.Compare(a.ID, b.ID))
	})
	total := len(items)
	start := min(q.offset, total)
	end := min(start+q.limit, total)
	return pageBody{Items: items[start:end], Total: total, Limit: q.limit, Offset: q.offset}
}

func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not supported here", nil)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	writeJSON(w, status, map[string]errorBody{"error": {Code: code, Message: msg, Fields: fields}})
}

func writeInvalid(w http.ResponseWriter, fields map[string]string) {
	writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", fields)
}

// demoClock starts at a fixed instant and advances one minute per call so
// the documented output is reproducible. Real code would use time.Now.
func demoClock() func() time.Time {
	t := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	return func() time.Time {
		cur := t
		t = t.Add(time.Minute)
		return cur
	}
}

func main() {
	log.SetFlags(0)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	a := &app{tasks: map[int]Task{}, now: demoClock()}
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("listening on %s", addr)

	select {
	case err := <-errc:
		log.Fatal(err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	log.Print("bye")
}
