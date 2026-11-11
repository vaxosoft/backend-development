// Package task validates tasks and reports problems with custom errors.
package task

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

var (
	// ErrValidation is matched (errors.Is) by every *FieldErrors.
	ErrValidation = errors.New("validation failed")
	// ErrInvalidStatus is wrapped by ParseStatus.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrNotFound is wrapped by Repo.Get.
	ErrNotFound = errors.New("not found")
)

// FieldErrors maps a field name to a human readable problem.
type FieldErrors struct {
	Fields map[string]string
}

// Error returns "validation failed: f1: msg1; f2: msg2" with fields sorted by name.
func (e *FieldErrors) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, name := range slices.Sorted(maps.Keys(e.Fields)) {
		parts = append(parts, name+": "+e.Fields[name])
	}
	return ErrValidation.Error() + ": " + strings.Join(parts, "; ")
}

// Is reports whether target is ErrValidation.
func (e *FieldErrors) Is(target error) bool { return target == ErrValidation }

type Task struct {
	ID     int
	Title  string
	Status string
	Tags   []string
}

// ParseStatus accepts "todo", "in_progress" and "done". Anything else returns
// an error wrapping ErrInvalidStatus whose message contains the quoted input.
func ParseStatus(s string) (string, error) {
	switch s {
	case "todo", "in_progress", "done":
		return s, nil
	}
	return "", fmt.Errorf("%q: %w", s, ErrInvalidStatus)
}

// Validate checks the task and returns nil when it is valid. Otherwise it
// returns a *FieldErrors with an entry for each broken rule:
//
//	title:  trimmed length must be 1-200       -> "must be 1-200 characters"
//	status: must parse with ParseStatus        -> "must be todo, in_progress or done"
//	tags:   at most 10 entries                 -> "at most 10 tags"
//
// An empty Status is allowed (it means the default).
//
// Beware: the returned error must be a true nil interface when the task is valid.
func Validate(t Task) error {
	fields := map[string]string{}
	if n := len(strings.TrimSpace(t.Title)); n < 1 || n > 200 {
		fields["title"] = "must be 1-200 characters"
	}
	if t.Status != "" {
		if _, err := ParseStatus(t.Status); err != nil {
			fields["status"] = "must be todo, in_progress or done"
		}
	}
	if len(t.Tags) > 10 {
		fields["tags"] = "at most 10 tags"
	}
	if len(fields) == 0 {
		return nil
	}
	return &FieldErrors{Fields: fields}
}

// Repo is a tiny in-memory repository.
type Repo struct {
	tasks map[int]Task
}

func NewRepo(tasks ...Task) *Repo {
	r := &Repo{tasks: map[int]Task{}}
	for _, t := range tasks {
		r.tasks[t.ID] = t
	}
	return r
}

// Get returns the task, or an error that wraps ErrNotFound and mentions the id.
func (r *Repo) Get(id int) (Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("task %d: %w", id, ErrNotFound)
	}
	return t, nil
}
