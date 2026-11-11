// Task Tracker: due-date reminders from an injected clock, run by an in-process ticker job.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Clock lets tests and demos control "now"; production code passes ClockFunc(time.Now).
type Clock interface{ Now() time.Time }

type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

// stepClock moves forward by step each time it is read.
type stepClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

type Task struct {
	ID      int
	Title   string
	Status  string
	DueDate string // YYYY-MM-DD, empty when unset
}

// Urgency is derived from calendar dates (UTC), never from a 24h division of instants.
func Urgency(now time.Time, t Task) (string, error) {
	if t.Status == "done" || t.DueDate == "" {
		return "", nil
	}
	due, err := time.Parse(time.DateOnly, t.DueDate)
	if err != nil {
		return "", fmt.Errorf("task %d: %w", t.ID, err)
	}
	today := now.UTC().Truncate(24 * time.Hour)
	switch days := int(due.Sub(today).Hours() / 24); {
	case days < 0:
		return fmt.Sprintf("overdue by %d day(s)", -days), nil
	case days == 0:
		return "due today", nil
	case days <= 2:
		return fmt.Sprintf("due in %d day(s)", days), nil
	}
	return "", nil
}

// Every runs fn at each tick until ctx ends, then returns. It never runs fn concurrently with itself.
func Every(ctx context.Context, interval time.Duration, fn func(now time.Time)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			fn(now)
		}
	}
}

func main() {
	tasks := []Task{
		{1, "Write docs", "todo", "2030-01-30"},
		{2, "Ship it", "in_progress", "2030-02-01"},
		{3, "Plan", "todo", "2030-01-31"},
		{4, "Old news", "done", "2020-01-01"},
		{5, "Someday", "todo", ""},
		{6, "Broken", "todo", "31/01/2030"},
	}
	var clock Clock = &stepClock{now: time.Date(2030, 1, 31, 23, 0, 0, 0, time.UTC), step: 24 * time.Hour}

	ctx, cancel := context.WithCancel(context.Background())
	runs := 0
	Every(ctx, 2*time.Millisecond, func(time.Time) {
		runs++
		now := clock.Now()
		fmt.Println("run", runs, now.Format(time.RFC3339))
		for _, t := range tasks {
			msg, err := Urgency(now, t)
			switch {
			case err != nil:
				fmt.Println("  skip:", err)
			case msg != "":
				fmt.Printf("  #%d %s: %s\n", t.ID, t.Title, msg)
			}
		}
		if runs == 2 {
			cancel()
		}
	})
	fmt.Println("scheduler stopped:", ctx.Err())
}
