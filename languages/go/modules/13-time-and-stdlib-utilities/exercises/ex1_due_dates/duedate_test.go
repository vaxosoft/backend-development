package duedate

import (
	"errors"
	"testing"
	"time"
)

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func TestParse(t *testing.T) {
	got, err := Parse("2030-01-31")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := utc(2030, 1, 31, 0, 0); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("Parse = %v, want %v in UTC", got, want)
	}
	if _, err := Parse("2028-02-29"); err != nil {
		t.Fatalf("leap day rejected: %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{
		"", "2030-02-30", "2029-02-29", "2030-1-5", "31/01/2030", " 2030-01-31",
		"2030-01-31 ", "2030-01-31T00:00:00Z", "2030-13-01", "tomorrow",
	} {
		t.Run(in, func(t *testing.T) {
			if _, err := Parse(in); !errors.Is(err, ErrInvalidDate) {
				t.Fatalf("Parse(%q) error = %v, want ErrInvalidDate", in, err)
			}
		})
	}
}

func TestDaysUntil(t *testing.T) {
	due := utc(2030, 3, 1, 0, 0)
	tests := []struct {
		name string
		now  time.Time
		want int
	}{
		{"same day early", utc(2030, 3, 1, 0, 0), 0},
		{"same day late", utc(2030, 3, 1, 23, 59), 0},
		{"one day before late", utc(2030, 2, 28, 23, 59), 1},
		{"a month before", utc(2030, 1, 30, 12, 0), 30},
		{"yesterday", utc(2030, 3, 2, 0, 1), -1},
		{"far past", utc(2031, 3, 1, 12, 0), -365},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DaysUntil(tc.now, due); got != tc.want {
				t.Fatalf("DaysUntil(%v, %v) = %d, want %d", tc.now, due, got, tc.want)
			}
		})
	}
}

func TestDaysUntilLeapYear(t *testing.T) {
	if got := DaysUntil(utc(2028, 2, 28, 12, 0), utc(2028, 3, 1, 0, 0)); got != 2 {
		t.Fatalf("DaysUntil across Feb 29 = %d, want 2", got)
	}
}

func TestDaysUntilUsesTheUTCDateOfNow(t *testing.T) {
	// 2030-01-31 23:30 at UTC-5 is already 2030-02-01 in UTC.
	late := time.Date(2030, 1, 31, 23, 30, 0, 0, time.FixedZone("UTC-5", -5*3600))
	if got := DaysUntil(late, utc(2030, 2, 1, 0, 0)); got != 0 {
		t.Fatalf("DaysUntil = %d, want 0", got)
	}
	// 2030-02-01 02:00 at UTC+4 is still 2030-01-31 in UTC.
	early := time.Date(2030, 2, 1, 2, 0, 0, 0, time.FixedZone("UTC+4", 4*3600))
	if got := DaysUntil(early, utc(2030, 2, 1, 0, 0)); got != 1 {
		t.Fatalf("DaysUntil = %d, want 1", got)
	}
}

func TestDaysUntilAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata")
	}
	// The day the clocks change is 23 hours long locally; the UTC date still moves by one.
	now := time.Date(2030, 3, 10, 12, 0, 0, 0, ny)
	if got := DaysUntil(now, utc(2030, 3, 12, 0, 0)); got != 2 {
		t.Fatalf("DaysUntil = %d, want 2", got)
	}
}

func TestIsOverdue(t *testing.T) {
	now := utc(2030, 3, 1, 15, 0)
	tests := []struct {
		name, status, due string
		want              bool
	}{
		{"no due date", "todo", "", false},
		{"yesterday", "todo", "2030-02-28", true},
		{"today is not overdue", "in_progress", "2030-03-01", false},
		{"tomorrow", "todo", "2030-03-02", false},
		{"done is never overdue", "done", "2020-01-01", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IsOverdue(now, tc.status, tc.due)
			if err != nil || got != tc.want {
				t.Fatalf("IsOverdue = %v, %v; want %v, nil", got, err, tc.want)
			}
		})
	}
	if _, err := IsOverdue(now, "todo", "2030-02-30"); !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("error = %v, want ErrInvalidDate", err)
	}
}

func TestFormatTimestamp(t *testing.T) {
	zone := time.FixedZone("UTC+4", 4*3600)
	tests := []struct {
		in   time.Time
		want string
	}{
		{utc(2030, 1, 1, 9, 0), "2030-01-01T09:00:00Z"},
		{time.Date(2030, 1, 1, 13, 0, 0, 0, zone), "2030-01-01T09:00:00Z"},
		{time.Date(2030, 1, 1, 1, 0, 0, 0, zone), "2029-12-31T21:00:00Z"},
		{time.Date(2030, 1, 1, 9, 0, 0, 123456789, time.UTC), "2030-01-01T09:00:00Z"},
	}
	for _, tc := range tests {
		if got := FormatTimestamp(tc.in); got != tc.want {
			t.Errorf("FormatTimestamp(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
