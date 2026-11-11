// Package duedate is exercise 1: calendar-date arithmetic for Task Tracker due dates.
package duedate

import (
	"errors"
	"time"
)

// ErrInvalidDate is returned for anything that is not a real YYYY-MM-DD date.
var ErrInvalidDate = errors.New("due date must be a valid YYYY-MM-DD date")

// Parse reads a strict YYYY-MM-DD date and returns midnight UTC of that day.
func Parse(s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, ErrInvalidDate
	}
	return t, nil
}

// DaysUntil returns the number of calendar days from the UTC date of now to the due date.
// It is negative once the due date has passed and 0 on the due date itself.
func DaysUntil(now, due time.Time) int {
	today := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return int(due.Sub(today).Hours() / 24)
}

// IsOverdue reports whether a task with the given status and optional due date is overdue.
func IsOverdue(now time.Time, status, due string) (bool, error) {
	if due == "" {
		return false, nil
	}
	d, err := Parse(due)
	if err != nil {
		return false, err
	}
	return status != "done" && DaysUntil(now, d) < 0, nil
}

// FormatTimestamp renders t as the RFC 3339 UTC string used on the wire.
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
