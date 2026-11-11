package store

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestCreateAssignsSequentialIDs(t *testing.T) {
	s := New()
	a := s.Create(Task{Title: "a"})
	b := s.Create(Task{Title: "b"})
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids = %d, %d; want 1, 2", a.ID, b.ID)
	}
	got, err := s.Get(2)
	if err != nil || got.Title != "b" {
		t.Fatalf("Get(2) = %+v, %v", got, err)
	}
}

func TestGetMissing(t *testing.T) {
	_, err := New().Get(1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestStoreDoesNotAliasCallerSlices(t *testing.T) {
	s := New()
	tags := []string{"a", "b"}
	created := s.Create(Task{Title: "x", Tags: tags})

	tags[0] = "changed-by-caller"
	created.Tags[1] = "changed-via-return-value"

	got, _ := s.Get(1)
	if !slices.Equal(got.Tags, []string{"a", "b"}) {
		t.Fatalf("stored tags were modified through an alias: %q", got.Tags)
	}

	got.Tags[0] = "changed-via-get"
	again, _ := s.Get(1)
	if !slices.Equal(again.Tags, []string{"a", "b"}) {
		t.Fatalf("Get leaked the internal slice: %q", again.Tags)
	}

	for task := range s.All() {
		task.Tags[0] = "changed-via-all"
	}
	final, _ := s.Get(1)
	if !slices.Equal(final.Tags, []string{"a", "b"}) {
		t.Fatalf("All leaked the internal slice: %q", final.Tags)
	}
}

func TestAllIsOrderedAndAStableSnapshot(t *testing.T) {
	s := New()
	for _, title := range []string{"a", "b", "c", "d"} {
		s.Create(Task{Title: title})
	}
	seq := s.All()
	s.Create(Task{Title: "late"})

	var titles []string
	for task := range seq {
		titles = append(titles, task.Title)
	}
	if want := []string{"a", "b", "c", "d"}; !slices.Equal(titles, want) {
		t.Fatalf("titles = %q, want %q", titles, want)
	}
}

func TestAllStopsWhenConsumerBreaks(t *testing.T) {
	s := New()
	for range 5 {
		s.Create(Task{})
	}
	n := 0
	for range s.All() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Fatalf("iterated %d times", n)
	}
}

func TestFilterAndMapAreLazyAndStopEarly(t *testing.T) {
	s := New()
	for _, title := range []string{"alpha", "beta", "gamma", "delta"} {
		s.Create(Task{Title: title})
	}
	calls := 0
	longTitles := Filter(s.All(), func(task Task) bool {
		calls++
		return len(task.Title) == 5
	})
	upper := Map(longTitles, func(task Task) string { return strings.ToUpper(task.Title) })

	if calls != 0 {
		t.Fatal("Filter must be lazy: no predicate calls before iteration")
	}
	for v := range upper {
		if v != "ALPHA" {
			t.Fatalf("first value = %q", v)
		}
		break
	}
	if calls != 1 {
		t.Fatalf("predicate calls = %d, want 1 (stop pulling after break)", calls)
	}
	if got := slices.Collect(upper); !slices.Equal(got, []string{"ALPHA", "GAMMA", "DELTA"}) {
		t.Fatalf("collected %q", got)
	}
}

func TestConcurrentCreate(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { s.Create(Task{}) })
	}
	wg.Wait()
	if n := len(slices.Collect(s.All())); n != 50 {
		t.Fatalf("stored %d tasks, want 50", n)
	}
}
