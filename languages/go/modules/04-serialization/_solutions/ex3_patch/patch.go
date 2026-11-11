// Package patch is exercise 3: strict PATCH decoding with absent / null / value semantics.
package patch

import (
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrTrailingData is returned by Decode when the body has anything after the first JSON value.
var ErrTrailingData = errors.New("trailing data after JSON value")

// Optional records whether a JSON key was absent, null, or carried a value.
type Optional[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON is only invoked by encoding/json when the key is present (including for null).
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// Patch is the decoded body of PATCH /tasks/{id}. Only these four fields are writable.
type Patch struct {
	Title   Optional[string]   `json:"title"`
	Status  Optional[string]   `json:"status"`
	DueDate Optional[string]   `json:"due_date"`
	Tags    Optional[[]string] `json:"tags"`
}

// Task is the stored task.
type Task struct {
	ID      int
	Title   string
	Status  string
	DueDate *string
	Tags    []string
}

// Decode reads exactly one JSON object from r into a Patch.
// Unknown fields (for example id or created_at), an empty body, malformed JSON
// and trailing data (ErrTrailingData) are errors.
func Decode(r io.Reader) (Patch, error) {
	var p Patch
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Patch{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Patch{}, ErrTrailingData
	}
	return p, nil
}

// Apply returns t with the patch applied, or t unchanged plus a field -> message map if anything is invalid.
//
//	title:    not null; trimmed; 1-200 characters
//	status:   not null; todo | in_progress | done
//	due_date: null clears it; otherwise YYYY-MM-DD
//	tags:     null clears them; otherwise at most 10, each 1-30 chars, lowercase, no duplicates
//
// Absent keys leave the field alone. The returned Task must not share the Tags slice with the input patch.
func (p Patch) Apply(t Task) (Task, map[string]string) {
	out := t
	fields := map[string]string{}

	if p.Title.Set {
		title := strings.TrimSpace(p.Title.Value)
		if n := utf8.RuneCountInString(title); p.Title.Null || n < 1 || n > 200 {
			fields["title"] = "must be 1-200 characters"
		} else {
			out.Title = title
		}
	}
	if p.Status.Set {
		switch {
		case p.Status.Null:
			fields["status"] = "must not be null"
		case !slices.Contains([]string{"todo", "in_progress", "done"}, p.Status.Value):
			fields["status"] = "must be one of todo, in_progress, done"
		default:
			out.Status = p.Status.Value
		}
	}
	if p.DueDate.Set {
		if p.DueDate.Null {
			out.DueDate = nil
		} else if _, err := time.Parse("2006-01-02", p.DueDate.Value); err != nil {
			fields["due_date"] = "must be YYYY-MM-DD"
		} else {
			d := p.DueDate.Value
			out.DueDate = &d
		}
	}
	if p.Tags.Set {
		if p.Tags.Null {
			out.Tags = []string{}
		} else if msg := checkTags(p.Tags.Value); msg != "" {
			fields["tags"] = msg
		} else {
			out.Tags = slices.Clone(p.Tags.Value)
		}
	}

	if len(fields) > 0 {
		return t, fields
	}
	return out, nil
}

func checkTags(tags []string) string {
	if len(tags) > 10 {
		return "at most 10 tags"
	}
	seen := map[string]bool{}
	for _, tag := range tags {
		if n := utf8.RuneCountInString(tag); n < 1 || n > 30 {
			return "each tag must be 1-30 characters"
		}
		if tag != strings.ToLower(tag) {
			return "tags must be lowercase"
		}
		if seen[tag] {
			return "tags must be unique"
		}
		seen[tag] = true
	}
	return ""
}
