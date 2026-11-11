// Task Tracker: strict request decoding, validation and the error envelope.
// Uses httptest.NewRecorder, so nothing listens on a port.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Task struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	DueDate   *string  `json:"due_date"`
	Tags      []string `json:"tags"`
	OwnerID   *int     `json:"owner_id"`
	CreatedAt string   `json:"created_at"`
}

type createRequest struct {
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	DueDate *string  `json:"due_date"`
	Tags    []string `json:"tags"`
	OwnerID *int     `json:"owner_id"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, e apiError) {
	writeJSON(w, status, map[string]apiError{"error": e})
}

// decodeJSON reads exactly one JSON value, rejecting unknown fields and trailing data.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single JSON value")
	}
	return nil
}

func decodeProblem(err error) (int, apiError) {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		tooBig    *http.MaxBytesError
	)
	switch {
	case errors.As(err, &tooBig):
		return http.StatusRequestEntityTooLarge, apiError{Code: "payload_too_large", Message: "request body too large"}
	case errors.As(err, &syntaxErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return http.StatusBadRequest, apiError{Code: "bad_request", Message: "malformed JSON"}
	case errors.As(err, &typeErr):
		return http.StatusBadRequest, apiError{Code: "bad_request", Message: "field " + typeErr.Field + " must be a " + typeErr.Type.String()}
	default:
		return http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()}
	}
}

func (c *createRequest) validate() map[string]string {
	f := map[string]string{}
	c.Title = strings.TrimSpace(c.Title)
	if n := utf8.RuneCountInString(c.Title); n < 1 || n > 200 {
		f["title"] = "must be 1-200 characters"
	}
	switch c.Status {
	case "":
		c.Status = "todo"
	case "todo", "in_progress", "done":
	default:
		f["status"] = "must be one of todo, in_progress, done"
	}
	if c.DueDate != nil {
		if _, err := time.Parse("2006-01-02", *c.DueDate); err != nil {
			f["due_date"] = "must be YYYY-MM-DD"
		}
	}
	if len(c.Tags) > 10 {
		f["tags"] = "at most 10 tags"
	}
	return f
}

func createTask(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decodeJSON(w, r, &req, 256); err != nil {
		status, e := decodeProblem(err)
		writeError(w, status, e)
		return
	}
	if fields := req.validate(); len(fields) > 0 {
		writeError(w, http.StatusUnprocessableEntity, apiError{Code: "validation_failed", Message: "invalid task", Fields: fields})
		return
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	t := Task{ID: 1, Title: req.Title, Status: req.Status, DueDate: req.DueDate, Tags: req.Tags,
		OwnerID: req.OwnerID, CreatedAt: "2030-01-01T09:00:00Z"}
	w.Header().Set("Location", "/tasks/"+strconv.Itoa(t.ID))
	writeJSON(w, http.StatusCreated, t)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", createTask)

	cases := []struct{ name, body string }{
		{"valid", `{"title":"  Write docs ","tags":["docs"],"due_date":"2030-01-31"}`},
		{"invalid fields", `{"title":"","status":"later","due_date":"tomorrow"}`},
		{"malformed", `{"title":`},
		{"empty body", ``},
		{"unknown field", `{"title":"x","colour":"red"}`},
		{"wrong type", `{"title":42}`},
		{"trailing data", `{"title":"x"} {"title":"y"}`},
		{"too large", `{"title":"` + strings.Repeat("a", 300) + `"}`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(c.body))
		mux.ServeHTTP(rec, req)
		fmt.Printf("%-15s %d %s", c.name, rec.Code, rec.Body.String())
	}
}
