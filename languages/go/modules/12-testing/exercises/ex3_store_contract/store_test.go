package taskstore

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

var ctx = context.Background()

// runContract is the specification of Store. Any implementation (the in-memory one, a SQL one, a
// wrapper) is correct when it passes. newStore must return a fresh, empty store every time.
func runContract(t *testing.T, newStore func() Store) {
	t.Run("create trims, defaults and numbers from 1", func(t *testing.T) {
		s := newStore()
		a, err := s.Create(ctx, Task{Title: "  Write docs  "})
		if err != nil {
			t.Fatal(err)
		}
		b, _ := s.Create(ctx, Task{Title: "Ship", Status: "done", Tags: []string{"p1"}})
		if a.ID != 1 || a.Title != "Write docs" || a.Status != "todo" {
			t.Errorf("a = %+v", a)
		}
		if b.ID != 2 || b.Status != "done" || !slices.Equal(b.Tags, []string{"p1"}) {
			t.Errorf("b = %+v", b)
		}
	})

	t.Run("invalid tasks are rejected and consume no id", func(t *testing.T) {
		s := newStore()
		for _, bad := range []Task{
			{Title: ""}, {Title: "   "}, {Title: strings.Repeat("é", 201)}, {Title: "x", Status: "later"},
		} {
			if _, err := s.Create(ctx, bad); !errors.Is(err, ErrInvalid) {
				t.Errorf("Create(%+v): err = %v, want ErrInvalid", bad, err)
			}
		}
		if got, _ := s.Create(ctx, Task{Title: "ok"}); got.ID != 1 {
			t.Errorf("first stored task has id %d, want 1", got.ID)
		}
	})

	t.Run("get returns what was stored", func(t *testing.T) {
		s := newStore()
		created, _ := s.Create(ctx, Task{Title: "Write docs", Tags: []string{"docs", "p1"}})
		got, err := s.Get(ctx, created.ID)
		if err != nil || !reflect.DeepEqual(got, created) {
			t.Fatalf("Get = %+v, %v; want %+v", got, err, created)
		}
		if _, err := s.Get(ctx, 99); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(99): err = %v", err)
		}
	})

	t.Run("callers never share memory with the store", func(t *testing.T) {
		s := newStore()
		tags := []string{"a", "b"}
		created, _ := s.Create(ctx, Task{Title: "x", Tags: tags})
		tags[0] = "changed after create"
		created.Tags[1] = "changed in the returned value"

		got, _ := s.Get(ctx, created.ID)
		got.Tags[0] = "changed in a Get result"
		list, _ := s.List(ctx, "")
		list[0].Tags[1] = "changed in a List result"

		if again, _ := s.Get(ctx, created.ID); !slices.Equal(again.Tags, []string{"a", "b"}) {
			t.Fatalf("stored tags were modified through a caller's slice: %v", again.Tags)
		}
	})

	t.Run("update replaces and validates", func(t *testing.T) {
		s := newStore()
		created, _ := s.Create(ctx, Task{Title: "draft", Tags: []string{"a"}})
		updated, err := s.Update(ctx, Task{ID: created.ID, Title: " final ", Status: "done"})
		if err != nil {
			t.Fatal(err)
		}
		if updated.Title != "final" || updated.Status != "done" || len(updated.Tags) != 0 {
			t.Errorf("updated = %+v (Update replaces the whole task)", updated)
		}
		if _, err := s.Update(ctx, Task{ID: created.ID, Title: ""}); !errors.Is(err, ErrInvalid) {
			t.Errorf("invalid update: err = %v", err)
		}
		if got, _ := s.Get(ctx, created.ID); got.Title != "final" {
			t.Errorf("a rejected update changed the stored task: %+v", got)
		}
		if _, err := s.Update(ctx, Task{ID: 99, Title: "x"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("update of unknown id: err = %v", err)
		}
	})

	t.Run("list is ordered, filterable and never nil", func(t *testing.T) {
		s := newStore()
		empty, err := s.List(ctx, "")
		if err != nil || empty == nil || len(empty) != 0 {
			t.Fatalf("empty store: %#v, %v", empty, err)
		}
		for _, st := range []string{"todo", "done", "todo", "done"} {
			s.Create(ctx, Task{Title: "t", Status: st})
		}
		ids := func(status string) []int {
			ts, _ := s.List(ctx, status)
			out := []int{}
			for _, task := range ts {
				out = append(out, task.ID)
			}
			return out
		}
		if got := ids(""); !slices.Equal(got, []int{1, 2, 3, 4}) {
			t.Errorf("all = %v", got)
		}
		if got := ids("done"); !slices.Equal(got, []int{2, 4}) {
			t.Errorf("done = %v", got)
		}
		if none, _ := s.List(ctx, "in_progress"); none == nil {
			t.Error("a list without matches must be an empty slice, not nil")
		}
	})

	t.Run("delete removes one task and ids are not reused", func(t *testing.T) {
		s := newStore()
		s.Create(ctx, Task{Title: "a"})
		s.Create(ctx, Task{Title: "b"})
		if err := s.Delete(ctx, 2); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, 2); !errors.Is(err, ErrNotFound) {
			t.Errorf("second delete: err = %v", err)
		}
		if _, err := s.Get(ctx, 1); err != nil {
			t.Errorf("task 1 lost: %v", err)
		}
		if c, _ := s.Create(ctx, Task{Title: "c"}); c.ID != 3 {
			t.Errorf("new task got id %d, want 3", c.ID)
		}
	})

	t.Run("concurrent creates get distinct ids", func(t *testing.T) {
		s := newStore()
		var mu sync.Mutex
		seen := map[int]bool{}
		var wg sync.WaitGroup
		for range 50 {
			wg.Go(func() {
				got, err := s.Create(ctx, Task{Title: "x"})
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				seen[got.ID] = true
				mu.Unlock()
			})
		}
		wg.Wait()
		if len(seen) != 50 {
			t.Fatalf("%d distinct ids for 50 creates", len(seen))
		}
	})
}

