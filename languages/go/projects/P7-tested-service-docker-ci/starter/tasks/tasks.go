// Package tasks holds the Task model and its storage abstraction.
package tasks

import (
	"context"
	"errors"
	"sync"
)

// Task statuses.
const (
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
)

// ErrNotFound is returned by Store.Get for unknown ids.
var ErrNotFound = errors.New("task not found")

// Task is a unit of work.
type Task struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// Store persists tasks. Handlers depend on this interface, so tests can swap
// in a fake.
type Store interface {
	// Create assigns the next id (starting at 1) and stores the task.
	Create(ctx context.Context, t Task) (Task, error)
	// Get returns ErrNotFound for unknown ids.
	Get(ctx context.Context, id int) (Task, error)
	// List returns all tasks in creation order.
	List(ctx context.Context) ([]Task, error)
}

// MemoryStore is a Store that keeps tasks in memory.
type MemoryStore struct {
	mu    sync.Mutex
	tasks []Task
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// Create implements Store.
func (m *MemoryStore) Create(_ context.Context, t Task) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.ID = len(m.tasks) + 1
	m.tasks = append(m.tasks, t)
	return t, nil
}

// Get implements Store.
func (m *MemoryStore) Get(_ context.Context, id int) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id < 1 || id > len(m.tasks) {
		return Task{}, ErrNotFound
	}
	return m.tasks[id-1], nil
}

// List implements Store.
func (m *MemoryStore) List(_ context.Context) ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Task{}, m.tasks...), nil
}
