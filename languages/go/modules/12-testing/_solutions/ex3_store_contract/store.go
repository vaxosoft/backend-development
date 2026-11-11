// Package taskstore is exercise 3: an in-memory store that satisfies a contract test suite, plus a
// fault-injecting wrapper for testing error paths.
//
// Run the tests: go test -race ./modules/12-testing/exercises/ex3_store_contract/
package taskstore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	// ErrNotFound is returned when a task id does not exist.
	ErrNotFound = errors.New("task not found")
	// ErrInvalid is returned for a task that breaks the rules below.
	ErrInvalid = errors.New("invalid task")
)

// Task is a Task Tracker task. Valid statuses: todo, in_progress, done.
type Task struct {
	ID     int
	Title  string
	Status string
	Tags   []string
}

// Store is the persistence boundary. Handlers and services depend on it, tests swap implementations.
type Store interface {
	// Create validates and stores t: the title is trimmed and must have 1-200 characters, an empty
	// Status becomes "todo", any other unknown Status is ErrInvalid. Ids start at 1, increase by
	// one per stored task and are never reused. A rejected Create consumes no id.
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, id int) (Task, error)
	// Update replaces the task with t.ID, validating like Create. Unknown id: ErrNotFound.
	Update(ctx context.Context, t Task) (Task, error)
	// List returns the tasks with the given status (all when status is empty) ordered by id.
	// The result is never nil.
	List(ctx context.Context, status string) ([]Task, error)
	Delete(ctx context.Context, id int) error
}

// MemStore is a Store backed by a map. It must be safe for concurrent use, and must never share
// memory (the Tags slice!) with its callers: not on the way in, not on the way out.
type MemStore struct {
	mu     sync.Mutex
	nextID int
	tasks  map[int]Task
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{tasks: map[int]Task{}}
}

func normalize(t Task) (Task, error) {
	t.Title = strings.TrimSpace(t.Title)
	if n := utf8.RuneCountInString(t.Title); n < 1 || n > 200 {
		return Task{}, fmt.Errorf("%w: title must be 1-200 characters", ErrInvalid)
	}
	switch t.Status {
	case "":
		t.Status = "todo"
	case "todo", "in_progress", "done":
	default:
		return Task{}, fmt.Errorf("%w: unknown status %q", ErrInvalid, t.Status)
	}
	t.Tags = slices.Clone(t.Tags)
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return t, nil
}

func copyOf(t Task) Task {
	t.Tags = slices.Clone(t.Tags)
	return t
}

func (s *MemStore) Create(_ context.Context, t Task) (Task, error) {
	t, err := normalize(t)
	if err != nil {
		return Task{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	t.ID = s.nextID
	s.tasks[t.ID] = t
	return copyOf(t), nil
}

func (s *MemStore) Get(_ context.Context, id int) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return copyOf(t), nil
}

func (s *MemStore) Update(_ context.Context, t Task) (Task, error) {
	t, err := normalize(t)
	if err != nil {
		return Task{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[t.ID]; !ok {
		return Task{}, ErrNotFound
	}
	s.tasks[t.ID] = t
	return copyOf(t), nil
}

func (s *MemStore) List(_ context.Context, status string) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Task{}
	for id := 1; id <= s.nextID; id++ {
		if t, ok := s.tasks[id]; ok && (status == "" || t.Status == status) {
			out = append(out, copyOf(t))
		}
	}
	return out, nil
}

func (s *MemStore) Delete(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}

// Faulty wraps a Store and injects a failure: the first okCalls calls are passed through, every
// later call returns err without reaching the inner store. It must be safe for concurrent use.
type Faulty struct {
	inner   Store
	okCalls int
	err     error

	mu    sync.Mutex
	calls int
}

// NewFaulty wraps inner. Use a large okCalls for a wrapper that (for now) never fails.
func NewFaulty(inner Store, okCalls int, err error) *Faulty {
	return &Faulty{inner: inner, okCalls: okCalls, err: err}
}

// Calls reports how many store calls were attempted, including the failed ones.
func (f *Faulty) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fail counts the call and returns the injected error once the budget of good calls is used up.
func (f *Faulty) fail() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls > f.okCalls {
		return f.err
	}
	return nil
}

func (f *Faulty) Create(ctx context.Context, t Task) (Task, error) {
	if err := f.fail(); err != nil {
		return Task{}, err
	}
	return f.inner.Create(ctx, t)
}

func (f *Faulty) Get(ctx context.Context, id int) (Task, error) {
	if err := f.fail(); err != nil {
		return Task{}, err
	}
	return f.inner.Get(ctx, id)
}

func (f *Faulty) Update(ctx context.Context, t Task) (Task, error) {
	if err := f.fail(); err != nil {
		return Task{}, err
	}
	return f.inner.Update(ctx, t)
}

func (f *Faulty) List(ctx context.Context, status string) ([]Task, error) {
	if err := f.fail(); err != nil {
		return nil, err
	}
	return f.inner.List(ctx, status)
}

func (f *Faulty) Delete(ctx context.Context, id int) error {
	if err := f.fail(); err != nil {
		return err
	}
	return f.inner.Delete(ctx, id)
}

// DeleteDone removes every task with status "done" and reports how many were removed. On the
// first store error it stops and returns the count so far with an error that wraps the cause
// (errors.Is must still find it) and says which task failed.
func DeleteDone(ctx context.Context, s Store) (deleted int, err error) {
	done, err := s.List(ctx, "done")
	if err != nil {
		return 0, fmt.Errorf("list done tasks: %w", err)
	}
	for _, t := range done {
		if err := s.Delete(ctx, t.ID); err != nil {
			return deleted, fmt.Errorf("delete task %d: %w", t.ID, err)
		}
		deleted++
	}
	return deleted, nil
}
