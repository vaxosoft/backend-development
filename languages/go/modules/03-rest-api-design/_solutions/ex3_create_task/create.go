// Package tasks is a POST /tasks endpoint: types, store and response helpers are
// provided; the handler and validation are yours.
package tasks

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Input is the writable part of a task (the body of POST /tasks).
type Input struct {
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	DueDate *string  `json:"due_date"`
	Tags    []string `json:"tags"`
	OwnerID *int     `json:"owner_id"`
}

// Task is what the API returns.
type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	DueDate   *string   `json:"due_date"`
	Tags      []string  `json:"tags"`
	OwnerID   *int      `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Store persists tasks. Create assigns the ID.
type Store interface {
	Create(t Task) (Task, error)
}

// MemStore is an in-memory Store.
type MemStore struct {
	mu    sync.Mutex
	Tasks []Task
}

func (s *MemStore) Create(t Task) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = len(s.Tasks) + 1
	s.Tasks = append(s.Tasks, t)
	return t, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	body := map[string]any{"code": code, "message": message}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	writeJSON(w, status, map[string]any{"error": body})
}

// NewCreateHandler returns the handler for POST /tasks.
//
// Request body: a single JSON object with the fields of Input. Behaviour:
//
//   - body larger than 64 KiB                 -> 413 code "payload_too_large"
//   - malformed JSON, empty body, wrong field types, unknown fields
//     (such as "id"), or anything after the JSON object
//     -> 400 code "bad_request"
//   - well-formed but invalid data            -> 422 code "validation_failed"
//     with "fields" naming EVERY broken field (see validate)
//   - store failure                           -> 500 code "internal", message
//     "internal server error" (never the store's error text)
//   - success                                 -> 201, header "Location: /tasks/<id>",
//     body = the stored Task as JSON, CreatedAt = now()
//
// The task handed to the store is normalised by validate: trimmed title,
// status defaulted to "todo", Tags never nil.
func NewCreateHandler(store Store, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var in Input
		if err := decodeStrict(r.Body, &in); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large", nil)
				return
			}
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON request body", nil)
			return
		}
		if fields := validate(&in); len(fields) > 0 {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", fields)
			return
		}
		created, err := store.Create(Task{
			Title: in.Title, Status: in.Status, DueDate: in.DueDate,
			Tags: in.Tags, OwnerID: in.OwnerID, CreatedAt: now().UTC(),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
			return
		}
		w.Header().Set("Location", "/tasks/"+strconv.Itoa(created.ID))
		writeJSON(w, http.StatusCreated, created)
	})
}

// validate normalises in (see above) and returns a map of field name to
// problem. An empty map means the input is valid. Rules and exact messages:
//
//	title     trimmed, 1-200 characters (runes)  "must be 1-200 characters"
//	status    "" (becomes "todo"), "todo", "in_progress", "done"
//	                                             "must be one of todo, in_progress, done"
//	due_date  nil or a real calendar date YYYY-MM-DD
//	                                             "must be a date in YYYY-MM-DD format"
//	tags      at most 10 entries               "at most 10 tags"
//	          each 1-30 bytes, lowercase, unique "each tag must be 1-30 lowercase characters and unique"
//	owner_id  nil or a positive integer          "must be a positive integer"
//
// When there are more than 10 tags the message is "at most 10 tags", even if
// individual tags are also invalid.
func validate(in *Input) map[string]string {
	f := map[string]string{}

	in.Title = strings.TrimSpace(in.Title)
	if n := utf8.RuneCountInString(in.Title); n < 1 || n > 200 {
		f["title"] = "must be 1-200 characters"
	}

	switch in.Status {
	case "":
		in.Status = "todo"
	case "todo", "in_progress", "done":
	default:
		f["status"] = "must be one of todo, in_progress, done"
	}

	if in.DueDate != nil {
		if _, err := time.Parse("2006-01-02", *in.DueDate); err != nil {
			f["due_date"] = "must be a date in YYYY-MM-DD format"
		}
	}

	if in.Tags == nil {
		in.Tags = []string{}
	}
	seen := map[string]bool{}
	for _, tag := range in.Tags {
		if n := len(tag); n < 1 || n > 30 || tag != strings.ToLower(tag) || seen[tag] {
			f["tags"] = "each tag must be 1-30 lowercase characters and unique"
			break
		}
		seen[tag] = true
	}
	if len(in.Tags) > 10 {
		f["tags"] = "at most 10 tags"
	}

	if in.OwnerID != nil && *in.OwnerID < 1 {
		f["owner_id"] = "must be a positive integer"
	}
	return f
}

// decodeStrict decodes exactly one JSON value and rejects unknown fields and trailing data.
func decodeStrict(r io.Reader, dst any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON object")
	}
	return nil
}
