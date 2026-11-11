// Package taskstore is exercise 3: an in-memory store that satisfies a contract test suite, plus a
// fault-injecting wrapper for testing error paths.
//
// Run the tests: go test -race ./modules/12-testing/exercises/ex3_store_contract/
package taskstore

import (
	"context"
	"errors"
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
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{}
}

func (s *MemStore) Create(ctx context.Context, t Task) (Task, error) { panic("TODO") }
func (s *MemStore) Get(ctx context.Context, id int) (Task, error)    { panic("TODO") }
func (s *MemStore) Update(ctx context.Context, t Task) (Task, error) { panic("TODO") }
func (s *MemStore) List(ctx context.Context, status string) ([]Task, error) {
	panic("TODO")
}
func (s *MemStore) Delete(ctx context.Context, id int) error { panic("TODO") }

// Faulty wraps a Store and injects a failure: the first okCalls calls are passed through, every
// later call returns err without reaching the inner store. It must be safe for concurrent use.
type Faulty struct {
	Store // the inner store; methods you do not need to intercept would be promoted from it
}

// NewFaulty wraps inner. Use a large okCalls for a wrapper that (for now) never fails.
func NewFaulty(inner Store, okCalls int, err error) *Faulty {
	return &Faulty{Store: inner}
}

// Calls reports how many store calls were attempted, including the failed ones.
func (f *Faulty) Calls() int { panic("TODO") }

func (f *Faulty) Create(ctx context.Context, t Task) (Task, error) { panic("TODO") }
func (f *Faulty) Get(ctx context.Context, id int) (Task, error)    { panic("TODO") }
func (f *Faulty) Update(ctx context.Context, t Task) (Task, error) { panic("TODO") }
func (f *Faulty) List(ctx context.Context, status string) ([]Task, error) {
	panic("TODO")
}
func (f *Faulty) Delete(ctx context.Context, id int) error { panic("TODO") }

// DeleteDone removes every task with status "done" and reports how many were removed. On the
// first store error it stops and returns the count so far with an error that wraps the cause
// (errors.Is must still find it) and says which task failed.
func DeleteDone(ctx context.Context, s Store) (deleted int, err error) {
	panic("TODO")
}
