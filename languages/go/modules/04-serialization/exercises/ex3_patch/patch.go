// Package patch is exercise 3: strict PATCH decoding with absent / null / value semantics.
package patch

import (
	"errors"
	"io"
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
	panic("TODO")
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
	panic("TODO")
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
	panic("TODO")
}
