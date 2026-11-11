// Package reminders is example 03: dependencies behind small interfaces, replaced by hand-written
// fakes (a store, a notifier that records, a clock you can move, an in-memory file system).
//
//	go test -v ./modules/12-testing/examples/03_fakes_and_clocks/
package reminders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"
)

type Task struct {
	ID    int
	Title string
	Owner string
	Due   time.Time // zero means no due date
	Done  bool
}

// The interfaces are declared by the consumer and are as small as the consumer needs.
type (
	Store interface {
		List(ctx context.Context) ([]Task, error)
	}
	Notifier interface {
		Notify(ctx context.Context, owner, message string) error
	}
	Clock interface {
		Now() time.Time
	}
)

type Service struct {
	Store    Store
	Notifier Notifier
	Clock    Clock
}

// SendOverdue notifies the owner of every open task whose due date has passed. One failing
// notification does not stop the others; their errors are joined.
func (s *Service) SendOverdue(ctx context.Context) (sent int, err error) {
	tasks, err := s.Store.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list tasks: %w", err)
	}
	now := s.Clock.Now()
	var errs []error
	for _, t := range tasks {
		if t.Done || t.Due.IsZero() || !t.Due.Before(now) {
			continue
		}
		msg := fmt.Sprintf("task %q was due on %s", t.Title, t.Due.Format("2006-01-02"))
		if err := s.Notifier.Notify(ctx, t.Owner, msg); err != nil {
			errs = append(errs, fmt.Errorf("notify %s: %w", t.Owner, err))
			continue
		}
		sent++
	}
	return sent, errors.Join(errs...)
}

// LoadSeed reads tasks from a JSON file; taking an fs.FS lets tests supply an in-memory one.
func LoadSeed(fsys fs.FS, name string) ([]Task, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read seed: %w", err)
	}
	var raw []struct {
		Title string `json:"title"`
		Owner string `json:"owner"`
		Due   string `json:"due_date"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse seed %s: %w", name, err)
	}
	tasks := make([]Task, len(raw))
	for i, r := range raw {
		tasks[i] = Task{ID: i + 1, Title: r.Title, Owner: r.Owner}
		if r.Due != "" {
			due, err := time.Parse("2006-01-02", r.Due)
			if err != nil {
				return nil, fmt.Errorf("seed task %d: %w", i+1, err)
			}
			tasks[i].Due = due
		}
	}
	return tasks, nil
}
