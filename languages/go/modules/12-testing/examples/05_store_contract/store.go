// Package taskstore is example 05: one contract test suite run against two implementations of the same
// interface (in-memory and SQLite), so the fast fake and the real thing cannot drift apart.
//
//	go test -v ./modules/12-testing/examples/05_store_contract/
//	go test -short ./modules/12-testing/examples/05_store_contract/   # skips the SQLite run
package taskstore

import (
	"context"
	"errors"
	"slices"
	"sync"
)

var ErrNotFound = errors.New("task not found")

type Task struct {
	ID     int
	Title  string
	Status string
	Tags   []string
}

// Store is declared where it is consumed; both implementations must behave identically.
type Store interface {
	// Create stores t, assigns the next id and defaults an empty Status to "todo".
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, id int) (Task, error)
	// List returns tasks ordered by id; an empty status means all.
	List(ctx context.Context, status string) ([]Task, error)
	Delete(ctx context.Context, id int) error
}

// MemStore keeps tasks in a map and is what handler tests usually inject.
type MemStore struct {
	mu     sync.Mutex
	nextID int
	tasks  map[int]Task
}

func NewMemStore() *MemStore { return &MemStore{tasks: map[int]Task{}} }

func (s *MemStore) Create(_ context.Context, t Task) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	t.ID = s.nextID
	if t.Status == "" {
		t.Status = "todo"
	}
	t.Tags = slices.Clone(t.Tags) // do not alias the caller's slice
	if t.Tags == nil {
		t.Tags = []string{}
	}
	s.tasks[t.ID] = t
	return clone(t), nil
}

func clone(t Task) Task {
	t.Tags = slices.Clone(t.Tags)
	return t
}

func (s *MemStore) Get(_ context.Context, id int) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return clone(t), nil
}

func (s *MemStore) List(_ context.Context, status string) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Task{}
	for id := 1; id <= s.nextID; id++ {
		if t, ok := s.tasks[id]; ok && (status == "" || t.Status == status) {
			out = append(out, clone(t))
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
