// Package datetype is exercise 2: custom JSON for a calendar date and an enum.
package datetype

import (
	"time"
)

// Date is a calendar date that appears in JSON as "YYYY-MM-DD".
type Date struct{ time.Time }

// MarshalJSON encodes d as a JSON string such as "2030-01-31".
func (d Date) MarshalJSON() ([]byte, error) {
	panic("TODO")
}

// UnmarshalJSON accepts only a JSON string holding a valid YYYY-MM-DD date.
// "2030-02-30", "31/01/2030" and non-strings must return an error.
func (d *Date) UnmarshalJSON(data []byte) error {
	panic("TODO")
}

// Status is the task status enum. It appears in JSON as "todo", "in_progress" or "done".
type Status int

const (
	Todo Status = iota
	InProgress
	Done
)

// MarshalText returns the wire name, or an error for a value outside the enum.
// Implementing TextMarshaler also makes Status usable as a JSON map key.
func (s Status) MarshalText() ([]byte, error) {
	panic("TODO")
}

// UnmarshalText parses a wire name and returns an error for anything else.
func (s *Status) UnmarshalText(b []byte) error {
	panic("TODO")
}

// Task uses the two types above. A nil DueDate is JSON null.
type Task struct {
	Title   string `json:"title"`
	Status  Status `json:"status"`
	DueDate *Date  `json:"due_date"`
}
