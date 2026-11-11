// Package duedate is exercise 1: calendar-date arithmetic for Task Tracker due dates.
//
// Run the tests: go test ./modules/13-time-and-stdlib-utilities/exercises/ex1_due_dates/
package duedate

import (
	"errors"
	"time"
)

// ErrInvalidDate is returned for anything that is not a real YYYY-MM-DD date.
var ErrInvalidDate = errors.New("due date must be a valid YYYY-MM-DD date")

// Parse reads a strict YYYY-MM-DD date and returns midnight UTC of that day.
// Anything else (other layouts, missing zero padding, surrounding spaces,
// 2030-02-30, a full timestamp) is ErrInvalidDate.
func Parse(s string) (time.Time, error) {
	return time.Time{}, nil // TODO
}

// DaysUntil returns the number of calendar days from the UTC date of now to the due date.
// It is negative once the due date has passed and 0 on the due date itself;
// the time of day and the zone of now only matter through its UTC date.
func DaysUntil(now, due time.Time) int {
	return 0 // TODO
}

// IsOverdue reports whether a task with the given status and optional due date
// ("" = none) is overdue. A done task is never overdue, nor is one due today.
// An invalid due date is reported as ErrInvalidDate.
func IsOverdue(now time.Time, status, due string) (bool, error) {
	return false, nil // TODO
}

// FormatTimestamp renders t as the RFC 3339 UTC string used on the wire
// (for example 2030-01-01T09:00:00Z), whatever zone t is in.
func FormatTimestamp(t time.Time) string {
	return "" // TODO
}