var errBoom = errors.New("boom")

func TestMemStoreContract(t *testing.T) {
	runContract(t, func() Store { return NewMemStore() })
}

// A wrapper that is not failing yet must be invisible: it passes the very same contract.
func TestFaultyIsTransparentUntilItFails(t *testing.T) {
	runContract(t, func() Store { return NewFaulty(NewMemStore(), 1<<30, errBoom) })
}

func TestFaultyFailsAfterOkCalls(t *testing.T) {
	inner := NewMemStore()
	f := NewFaulty(inner, 2, errBoom)

	if _, err := f.Create(ctx, Task{Title: "one"}); err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if _, err := f.Get(ctx, 1); err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if _, err := f.Create(ctx, Task{Title: "two"}); !errors.Is(err, errBoom) {
		t.Fatalf("call 3: err = %v, want errBoom", err)
	}
	if _, err := f.List(ctx, ""); !errors.Is(err, errBoom) {
		t.Fatalf("call 4: err = %v, want errBoom", err)
	}
	if err := f.Delete(ctx, 1); !errors.Is(err, errBoom) {
		t.Fatalf("call 5: err = %v, want errBoom", err)
	}
	if _, err := f.Update(ctx, Task{ID: 1, Title: "x"}); !errors.Is(err, errBoom) {
		t.Fatalf("call 6: err = %v, want errBoom", err)
	}
	if _, err := f.Get(ctx, 1); !errors.Is(err, errBoom) {
		t.Fatalf("call 7: err = %v, want errBoom", err)
	}
	if got := f.Calls(); got != 7 {
		t.Errorf("Calls = %d, want 7", got)
	}
	// Failed calls must not reach the inner store.
	all, _ := inner.List(ctx, "")
	if len(all) != 1 || all[0].Title != "one" {
		t.Fatalf("inner store = %+v: failed calls must not be forwarded", all)
	}
}

func TestFaultyCountsConcurrentCalls(t *testing.T) {
	f := NewFaulty(NewMemStore(), 1<<30, errBoom)
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() { f.List(ctx, "") })
	}
	wg.Wait()
	if got := f.Calls(); got != 40 {
		t.Fatalf("Calls = %d, want 40", got)
	}
}

func seed(t *testing.T, s Store, statuses ...string) {
	t.Helper()
	for _, st := range statuses {
		if _, err := s.Create(ctx, Task{Title: "t", Status: st}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeleteDone(t *testing.T) {
	s := NewMemStore()
	seed(t, s, "done", "todo", "done", "in_progress", "done")
	n, err := DeleteDone(ctx, s)
	if err != nil || n != 3 {
		t.Fatalf("DeleteDone = %d, %v; want 3, nil", n, err)
	}
	left, _ := s.List(ctx, "")
	if len(left) != 2 {
		t.Fatalf("%d tasks left, want 2", len(left))
	}
	if n, err := DeleteDone(ctx, s); err != nil || n != 0 {
		t.Fatalf("nothing to delete: %d, %v", n, err)
	}
}

// Fault injection reaches the error paths a healthy fake never takes.
func TestDeleteDoneStopsAtTheFirstError(t *testing.T) {
	inner := NewMemStore()
	seed(t, inner, "done", "done", "done")

	// List and the first Delete succeed (2 calls); the second Delete fails.
	n, err := DeleteDone(ctx, NewFaulty(inner, 2, errBoom))
	if n != 1 || !errors.Is(err, errBoom) {
		t.Fatalf("DeleteDone = %d, %v; want 1 and errBoom", n, err)
	}
	if left, _ := inner.List(ctx, ""); len(left) != 2 {
		t.Fatalf("%d tasks left in the inner store, want 2", len(left))
	}
}

func TestDeleteDoneListFailure(t *testing.T) {
	n, err := DeleteDone(ctx, NewFaulty(NewMemStore(), 0, errBoom))
	if n != 0 || !errors.Is(err, errBoom) {
		t.Fatalf("DeleteDone = %d, %v", n, err)
	}
}
