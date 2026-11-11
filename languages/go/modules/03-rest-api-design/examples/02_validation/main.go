// Request validation: strict decoding (400/413) separated from business rules (422).
// Runs in-process with httptest, so no port is needed.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"unicode/utf8"
)

type NewTask struct {
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	DueDate *string  `json:"due_date"`
	Tags    []string `json:"tags"`
}

// decode reads one JSON value and nothing else.
func decode(w http.ResponseWriter, r *http.Request, dst any) (status int, code, msg string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil {
		if _, extra := dec.Token(); !errors.Is(extra, io.EOF) {
			err = errors.New("trailing data after JSON value")
		}
	}
	var tooBig *http.MaxBytesError // typed errors are matched; message texts are not parsed
	switch {
	case err == nil:
		return 0, "", "", true
	case errors.As(err, &tooBig):
		return http.StatusRequestEntityTooLarge, "payload_too_large", fmt.Sprintf("body exceeds %d bytes", tooBig.Limit), false
	default:
		return http.StatusBadRequest, "bad_request", "invalid JSON request body", false
	}
}

// validate returns field -> problem; an empty map means valid.
func validate(t *NewTask) map[string]string {
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
	if len(t.Tags) > 10 {
		f["tags"] = "at most 10 tags"
	} else {
		seen := map[string]bool{}
		for _, tag := range t.Tags {
			if n := len(tag); n < 1 || n > 30 || tag != strings.ToLower(tag) || seen[tag] {
				f["tags"] = "each tag must be 1-30 lowercase characters and unique"
				break
			}
			seen[tag] = true
		}
	}
	return f
}

func createTask(w http.ResponseWriter, r *http.Request) {
	var in NewTask
	if status, code, msg, ok := decode(w, r, &in); !ok {
		reply(w, status, code, msg, nil)
		return
	}
	if fields := validate(&in); len(fields) > 0 {
		reply(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", fields)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "{\"title\":%q,\"status\":%q}\n", in.Title, in.Status)
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func reply(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]errorBody{"error": {code, msg, fields}})
}

func main() {
	cases := []struct{ name, body string }{
		{"valid", `{"title":"  Write docs  ","tags":["docs","p1"],"due_date":"2030-01-31"}`},
		{"malformed JSON", `{"title":`},
		{"wrong type", `{"title":42}`},
		{"unknown field", `{"title":"x","owner":"me"}`},
		{"trailing data", `{"title":"x"} {"title":"y"}`},
		{"too large", `{"title":"` + strings.Repeat("a", 600) + `"}`},
		{"empty title", `{"title":"   "}`},
		{"several problems", `{"title":"","status":"blocked","due_date":"2030-02-30","tags":["Docs","a","a"]}`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		createTask(rec, httptest.NewRequest("POST", "/tasks", strings.NewReader(c.body)))
		fmt.Printf("%-16s %d %s", c.name, rec.Code, rec.Body.String())
	}
}
