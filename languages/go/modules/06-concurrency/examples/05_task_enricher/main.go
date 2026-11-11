// Task Tracker: a concurrent-safe in-memory store and a worker pool that normalises tags.
package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

type Task struct {
	ID    int
	Title string
	Tags  []string
}

// Store guards its map with an RWMutex: many readers or one writer.
type Store struct {
	mu     sync.RWMutex
	tasks  map[int]Task
	nextID int
}

func NewStore() *Store { return &Store{tasks: map[int]Task{}, nextID: 1} }

func (s *Store) Add(title string, tags ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	s.nextID++
	s.tasks[id] = Task{ID: id, Title: title, Tags: slices.Clone(tags)}
	return id
}

func (s *Store) Get(id int) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	t.Tags = slices.Clone(t.Tags) // never hand out the internal slice
	return t, ok
}

func (s *Store) SetTags(id int, tags []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tasks[id]; ok {
		t.Tags = tags
		s.tasks[id] = t
	}
}

func (s *Store) IDs() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]int, 0, len(s.tasks))
	for id := range s.tasks {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func normalise(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			out = append(out, t)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Enrich normalises the tags of every task using a fixed number of workers
// and returns how many tasks actually changed. The producer owns (and closes) the jobs channel.
func Enrich(ctx context.Context, s *Store, workers int) (int, error) {
	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for _, id := range s.IDs() {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	var changed atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for id := range jobs {
				t, ok := s.Get(id)
				if !ok {
					continue
				}
				if clean := normalise(t.Tags); !slices.Equal(clean, t.Tags) {
					s.SetTags(id, clean)
					changed.Add(1)
				}
			}
		})
	}
	wg.Wait()
	return int(changed.Load()), ctx.Err()
}

func main() {
	store := NewStore()
	store.Add("Write docs", " Docs", "P1", "docs")
	store.Add("Fix bug", "bug")
	store.Add("Ship it", "RELEASE ", "", "release")
	store.Add("Plan", "b", "a")
	store.Add("Empty")

	// Readers run while the pool writes; -race stays quiet because every access takes the lock.
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for range 100 {
				for _, id := range store.IDs() {
					store.Get(id)
				}
			}
		})
	}
	changed, err := Enrich(context.Background(), store, 3)
	readers.Wait()
	fmt.Println("changed:", changed, "err:", err)

	for _, id := range store.IDs() {
		t, _ := store.Get(id)
		fmt.Printf("%d %-10s %q\n", t.ID, t.Title, t.Tags)
	}

	changed, err = Enrich(context.Background(), store, 3)
	fmt.Println("second pass changed:", changed, "err:", err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Enrich(ctx, store, 3)
	fmt.Println("cancelled:", err)
}
