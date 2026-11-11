// Package datetype is exercise 2: custom JSON for a calendar date and an enum.
package datetype

import (
	"encoding/json"
	"fmt"
	"time"
)

const layout = "2006-01-02"

// Date is a calendar date that appears in JSON as "YYYY-MM-DD".
type Date struct{ time.Time }

// MarshalJSON encodes d as a JSON string such as "2030-01-31".
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(layout))
}

// UnmarshalJSON accepts only a JSON string holding a valid YYYY-MM-DD date.
// "2030-02-30", "31/01/2030" and non-strings must return an error.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("date must be a string: %w", err)
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	d.Time = t
	return nil
}

// Status is the task status enum. It appears in JSON as "todo", "in_progress" or "done".
type Status int

const (
	Todo Status = iota
	InProgress
	Done
)

var names = [...]string{"todo", "in_progress", "done"}

// MarshalText returns the wire name, or an error for a value outside the enum.
// Implementing TextMarshaler also makes Status usable as a JSON map key.
func (s Status) MarshalText() ([]byte, error) {
	if s < 0 || int(s) >= len(names) {
		return nil, fmt.Errorf("invalid status %d", int(s))
	}
	return []byte(names[s]), nil
}

// UnmarshalText parses a wire name and returns an error for anything else.
func (s *Status) UnmarshalText(b []byte) error {
	for i, n := range names {
		if n == string(b) {
			*s = Status(i)
			return nil
		}
	}
	return fmt.Errorf("unknown status %q", b)
}

// Task uses the two types above. A nil DueDate is JSON null.
type Task struct {
	Title   string `json:"title"`
	Status  Status `json:"status"`
	DueDate *Date  `json:"due_date"`
}
