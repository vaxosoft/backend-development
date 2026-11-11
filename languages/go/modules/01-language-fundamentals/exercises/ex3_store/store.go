// Package store practises value vs reference semantics, generics and iterators.
package store

import (
	"errors"
	"iter"
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

// Create assigns the next id (1, 2, 3, ...), stores a copy and returns the stored task.
func (s *Store) Create(t Task) Task {
	panic("TODO")
}

// Get returns a copy of the task or an error wrapping ErrNotFound.
func (s *Store) Get(id int) (Task, error) {
	panic("TODO")
}

// All returns an iterator over a snapshot of the tasks in ascending id order.
// It must stop as soon as the consumer's yield returns false.
func (s *Store) All() iter.Seq[Task] {
	panic("TODO")
}

// Filter yields only the elements for which keep returns true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	panic("TODO")
}

// Map yields f(v) for each v in seq.
func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	panic("TODO")
}
