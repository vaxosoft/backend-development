// Package listing parses list-endpoint query parameters, sorts and paginates.
package listing

import (
	"net/url"
	"time"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
	DefaultSort  = "-created_at"
)

// Task is the subset of fields the list endpoint cares about.
type Task struct {
	ID        int
	Status    string
	DueDate   *string // YYYY-MM-DD, nil when unset
	CreatedAt time.Time
}

// Params are the validated query parameters of GET /tasks.
type Params struct {
	Status string
	Tag    string
	Sort   string
	Limit  int
	Offset int
}

// QueryError names the offending query parameter (the handler turns it into a 400).
type QueryError struct {
	Param string
	Msg   string
}

func (e *QueryError) Error() string { return e.Param + ": " + e.Msg }

// ParseParams validates the query string:
//
//	status  "", "todo", "in_progress" or "done"
//	tag     any string, may be empty
//	sort    "" (-> DefaultSort), "created_at", "-created_at", "due_date" or "-due_date"
//	limit   absent -> DefaultLimit; a positive integer; values above MaxLimit are CLAMPED to MaxLimit
//	offset  absent -> 0; a non-negative integer
//
// Anything else returns a *QueryError whose Param is the parameter name.
func ParseParams(v url.Values) (Params, error) {
	panic("TODO")
}

// Sort orders tasks in place, stably, according to sort (a valid value from ParseParams):
//
//	created_at / -created_at   ascending / descending by CreatedAt
//	due_date / -due_date       ascending / descending by DueDate; tasks WITHOUT a due date come last in both directions
//
// Ties are broken by ascending ID.
func Sort(tasks []Task, sort string) {
	panic("TODO")
}

// Page is the list response envelope.
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Paginate returns the window items[Offset : Offset+Limit] clipped to the slice.
// Total is len(items). Items is never nil, so it encodes as [] when the
// offset is past the end. The input slice is not modified.
func Paginate[T any](items []T, p Params) Page[T] {
	panic("TODO")
}
