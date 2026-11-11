// Task Tracker domain in plain Go: types, an interface, an in-memory store,
// validation errors, closures and iterators working together.
package main

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"
	"sync"
)

type Status string

const (
	Todo       Status = "todo"
	InProgress Status = "in_progress"
	Done       Status = "done"
)

func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case Todo, InProgress, Done:
		return st, nil
	}
	return "", fmt.Errorf("status %q: %w", s, ErrInvalid)
}

type Task struct {
	ID     int
	Title  string
	Status Status
	Tags   []string
}

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid value")
)

func (t Task) Validate() error {
	var errs []error
	if n := len(strings.TrimSpace(t.Title)); n < 1 || n > 200 {
		errs = append(errs, fmt.Errorf("title must be 1-200 characters: %w", ErrInvalid))
	}
	if len(t.Tags) > 10 {
		errs = append(errs, fmt.Errorf("at most 10 tags: %w", ErrInvalid))
	}
	return errors.Join(errs...) // nil when errs is empty
}

// Store is declared where it is consumed; any type with these methods fits.
type Store interface {
	Create(t Task) (Task, error)
	Get(id int) (Task, error)
	All() iter.Seq[Task]
}

type MemStore struct {
	mu     sync.Mutex
	nextID int
	tasks  map[int]Task
}

func NewMemStore() *MemStore { return &MemStore{nextID: 1, tasks: map[int]Task{}} }

func (s *MemStore) Create(t Task) (Task, error) {
	if err := t.Validate(); err != nil {
		return Task{}, fmt.Errorf("create: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = s.nextID
	s.nextID++
	if t.Status == "" {
		t.Status = Todo
	}
	t.Tags = slices.Clone(t.Tags) // do not alias the caller's slice
	s.tasks[t.ID] = t
	return t, nil
}

func (s *MemStore) Get(id int) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("task %d: %w", id, ErrNotFound)
	}
	return t, nil
}

// All yields a snapshot ordered by id. iter.Seq lets callers range over it.
func (s *MemStore) All() iter.Seq[Task] {
	s.mu.Lock()
	snapshot := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		snapshot = append(snapshot, t)
	}
	s.mu.Unlock()
	slices.SortFunc(snapshot, func(a, b Task) int { return cmp.Compare(a.ID, b.ID) })
	return slices.Values(snapshot)
}

// WithStatus returns a closure over want: a reusable predicate.
func WithStatus(want Status) func(Task) bool {
	return func(t Task) bool { return t.Status == want }
}

func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

func main() {
	var store Store = NewMemStore()

	for _, in := range []Task{
		{Title: "Write docs", Tags: []string{"docs"}},
		{Title: "Review PR", Status: InProgress},
		{Title: "Ship it", Status: Done},
		{Title: "  "},
	} {
		created, err := store.Create(in)
		if err != nil {
			fmt.Println("rejected:", errors.Is(err, ErrInvalid), "-", err)
			continue
		}
		fmt.Printf("created #%d %q (%s)\n", created.ID, created.Title, created.Status)
	}

	if _, err := store.Get(42); errors.Is(err, ErrNotFound) {
		fmt.Println("lookup:", err)
	}

	fmt.Print("open tasks:")
	for t := range Filter(store.All(), func(t Task) bool { return t.Status != Done }) {
		fmt.Print(" ", t.Title, ";")
	}
	fmt.Println()

	inProgress := slices.Collect(Filter(store.All(), WithStatus(InProgress)))
	fmt.Println("in progress:", len(inProgress))

	if _, err := ParseStatus("blocked"); err != nil {
		fmt.Println(err)
	}
}
