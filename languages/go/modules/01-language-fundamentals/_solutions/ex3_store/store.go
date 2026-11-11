// Package store practises value vs reference semantics, generics and iterators.
package store

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"
)

var ErrNotFound = errors.New("not found")

type Task struct {
	ID    int
	Title string
	Tags  []string
}

// Store keeps tasks in memory. It is safe for concurrent use.
//
// Tasks are values, but Tags is a slice: Store must copy it on the way in
// (Create) and on the way out (Get, All) so callers can never change stored
// data by modifying a slice they hold.
type Store struct {
	mu    sync.Mutex
	next  int
	tasks map[int]Task
}

func New() *Store {
	return &Store{next: 1, tasks: map[int]Task{}}
}

func clone(t Task) Task {
	t.Tags = slices.Clone(t.Tags)
	return t
}

// Create assigns the next id (1, 2, 3, ...), stores a copy and returns the stored task.
func (s *Store) Create(t Task) Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = s.next
	s.next++
	s.tasks[t.ID] = clone(t)
	return clone(t)
}

// Get returns a copy of the task or an error wrapping ErrNotFound.
func (s *Store) Get(id int) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("task %d: %w", id, ErrNotFound)
	}
	return clone(t), nil
}

// All returns an iterator over a snapshot of the tasks in ascending id order.
// It must stop as soon as the consumer's yield returns false.
func (s *Store) All() iter.Seq[Task] {
	s.mu.Lock()
	snapshot := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		snapshot = append(snapshot, clone(t))
	}
	s.mu.Unlock()
	slices.SortFunc(snapshot, func(a, b Task) int { return cmp.Compare(a.ID, b.ID) })
	return slices.Values(snapshot)
}

// Filter yields only the elements for which keep returns true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

// Map yields f(v) for each v in seq.
func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}
